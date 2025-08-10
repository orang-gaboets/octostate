package repo

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

// DeleteRepoCmd creates a new command to delete a GitHub repository.
func DeleteRepoCmd(svc repos.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		name           string
	)

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del", "remove"},
		Short:   "Delete a GitHub repository",
		Long:    "Delete a specified GitHub repository by providing the organization and repository name.",
		Example: `
			repo-builder repo delete --token <token> --org <org> --name <repo-name>
			repo-builder repo delete --app-id <app-id> --installation-id <installation-id> --app-key-path <path> --org <org> --name <repo-name>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Repositories()
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
	cmd.Flags().Int64Var(&appID, "app-id", 0, "GitHub App ID for authentication")
	cmd.Flags().Int64Var(&installationID, "installation-id", 0, "GitHub App installation ID for authentication")
	cmd.Flags().StringVar(&appKeyPath, "app-key-path", "", "Path to the GitHub App private key file")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name to delete")

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}
