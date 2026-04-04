package repo

import (
	"github.com/spf13/cobra"

	repopermissions "github.com/orang-gaboets/octostate/cmd/octostate/team/repo/permissions"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
)

// NewCmd creates a "team repo" command group for team repository operations.
func NewCmd(svc teams.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repo",
		Aliases: []string{"repos"},
		Short:   "Team repository operations",
		Long:    "Manage and query repositories associated with a GitHub team.",
	}

	cmd.AddCommand(
		repopermissions.NewCmd(svc),
	)

	return cmd
}
