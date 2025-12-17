package organization

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

// ListOrgReposCmd creates a command to list all repositories within a GitHub organization.
func ListOrgReposCmd(svc repos.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		repoType       string
	)

	cmd := &cobra.Command{
		Use:     "list-repos",
		Aliases: []string{"repos", "list-repo"},
		Short:   "List repositories in a GitHub organization",
		Long:    "Retrieve and display all repositories belonging to a specified GitHub organization.",
		Example: `
			repo-builder organization list-repos --token <token> --org <org-name>
			repo-builder organization list-repos --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --type all`,
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

			if !repos.RepoType(strings.TrimSpace(repoType)).IsValid() {
				return github.ErrInvalidFieldValue
			}

			opts := repos.ListOrgReposOptions{
				Service: service,
				Org:     strings.TrimSpace(org),
				Type:    repos.RepoType(strings.TrimSpace(repoType)),
			}

			_, err := repos.ListOrgRepos(ctx, opts)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&repoType, "type", "all", "Repository type filter: all, public, private, forks, sources, member")

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}
