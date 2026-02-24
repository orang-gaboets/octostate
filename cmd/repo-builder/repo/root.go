package repo

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

// NewRepoCmd creates a new "repo" command group for managing repositories on GitHub.
func NewRepoCmd(svc repos.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repo",
		Aliases: []string{"repos"},
		Short:   "Repository operation",
		Long:    "Manage repositories on GitHub",
	}

	cmd.AddCommand(
		CreateNewRepoFromTemplateCmd(svc),
		DeleteRepoCmd(svc),
		EditRepo(svc),
		GetRepoCmd(svc),
	)

	return cmd
}
