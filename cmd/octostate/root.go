package main

import (
	auditcmd "github.com/orang-gaboets/octostate/cmd/octostate/audit"
	configcmd "github.com/orang-gaboets/octostate/cmd/octostate/config"
	"github.com/orang-gaboets/octostate/cmd/octostate/organization"
	"github.com/orang-gaboets/octostate/cmd/octostate/repo"
	"github.com/orang-gaboets/octostate/cmd/octostate/team"
	"github.com/orang-gaboets/octostate/cmd/octostate/topic"
	"github.com/orang-gaboets/octostate/cmd/octostate/user"
	ghlogging "github.com/orang-gaboets/octostate/pkg/github/logging"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:           "octostate",
		Short:         "Octostate CLI",
		Long:          "A CLI tool for GitHub operations and GitOps",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(ghlogging.WithVerbose(cmd.Context(), verbose, cmd.ErrOrStderr()))
		},
	}

	cmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable diagnostic logs on stderr")

	cmd.AddCommand(
		auditcmd.NewAuditCmd(),
		configcmd.NewConfigCmd(),
		organization.NewOrganizationCmd(nil, nil, nil),
		repo.NewRepoCmd(nil),
		team.NewTeamCmd(nil),
		topic.NewTopicCmd(nil),
		user.NewUserCmd(nil),
	)

	return cmd
}

func Execute() error {
	rootCmd := newRootCmd()
	return rootCmd.Execute()
}
