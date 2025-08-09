package team

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
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
		DeleteTeamBySlugCmd(svc),
		GetTeamBySlugCmd(svc),
	)

	return cmd
}
