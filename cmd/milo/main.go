package main

import (
	"os"

	"github.com/spf13/cobra"
	"k8s.io/component-base/cli"

	apiserver "go.datum.net/milo/cmd/milo/apiserver"
	controller "go.datum.net/milo/cmd/milo/controller-manager"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "milo",
		Short: "Milo is a control plane for modern service providers, built on top of a comprehensive system of record that ties together key parts of your business.",
	}

	rootCmd.AddCommand(controller.NewCommand())
	rootCmd.AddCommand(apiserver.NewCommand())

	code := cli.Run(rootCmd)
	os.Exit(code)
}
