// milo/pkg/apiserver/admission/initializer/loopback_only.go
package initializer

import (
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/rest"
)

// Local duck-typed interface: any plugin with this method will match.
type wantsLoopback interface {
	SetLoopbackConfig(*rest.Config)
}

type LoopbackInitializer struct {
	Loopback *rest.Config
}

func (i LoopbackInitializer) Initialize(p admission.Interface) {
	if w, ok := p.(wantsLoopback); ok && i.Loopback != nil {
		w.SetLoopbackConfig(i.Loopback)
	}
}
