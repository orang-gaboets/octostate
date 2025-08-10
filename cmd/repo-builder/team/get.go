package team

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// GetTeamBySlugCmd creates a new command to get a GitHub team by its slug.
func GetTeamBySlugCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
	)

	cmd := &cobra.Command{
		Use:     "get-by-slug",
		Aliases: []string{"get", "get-slug", "slug", "gbs"},
		Short:   "Get a GitHub team by its slug",
		Long:    "Retrieve a GitHub team by its slug within an organization.",
		Example: `
			repo-builder team get-by-slug --token <token> --org <org> --slug <team-slug>
			repo-builder team get-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}
			opts := teams.GetTeamBySlugOptions{
				Org:     org,
				Slug:    slug,
				Service: service,
			}
			_, err := teams.GetTeamBySlug(ctx, opts)
			return err
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().Int64Var(&appID, "app-id", 0, "GitHub App ID for authentication")
	cmd.Flags().Int64Var(&installationID, "installation-id", 0, "GitHub App installation ID for authentication")
	cmd.Flags().StringVar(&appKeyPath, "app-key-path", "", "Path to the GitHub App private key file")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")

	github.MarkRequiredFlags(cmd, "org", "slug")

	return cmd
}
