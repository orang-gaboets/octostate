package main

import (
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/repos"
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo-builder",
		Short: "Repo Builder CLI",
		Long:  "A CLI tool to manage repositories on GitHub",
	}

	cmd.AddCommand(
		repos.NewRepoCmd(nil),
	)

	return cmd
}

func Execute() error {
	rootCmd := newRootCmd()
	return rootCmd.Execute()
}
