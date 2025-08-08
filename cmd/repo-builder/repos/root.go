package repos

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

func NewRepoCmd(svc repos.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Repository operation",
		Long:  "Manage repositories on GitHub",
	}

	cmd.AddCommand(
		CreateNewRepoFromTemplateCmd(svc),
		EditRepo(svc),
	)

	return cmd
}
