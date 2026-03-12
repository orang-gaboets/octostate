package config

import "github.com/spf13/cobra"

// NewConfigCmd creates the config command group.
func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg"},
		Short:   "Manage GitOps configuration",
		Long:    "Validate and inspect local GitOps desired-state configuration.",
	}

	cmd.AddCommand(
		PlanConfigCmd(),
		ValidateConfigCmd(),
	)

	return cmd
}
