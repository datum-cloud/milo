package admission

import (
	"io"

	"k8s.io/apiserver/pkg/admission"
	"k8s.io/klog/v2"
)

// Register registers the ResourceQuotaEnforcement admission plugin for custom plugin registries
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		klog.InfoS("Registered resource quota enforcement plugin with Milo apiserver")
		return NewResourceQuotaEnforcementPlugin()
	})
}
