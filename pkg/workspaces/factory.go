package workspaces

import (
	"context"
	"fmt"
	"net"
	"strings"

	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	kubeopenapi "k8s.io/kubernetes/pkg/generated/openapi"

	flowcontrolv1beta3 "k8s.io/api/flowcontrol/v1beta3"
	rbacv1 "k8s.io/api/rbac/v1" // ← NEW
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apiserver/pkg/admission"
	genericapiserver "k8s.io/apiserver/pkg/server"
	recommended "k8s.io/apiserver/pkg/server/options"
	"k8s.io/component-base/version"
	"k8s.io/kubernetes/pkg/controlplane"

	genopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	openapicommon "k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"
	controlplaneapiserver "k8s.io/kubernetes/pkg/controlplane/apiserver"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	// New groups from the parent list
	admissionregv1 "k8s.io/api/admissionregistration/v1"
	authnv1 "k8s.io/api/authentication/v1"
	authzv1 "k8s.io/api/authorization/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	eventsapi "k8s.io/api/events/v1"

	// Internal-APIServer objects (only served, never stored)
	apiserverinternalv1alpha1 "k8s.io/api/apiserverinternal/v1alpha1"
)

type idleListener struct{}

func (idleListener) Accept() (net.Conn, error) { return nil, fmt.Errorf("disabled") }
func (idleListener) Close() error              { return nil }
func (idleListener) Addr() net.Addr            { return &net.TCPAddr{IP: net.IPv4zero, Port: 0} }

// Factory is created once at Milo start-up and stamps out per-project servers.
type Factory struct {
	BaseOpts *recommended.RecommendedOptions // root flags (authn/authz/etcd …)
	Codecs   serializer.CodecFactory         // same codecs Milo root uses
	Loopback *rest.Config                    // loopback client config for the workspace server
	// Config for the OpenAPI v2 spec.
	Config *openapicommon.Config
	// Config for the OpenAPI v3 spec.
	Configv3  *openapicommon.OpenAPIV3Config
	Providers func(discovery discovery.DiscoveryInterface) ([]controlplaneapiserver.RESTStorageProvider, error)
}

func NewFactory(base *recommended.RecommendedOptions,
	codecs serializer.CodecFactory,
	loopback *rest.Config,
	OpenAPIV2 *openapicommon.Config,
	OpenAPIV3 *openapicommon.OpenAPIV3Config,
	providers func(discovery discovery.DiscoveryInterface) ([]controlplaneapiserver.RESTStorageProvider, error),
) *Factory {
	return &Factory{BaseOpts: base, Codecs: codecs, Loopback: loopback, Providers: providers, Config: OpenAPIV2, Configv3: OpenAPIV3}
}

// Build spins up an in-memory GenericAPIServer for the project <id>.
func (f *Factory) Build(ctx context.Context, id string) (*genericapiserver.GenericAPIServer, error) {
	if f.BaseOpts == nil || f.BaseOpts.Etcd == nil {
		return nil, fmt.Errorf("factory: Etcd backend not configured")
	}

	// ──-- 1. copy shared RecommendedOptions ────────────────────────────────────
	opts := *f.BaseOpts // value copy — safe to mutate

	// Never bind a host port: we expose only the http.Handler.
	// opts.SecureServing = &recommended.SecureServingOptionsWithLoopback{
	// 	SecureServingOptions: &recommended.SecureServingOptions{
	// 		BindPort:    0,
	// 		BindAddress: net.IPv4zero,
	// 	},
	// }

	opts.ExtraAdmissionInitializers =
		func(*genericapiserver.RecommendedConfig) ([]admission.PluginInitializer, error) {
			return nil, nil
		}

	// ──-- 2. clone EtcdOptions and rewrite the key prefix ─────────────────────
	etcd := *opts.Etcd        // shallow copy
	cfg := etcd.StorageConfig // copy again
	cfg.Prefix = fmt.Sprintf("/projects/%s/registry", id)

	var gvList = []schema.GroupVersion{
		corev1.SchemeGroupVersion,
		coordinationv1.SchemeGroupVersion,
		rbacv1.SchemeGroupVersion,
		flowcontrolv1beta3.SchemeGroupVersion,
		authnv1.SchemeGroupVersion,
		authzv1.SchemeGroupVersion,
		admissionregv1.SchemeGroupVersion,
		eventsapi.SchemeGroupVersion,
		discoveryv1.SchemeGroupVersion,
		// apiserverinternal is **served only**; still include so the codec knows it
		apiserverinternalv1alpha1.SchemeGroupVersion,
	}
	cfg.Codec = f.Codecs.LegacyCodec(gvList...) // use the same codec as Milo root
	cfg.EncodeVersioner = schema.GroupVersions(gvList)
	etcd.StorageConfig = cfg
	opts.Etcd = &etcd

	// ──-- 3. build the server ─────────────────────────────────────────────────
	rec := genericapiserver.NewRecommendedConfig(f.Codecs)
	opts.Features.EnablePriorityAndFairness = false
	opts.Admission = nil
	// opts.SecureServing = &recommended.SecureServingOptionsWithLoopback{
	// 	SecureServingOptions: &recommended.SecureServingOptions{
	// 		BindAddress: net.ParseIP("127.0.0.1"),
	// 		BindPort:    0, // still 0 → no listener
	// 	},
	// }
	// opts.SecureServing.ExternalAddress = net.ParseIP("127.0.0.1")
	// opts.SecureServing.BindPort = 443
	if err := opts.ApplyTo(rec); err != nil {
		return nil, fmt.Errorf("apply workspace options: %w", err)
	}

	rec.LoopbackClientConfig = f.Loopback // ← set it **after**
	if rec.LoopbackClientConfig == nil {
		klog.ErrorS(nil,
			"LOOPBACK IS NIL  ➜  APF bootstrap will fail",
			"project", id)
	} else {
		klog.InfoS("loop-back present",
			"project", id,
			"host", rec.LoopbackClientConfig.Host)
	}

	// Print rec.OpenAPIConfig if it is nil, so we can see it in the logs.
	klog.V(4).InfoS("workspace-recommended-config",
		"project", id,
		"loopback", rec.LoopbackClientConfig.Host,
		"openapi", rec.OpenAPIConfig,
	)

	// rec.OpenAPIConfig = f.Config
	// rec.OpenAPIV3Config = f.Configv3

	if rec.OpenAPIConfig == nil {
		rec.OpenAPIConfig = &openapicommon.Config{
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:   "Milo Workspace API",
					Version: "v1",
				},
			},
			GetDefinitions: kubeopenapi.GetOpenAPIDefinitions,
			GetOperationIDAndTagsFromRoute: func(r openapicommon.Route) (string, []string, error) {
				// e.g.  GET_/api       → "GET__api"
				//       GET_/apis      → "GET__apis"
				//       GET_/apis_v1   → "GET__apis_v1"
				id := strings.ToUpper(r.Method()) + "_" +
					strings.ReplaceAll(strings.Trim(r.Path(), "/"), "/", "_")
				return id, nil, nil
			},
		}
		rec.EffectiveVersion = version.DefaultKubeEffectiveVersion()
	}
	genopenapi.NewDefinitionNamer()

	if rec.OpenAPIV3Config == nil {
		rec.OpenAPIV3Config = &openapicommon.OpenAPIV3Config{
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:   "Milo Workspace API V3",
					Version: "v1",
				},
			},
			GetDefinitions: kubeopenapi.GetOpenAPIDefinitions,
			GetOperationIDAndTagsFromRoute: func(r openapicommon.Route) (string, []string, error) {
				// e.g.  GET_/api       → "GET__api"
				//       GET_/apis      → "GET__apis"
				//       GET_/apis_v1   → "GET__apis_v1"
				id := strings.ToUpper(r.Method()) + "_" +
					strings.ReplaceAll(strings.Trim(r.Path(), "/"), "/", "_")
				return id, nil, nil
			},
		}
	}

	// ---------------------------------------------------------------------
	// Ensure every route gets a *unique* operation-id.
	// ---------------------------------------------------------------------

	server, err := rec.Complete().New("workspace-"+id, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}
	if f.Providers != nil {
		// 1) discovery client that talks to THIS workspace
		cli, err := kubernetes.NewForConfig(rec.LoopbackClientConfig)
		if err != nil {
			return nil, fmt.Errorf("workspace %s: build client: %w", id, err)
		}

		// 2) the storage-provider slice the root server uses
		storage, err := f.Providers(cli.Discovery())
		if err != nil {
			return nil, fmt.Errorf("workspace %s: storage providers: %w", id, err)
		}

		// 3) enable every resource by default
		apiCfg := controlplane.DefaultAPIResourceConfigSource()
		restOpt := rec.RESTOptionsGetter

		klog.InfoS("workspace-providers", "project", id, "count", len(storage))

		for _, sp := range storage {
			grp, err := sp.NewRESTStorage(apiCfg, restOpt)
			if err != nil {
				klog.ErrorS(err, "workspace storage build failed",
					"project", id, "provider", fmt.Sprintf("%T", sp))
				return nil, fmt.Errorf("workspace %s: %w", id, err)
			}
			if len(grp.VersionedResourcesStorageMap) == 0 {
				continue // disabled by feature-gate
			}

			gv := grp.PrioritizedVersions[0] // always present
			klog.InfoS("group-result",
				"project", id,
				"group", gv.String(),
				"resources", len(grp.VersionedResourcesStorageMap))

			// ----- core (/api/v1) needs InstallLegacyAPIGroup -------------
			if gv.Group == "" {
				if err := server.InstallLegacyAPIGroup("/api", &grp); err != nil {
					return nil, fmt.Errorf("workspace %s: install legacy: %w", id, err)
				}
			} else {
				if err := server.InstallAPIGroup(&grp); err != nil {
					return nil, fmt.Errorf("workspace %s: install %s: %w", id, gv, err)
				}
			}
			// --------------------------------------------------------------
		}
	}
	prepared := server.PrepareRun()

	// ──-- 4. run it in the background ─────────────────────────────────────────
	prepared.RunPostStartHooks(ctx)

	return server, nil
}
