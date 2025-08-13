package app

import (
	"go.miloapis.com/milo/pkg/apiserver/admission/plugin/namespace/lifecycle"
	"k8s.io/apimachinery/pkg/util/sets"
	validatingadmissionpolicy "k8s.io/apiserver/pkg/admission/plugin/policy/validating"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"
	"k8s.io/kubernetes/pkg/kubeapiserver/options"
)

// DefaultOffAdmissionPlugins get admission plugins off by default for kube-apiserver.
func DefaultOffAdmissionPlugins() sets.Set[string] {
	defaultOnPlugins := sets.New[string](
		lifecycle.PluginName, // NamespaceLifecycle
		// defaulttolerationseconds.PluginName, // DefaultTolerationSeconds
		mutatingwebhook.PluginName,   // MutatingAdmissionWebhook
		validatingwebhook.PluginName, // ValidatingAdmissionWebhook
		// resourcequota.PluginName,            // ResourceQuota
		// certapproval.PluginName,              // CertificateApproval
		// certsigning.PluginName,               // CertificateSigning
		// ctbattest.PluginName,                 // ClusterTrustBundleAttest
		// certsubjectrestriction.PluginName,    // CertificateSubjectRestriction
		validatingadmissionpolicy.PluginName, // ValidatingAdmissionPolicy, only active when feature gate ValidatingAdmissionPolicy is enabled
	)

	universe := sets.New(options.AllOrderedPlugins...)
	universe.Insert(lifecycle.PluginName)

	return universe.Difference(defaultOnPlugins)
}
