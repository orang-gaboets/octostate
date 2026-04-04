package team

import (
	"github.com/spf13/cobra"

	teammembers "github.com/orang-gaboets/octostate/cmd/octostate/team/members"
	teamrepo "github.com/orang-gaboets/octostate/cmd/octostate/team/repo"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
)

// NewTeamCmd creates a new "team" command group for managing teams on GitHub
func NewTeamCmd(svc teams.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "team",
		Aliases: []string{"teams"},
		Short:   "Team operation",
		Long:    "Manage teams on GitHub",
	}

	cmd.AddCommand(
		CreateTeamCmd(svc),
		EditTeamCmd(svc),
		DeleteTeamBySlugCmd(svc),
		GetTeamBySlugCmd(svc),
		teammembers.NewCmd(svc),
		teamrepo.NewCmd(svc),
	)

	return cmd
}
