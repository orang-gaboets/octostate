package permissions

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
)

// ListCmd creates a command to list repository permissions for a GitHub team.
func ListCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List repository permissions for a GitHub team",
		Long:    "Retrieve repositories that a GitHub team can access, including the team's permissions for each repository.",
		Example: `
			octostate team repo permissions list --token <token> --org <org-name> --slug <team-slug>
			octostate team repo permissions list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug>`,
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

			opts := teams.ListTeamRepoPermissionsBySlugOptions{
				Service: service,
				Org:     strings.TrimSpace(org),
				Slug:    strings.TrimSpace(slug),
			}

			repos, err := teams.ListTeamRepoPermissionsBySlug(ctx, opts)
			if err != nil {
				return err
			}

			return cmdoutput.PrintJSON(cmd, repos)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")

	github.MarkRequiredFlags(cmd, "org", "slug")

	return cmd
}
