package infracluster

import (
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Options defines the configuration options available for modifying the
// behavior of the infrastructure cluster client.
type Options struct {
	KubeconfigFile string
}

func (o *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.KubeconfigFile, "infra-cluster-kubeconfig", "", "The path to the kubeconfig file for the infrastructure cluster.")
}

func (o *Options) GetClient() (*rest.Config, error) {
	config, err := clientcmd.BuildConfigFromFlags("", o.KubeconfigFile)
	if err != nil {
		return nil, err
	}
	return config, nil
}
