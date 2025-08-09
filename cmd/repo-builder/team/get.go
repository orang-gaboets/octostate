package team

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	githubclient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// GetTeamBySlugCmd creates a new command to get a GitHub team by its slug.
func GetTeamBySlugCmd(svc teams.Service) *cobra.Command {
	var (
		token string
		org   string
		slug  string
	)

	cmd := &cobra.Command{
		Use:     "get-by-slug",
		Aliases: []string{"get", "get-slug", "slug", "gbs"},
		Short:   "Get a GitHub team by its slug",
		Long:    "Retrieve a GitHub team by its slug within an organization.",
		Example: `repo-builder team get-by-slug --token <token> --org <org> --slug <team-slug>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if svc == nil {
				client := githubclient.New(ctx, token)
				service = client.Teams
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
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")

	github.MarkRequiredFlags(cmd, "token", "org", "slug")

	return cmd
}
