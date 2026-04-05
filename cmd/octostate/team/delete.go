package team

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
)

// DeleteTeamBySlugCmd creates a new command to delete a GitHub team by its slug.
func DeleteTeamBySlugCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
		dryRun         bool
		yes            bool
	)

	cmd := &cobra.Command{
		Use:     "delete-by-slug",
		Aliases: []string{"delete", "del", "ds"},
		Short:   "Delete a GitHub team by its slug",
		Long:    "Delete a GitHub team by its slug within an organization.",
		Example: `
			octostate team delete-by-slug --token <token> --org <org> --slug <team-slug> --yes
			octostate team delete-by-slug --token <token> --org <org> --slug <team-slug> --dry-run
			octostate team delete-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --yes`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := safety.RequireYesOrDryRun(yes, dryRun); err != nil {
				return err
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("Dry run: would delete team %s/%s", org, slug),
					map[string]any{
						"organization": org,
						"slug":         slug,
					},
				)
			}

			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}

			opts := teams.DeleteTeamBySlugOptions{
				Org:     org,
				Slug:    slug,
				Service: service,
			}
			if err := teams.DeleteTeamBySlug(ctx, opts); err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Deleted team %s/%s", org, slug),
				map[string]any{
					"organization": org,
					"slug":         slug,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	safety.AddDryRunFlag(cmd, &dryRun)
	safety.AddYesFlag(cmd, &yes)

	github.MarkRequiredFlags(cmd, "org", "slug")

	return cmd
}
