package permissions

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// NewCmd creates a "team repo permissions" command group.
func NewCmd(svc teams.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "permissions",
		Aliases: []string{"permission", "perms"},
		Short:   "Team repository permission operations",
		Long:    "List and manage repository permissions granted to a GitHub team.",
	}

	cmd.AddCommand(
		AddCmd(svc),
		ListCmd(svc),
		RemoveCmd(svc),
	)

	return cmd
}
