package providers

type Provider string

const (
	// ProviderSingle behaves as a normal controller-runtime manager
	ProviderSingle Provider = "single"

	// ProviderMilo discovers clusters by watching Project resources
	ProviderMilo Provider = "milo"

	// ProviderKind discovers clusters registered via kind
	ProviderKind Provider = "kind"
)

// AllowedProviders are the supported multicluster-runtime Provider implementations.
var AllowedProviders = []Provider{
	ProviderSingle,
	ProviderMilo,
}
