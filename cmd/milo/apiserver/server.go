package app

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"go.miloapis.com/milo/internal/apiserver/admission/plugin/namespace/lifecycle"
	projectstorage "go.miloapis.com/milo/internal/apiserver/storage/project"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1" // ← add / keep this
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apiserver/pkg/admission"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	genericapiserver "k8s.io/apiserver/pkg/server"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/util/notfoundhandler"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/cli/globalflag"
	"k8s.io/component-base/featuregate"
	"k8s.io/component-base/logs"
	logsapi "k8s.io/component-base/logs/api/v1"
	_ "k8s.io/component-base/metrics/prometheus/workqueue"
	"k8s.io/component-base/term"
	"k8s.io/component-base/version"
	"k8s.io/component-base/version/verflag"
	"k8s.io/klog/v2"
	aggregatorapiserver "k8s.io/kube-aggregator/pkg/apiserver"

	// Register JSON logging format
	_ "k8s.io/component-base/logs/json/register"
	controlplaneapiserver "k8s.io/kubernetes/pkg/controlplane/apiserver"
	"k8s.io/kubernetes/pkg/controlplane/apiserver/options"
)

func init() {
	utilruntime.Must(logsapi.AddFeatureGates(utilfeature.DefaultMutableFeatureGate))
	// Enable JSON logging support by default
	utilfeature.DefaultMutableFeatureGate.Set("LoggingBetaOptions=true")
}

var (
	// Configure the namespace that is used for system components and resources
	// automatically bootstrapped by the control plane.
	SystemNamespace string
	// Sessions feature/config flags
	featureSessions          bool
	sessionsProviderGroup    string
	sessionsProviderVersion  string
	sessionsProviderResource string
	// Standalone provider connection (direct URL + mTLS)
	sessionsProviderURL        string
	sessionsProviderCAFile     string
	sessionsProviderClientCert string
	sessionsProviderClientKey  string
	providerTimeoutSeconds     int
	providerRetries            int
	forwardExtras              []string
)

// NewCommand creates a *cobra.Command object with default parameters
func NewCommand() *cobra.Command {
	s := NewOptions()
	var namedFlagSets cliflag.NamedFlagSets

	cmd := &cobra.Command{
		Use:   "apiserver",
		Short: "Extendable API server",
		Long:  `The Milo API server provides an extensible API platform.`,

		// stop printing usage when the command errors
		SilenceUsage: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			// silence client-go warnings.
			// kube-apiserver loopback clients should not log self-issued warnings.
			rest.SetDefaultWarningHandler(rest.NoWarnings{})
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			verflag.PrintAndExitIfRequested()
			fs := cmd.Flags()

			// Activate logging as soon as possible, after that
			// show flags with the final logging configuration.
			if err := logsapi.ValidateAndApply(s.Logs, utilfeature.DefaultFeatureGate); err != nil {
				return err
			}
			cliflag.PrintFlags(fs)
			s.SystemNamespaces = []string{metav1.NamespaceSystem, metav1.NamespaceDefault, SystemNamespace}

			completedOptions, err := s.Complete(cmd.Context(), namedFlagSets, []string{}, []net.IP{})
			if err != nil {
				return err
			}

			// utilfeature.DefaultMutableFeatureGate.Set("ExternalServiceAccountTokenSigner=true")

			if errs := completedOptions.Validate(); len(errs) != 0 {
				return utilerrors.NewAggregate(errs)
			}

			// add feature enablement metrics
			utilfeature.DefaultMutableFeatureGate.AddMetrics()

			ctx := genericapiserver.SetupSignalContext()
			return Run(ctx, completedOptions)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			for _, arg := range args {
				if len(arg) > 0 {
					return fmt.Errorf("%q does not take any arguments, got %q", cmd.CommandPath(), args)
				}
			}
			return nil
		},
	}

	// Override the ComponentGlobalsRegistry to avoid k8s feature flags from being added.
	// Add our own feature gates or expose native k8s ones if necessary
	s.GenericServerRunOptions.ComponentGlobalsRegistry = featuregate.NewComponentGlobalsRegistry()
	s.GenericServerRunOptions.AddUniversalFlags(namedFlagSets.FlagSet("generic"))
	s.Etcd.AddFlags(namedFlagSets.FlagSet("etcd"))
	s.SecureServing.AddFlags(namedFlagSets.FlagSet("secure serving"))
	s.Audit.AddFlags(namedFlagSets.FlagSet("auditing"))
	s.Features.AddFlags(namedFlagSets.FlagSet("features"))
	s.Authentication.AddFlags(namedFlagSets.FlagSet("authentication"))
	s.Authorization.AddFlags(namedFlagSets.FlagSet("authorization"))
	// Add our own Admission flags
	s.Metrics.AddFlags(namedFlagSets.FlagSet("metrics"))
	logsapi.AddFlags(s.Logs, namedFlagSets.FlagSet("logs"))
	s.Traces.AddFlags(namedFlagSets.FlagSet("traces"))
	// Add misc flags for event ttl

	verflag.AddFlags(namedFlagSets.FlagSet("global"))
	globalflag.AddGlobalFlags(namedFlagSets.FlagSet("global"), cmd.Name(), logs.SkipLoggingConfigurationFlags())

	fs := cmd.Flags()
	for _, f := range namedFlagSets.FlagSets {
		fs.AddFlagSet(f)
	}

	fs.StringVar(&s.ServiceAccountSigningKeyFile, "service-account-signing-key-file", s.ServiceAccountSigningKeyFile, ""+
		"Path to the file that contains the current private key of the service account token issuer. The issuer will sign issued ID tokens with this private key.")

	fs.StringVar(&s.ServiceAccountSigningEndpoint, "service-account-signing-endpoint", s.ServiceAccountSigningEndpoint, ""+
		"Path to socket where a external JWT signer is listening. This flag is mutually exclusive with --service-account-signing-key-file and --service-account-key-file. Requires enabling feature gate (ExternalServiceAccountTokenSigner)")

	fs.StringVar(&s.ProxyClientCertFile, "proxy-client-cert-file", s.ProxyClientCertFile, ""+
		"Client certificate used to prove the identity of the aggregator or kube-apiserver "+
		"when it must call out during a request. This includes proxying requests to a user "+
		"api-server and calling out to webhook admission plugins. It is expected that this "+
		"cert includes a signature from the CA in the --requestheader-client-ca-file flag. "+
		"That CA is published in the 'extension-apiserver-authentication' configmap in "+
		"the kube-system namespace. Components receiving calls from kube-aggregator should "+
		"use that CA to perform their half of the mutual TLS verification.")
	fs.StringVar(&s.ProxyClientKeyFile, "proxy-client-key-file", s.ProxyClientKeyFile, ""+
		"Private key for the client certificate used to prove the identity of the aggregator or kube-apiserver "+
		"when it must call out during a request. This includes proxying requests to a user "+
		"api-server and calling out to webhook admission plugins.")

	fs.StringVar(&SystemNamespace, "system-namespace", "milo-system", "The namespace to use for system components and resources that are automatically created to run the system.")
	fs.BoolVar(&featureSessions, "feature-sessions", false, "Enable identity sessions virtual API")
	fs.StringVar(&sessionsProviderGroup, "sessions-provider-group", "identity.miloapis.com", "Provider group for sessions")
	fs.StringVar(&sessionsProviderVersion, "sessions-provider-version", "v1alpha1", "Provider version for sessions")
	fs.StringVar(&sessionsProviderResource, "sessions-provider-resource", "sessions", "Provider resource for sessions")
	fs.StringVar(&sessionsProviderURL, "sessions-provider-url", "", "Direct provider base URL (e.g., https://zitadel-apiserver:8443)")
	fs.StringVar(&sessionsProviderCAFile, "sessions-provider-ca-file", "", "Path to CA file to validate provider TLS")
	fs.StringVar(&sessionsProviderClientCert, "sessions-provider-client-cert", "", "Client certificate for mTLS to provider")
	fs.StringVar(&sessionsProviderClientKey, "sessions-provider-client-key", "", "Client private key for mTLS to provider")
	fs.IntVar(&providerTimeoutSeconds, "provider-timeout", 3, "Provider request timeout in seconds")
	fs.IntVar(&providerRetries, "provider-retries", 2, "Provider request retries")
	fs.StringSliceVar(&forwardExtras, "forward-extras", []string{"iam.miloapis.com/parent-api-group", "iam.miloapis.com/parent-type", "iam.miloapis.com/parent-name"}, "User extras keys to forward during impersonation")

	cols, _, _ := term.TerminalSize(cmd.OutOrStdout())
	cliflag.SetUsageAndHelpFunc(cmd, namedFlagSets, cols)

	return cmd
}

func NewOptions() *options.Options {
	s := options.NewOptions()
	s.Admission.GenericAdmission.DefaultOffPlugins = DefaultOffAdmissionPlugins()

	if s.Admission.GenericAdmission.Plugins == nil {
		s.Admission.GenericAdmission.Plugins = admission.NewPlugins()
	}
	lifecycle.Register(s.Admission.GenericAdmission.Plugins)

	s.Admission.GenericAdmission.RecommendedPluginOrder =
		append(s.Admission.GenericAdmission.RecommendedPluginOrder, lifecycle.PluginName)
	s.Admission.GenericAdmission.EnablePlugins = append(s.Admission.GenericAdmission.EnablePlugins, lifecycle.PluginName)

	wd, _ := os.Getwd()
	s.SecureServing.ServerCert.CertDirectory = filepath.Join(wd, ".sample-minimal-controlplane")

	// Wire ServiceAccount authentication without relying on pods and nodes.
	s.Authentication.ServiceAccounts.OptionalTokenGetter = genericTokenGetter
	s.ServiceAccountIssuer = &jwtTokenGenerator{}

	return s
}

// Run runs the specified APIServer. This should never exit.
func Run(ctx context.Context, opts options.CompletedOptions) error {
	// To help debugging, immediately log version
	klog.Infof("Version: %+v", version.Get())

	klog.InfoS("Golang settings", "GOGC", os.Getenv("GOGC"), "GOMAXPROCS", os.Getenv("GOMAXPROCS"), "GOTRACEBACK", os.Getenv("GOTRACEBACK"))

	config, err := NewConfig(opts)
	if err != nil {
		return err
	}

	// inject flag-derived extra config
	config.ExtraConfig.FeatureSessions = featureSessions
	config.ExtraConfig.SessionsProvider.Group = sessionsProviderGroup
	config.ExtraConfig.SessionsProvider.Version = sessionsProviderVersion
	config.ExtraConfig.SessionsProvider.Resource = sessionsProviderResource
	config.ExtraConfig.SessionsProvider.URL = sessionsProviderURL
	config.ExtraConfig.SessionsProvider.CAFile = sessionsProviderCAFile
	config.ExtraConfig.SessionsProvider.ClientCertFile = sessionsProviderClientCert
	config.ExtraConfig.SessionsProvider.ClientKeyFile = sessionsProviderClientKey
	config.ExtraConfig.SessionsProvider.TimeoutSeconds = providerTimeoutSeconds
	config.ExtraConfig.SessionsProvider.Retries = providerRetries
	config.ExtraConfig.SessionsProvider.ForwardExtras = forwardExtras

	completed, err := config.Complete()
	if err != nil {
		return err
	}

	server, err := CreateServerChain(completed)
	if err != nil {
		return err
	}

	prepared, err := server.PrepareRun()
	if err != nil {
		return err
	}

	return prepared.Run(ctx)
}

// CreateServerChain creates the apiservers connected via delegation.
func CreateServerChain(config CompletedConfig) (*aggregatorapiserver.APIAggregator, error) {
	// 1. CRDs
	notFoundHandler := notfoundhandler.New(config.ControlPlane.Generic.Serializer, genericapifilters.NoMuxAndDiscoveryIncompleteKey)

	config.APIExtensions.GenericConfig.RESTOptionsGetter =
		projectstorage.WithProjectAwareDecorator(config.APIExtensions.GenericConfig.RESTOptionsGetter)

	config.APIExtensions.ExtraConfig.CRDRESTOptionsGetter =
		projectstorage.WithProjectAwareDecorator(config.APIExtensions.ExtraConfig.CRDRESTOptionsGetter)

	apiExtensionsServer, err := config.APIExtensions.New(genericapiserver.NewEmptyDelegateWithCustomHandler(notFoundHandler))
	if err != nil {
		return nil, fmt.Errorf("failed to create apiextensions-apiserver: %w", err)
	}
	crdAPIEnabled := config.APIExtensions.GenericConfig.MergedResourceConfig.ResourceEnabled(apiextensionsv1.SchemeGroupVersion.WithResource("customresourcedefinitions"))

	// 2. Natively implemented resources
	nativeAPIs, err := config.ControlPlane.New("datum-apiserver", apiExtensionsServer.GenericAPIServer)
	if err != nil {
		return nil, fmt.Errorf("failed to create datum controlplane apiserver: %w", err)
	}
	client, err := kubernetes.NewForConfig(config.ControlPlane.Generic.LoopbackClientConfig)
	if err != nil {
		return nil, err
	}
	storageProviders, err := config.GenericStorageProviders(client.Discovery())
	if err != nil {
		return nil, fmt.Errorf("failed to create storage providers: %w", err)
	}

	wrapped := make([]controlplaneapiserver.RESTStorageProvider, 0, len(storageProviders))
	for _, p := range storageProviders {
		wrapped = append(wrapped, projectstorage.WrapProvider(p))
	}

	if err := nativeAPIs.InstallAPIs(wrapped...); err != nil {
		return nil, fmt.Errorf("failed to install APIs: %w", err)
	}

	// 3. Aggregator for APIServices, discovery and OpenAPI
	aggregatorServer, err := controlplaneapiserver.CreateAggregatorServer(
		config.Aggregator,
		nativeAPIs.GenericAPIServer,
		apiExtensionsServer.Informers.Apiextensions().V1().CustomResourceDefinitions(),
		crdAPIEnabled,
		controlplaneapiserver.DefaultGenericAPIServicePriorities(),
	)
	if err != nil {
		// we don't need special handling for innerStopCh because the aggregator server doesn't create any go routines
		return nil, fmt.Errorf("failed to create kube-aggregator: %w", err)
	}

	return aggregatorServer, nil
}
