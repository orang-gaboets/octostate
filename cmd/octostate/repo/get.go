package repo

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
)

// GetRepoCmd creates a new command to get a GitHub repository by owner and name.
func GetRepoCmd(svc repos.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		name           string
	)

	cmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"get-by-name", "find", "fetch"},
		Short:   "Get a GitHub repository",
		Long:    "Retrieve details of a GitHub repository by owner and repository name.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate repo get --org <org> --name <repo-name>
			octostate repo get --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name>`,
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

			opts := repos.GetOptions{
				Service: service,
				Owner:   strings.TrimSpace(org),
				Repo:    strings.TrimSpace(name),
			}
			repoInfo, err := repos.Get(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, repoInfo)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}
