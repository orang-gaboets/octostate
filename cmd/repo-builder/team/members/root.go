package members

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// NewCmd creates a "team members" command group for team membership operations.
func NewCmd(svc teams.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "members",
		Aliases: []string{"member"},
		Short:   "Team member operations",
		Long:    "Manage and query GitHub team members.",
	}

	cmd.AddCommand(
		ListCmd(svc),
	)

	return cmd
}
