package cmd

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

// NewRootCmd returns the root cobra command. If svc is nil, a GitHub client will
// be created from the provided token at runtime.
func NewRootCmd(svc repos.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo-builder",
		Short: "Create and manage GitHub repositories",
		Long:  "A command-line tool to create and manage GitHub repositories and GitHub teams.",
	}

	cmd.AddCommand(CreateNewRepoFromTemplateCmd(svc))
	cmd.AddCommand(EditRepo(svc))

	return cmd
}

// Execute executes the root command.
func Execute() {
	if err := NewRootCmd(nil).Execute(); err != nil {
		log.Fatal(err)
	}
}
