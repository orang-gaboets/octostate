package repo

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitHubClient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

// DeleteRepoCmd creates a new command to delete a GitHub repository.
func DeleteRepoCmd(svc repos.Service) *cobra.Command {
	var (
		token string
		org   string
		name  string
	)

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del", "remove"},
		Short:   "Delete a GitHub repository",
		Long:    "Delete a specified GitHub repository by providing the organization and repository name.",
		Example: `repo-builder repo delete --token <token> --org <org> --name <repo-name>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client := gitHubClient.New(ctx, token)
				service = client.Repositories
			}

			if org == "" || name == "" {
				return cmd.Help()
			}

			opts := repos.DeleteOptions{
				Repository: github.Repository{
					Org:  org,
					Name: name,
				},
				Service: service,
			}

			err := repos.Delete(ctx, opts)
			return err
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name to delete")

	requiredFlags := []string{"token", "org", "name"}
	for _, flag := range requiredFlags {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			cobra.CheckErr(err)
		}
	}

	return cmd
}
