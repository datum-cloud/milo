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
		Short: "Milo is a platform for building and managing the next generation of service providers and their customers.",
	}

	rootCmd.AddCommand(controller.NewCommand())
	rootCmd.AddCommand(apiserver.NewCommand())

	code := cli.Run(rootCmd)
	os.Exit(code)
}
