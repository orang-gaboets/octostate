package main

import (
	auditcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/audit"
	configcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/config"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/organization"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/team"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/topic"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/user"
	ghlogging "github.com/orang-gaboets/repo-builder/pkg/github/logging"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	var verbose bool

	cmd := &cobra.Command{
		Use:           "repo-builder",
		Short:         "Repo Builder CLI",
		Long:          "A CLI tool to manage repositories on GitHub",
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
