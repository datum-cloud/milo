package identity

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	generic "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	controlplaneapiserver "k8s.io/kubernetes/pkg/controlplane/apiserver"

	machineaccountkeysregistry "go.miloapis.com/milo/internal/apiserver/identity/machineaccountkeys"
	sessionsregistry "go.miloapis.com/milo/internal/apiserver/identity/sessions"
	useridentitiesregistry "go.miloapis.com/milo/internal/apiserver/identity/useridentities"
	identityapi "go.miloapis.com/milo/pkg/apis/identity"
	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

type StorageProvider struct {
	Sessions       sessionsregistry.Backend
	UserIdentities useridentitiesregistry.Backend
}

func (p StorageProvider) GroupName() string { return identityv1alpha1.SchemeGroupVersion.Group }

func (p StorageProvider) NewRESTStorage(
	_ serverstorage.APIResourceConfigSource,
	getter generic.RESTOptionsGetter,
) (genericapiserver.APIGroupInfo, error) {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(
		identityv1alpha1.SchemeGroupVersion.Group,
		legacyscheme.Scheme,
		metav1.ParameterCodec,
		legacyscheme.Codecs,
	)

	// Identity types do not have protobuf support, so we wrap the getter to override the
	// storage codec with a JSON-only one. Without this, CompleteWithOptions fails with
	// "internal type not encodable: object does not implement the protobuf marshalling interface".
	machineAccountKeyStorage, err := machineaccountkeysregistry.NewREST(jsonOnlyGetter(getter))
	if err != nil {
		return apiGroupInfo, err
	}

	storage := map[string]rest.Storage{
		"sessions":           sessionsregistry.NewREST(p.Sessions),
		"useridentities":     useridentitiesregistry.NewREST(p.UserIdentities),
		"machineaccountkeys": machineAccountKeyStorage,
	}

	apiGroupInfo.VersionedResourcesStorageMap = map[string]map[string]rest.Storage{
		identityv1alpha1.SchemeGroupVersion.Version: storage,
	}

	return apiGroupInfo, nil
}

var _ controlplaneapiserver.RESTStorageProvider = StorageProvider{}

// jsonOnlyGetter wraps a RESTOptionsGetter to override the storage codec with a
// JSON-only variant for identity resources. This is required because identity types
// do not implement the protobuf marshalling interface, and the default storage factory
// uses protobuf as the preferred encoding format.
type jsonOnlyGetterWrapper struct {
	inner generic.RESTOptionsGetter
	codec runtime.Codec
}

func jsonOnlyGetter(inner generic.RESTOptionsGetter) generic.RESTOptionsGetter {
	// Build a fresh scheme with only JSON serializers (no protobuf registered).
	identityScheme := runtime.NewScheme()
	identityapi.Install(identityScheme)
	metav1.AddToGroupVersion(identityScheme, schema.GroupVersion{Group: "", Version: "v1"})

	// serializer.NewCodecFactory registers JSON and YAML only — no protobuf.
	identityCodecs := serializer.NewCodecFactory(identityScheme)
	jsonCodec := identityCodecs.LegacyCodec(identityv1alpha1.SchemeGroupVersion)

	return &jsonOnlyGetterWrapper{inner: inner, codec: jsonCodec}
}

func (g *jsonOnlyGetterWrapper) GetRESTOptions(gr schema.GroupResource, example runtime.Object) (generic.RESTOptions, error) {
	opts, err := g.inner.GetRESTOptions(gr, example)
	if err != nil {
		return opts, err
	}
	// Replace the default codec (which tries protobuf) with a JSON-only one.
	opts.StorageConfig.Codec = g.codec
	return opts, nil
}
