package app

import "github.com/spf13/cobra"

func Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "iam-apiserver",
		Short: "CLI for interacting with the Datum IAM service",
	}

	cmd.AddCommand(
		addResourcesCommand(),
		serve(),
	)

	return cmd
}
