package quota

import (
	"io"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/klog/v2"
)

// init registers the plugin when the package is imported
// This is the standard pattern used by Kubernetes admission plugins
func init() {
	// Plugin registration will happen when the package is imported
	// The actual registration needs to be done where admission plugins are configured
}

// Register registers the ClaimCreationQuota admission plugin for custom plugin registries
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		klog.InfoS("Registered claim creation plugin with Milo apiserver")
		return NewClaimCreationPlugin()
	})
}
