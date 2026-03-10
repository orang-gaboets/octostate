package audit

import "github.com/spf13/cobra"

// NewAuditCmd creates the audit command group.
func NewAuditCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit GitOps actual state",
		Long:  "Snapshot and compare actual GitHub state for GitOps workflows.",
	}

	cmd.AddCommand(
		PullCmd(),
	)

	return cmd
}
