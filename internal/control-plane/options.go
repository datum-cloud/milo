package controlplane

import "github.com/spf13/pflag"

const (
	ScopeCore    string = "core"
	ScopeProject string = "project"
)

type Options struct {
	// Can either be "core" or "project".
	Scope string
}

func NewOptions() *Options {
	return &Options{}
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {

	fs.StringVar(&o.Scope, "control-plane-scope", "", "The scope of the control plane. Can be either 'core' or 'project'.")
}
