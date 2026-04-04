package team

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
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
			octostate team get-by-slug --token <token> --org <org> --slug <team-slug>
			octostate team get-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug>`,
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
			teamInfo, err := teams.GetTeamBySlug(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, teamInfo)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")

	github.MarkRequiredFlags(cmd, "org", "slug")

	return cmd
}
