package main

import (
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/organization"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/team"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/topic"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo-builder",
		Short: "Repo Builder CLI",
		Long:  "A CLI tool to manage repositories on GitHub",
	}

	cmd.AddCommand(
		organization.NewOrganizationCmd(nil),
		repo.NewRepoCmd(nil),
		team.NewTeamCmd(nil),
		topic.NewTopicCmd(nil),
	)

	return cmd
}

func Execute() error {
	rootCmd := newRootCmd()
	return rootCmd.Execute()
}
