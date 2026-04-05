package members

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
)

// RemoveCmd creates a command to remove a user from a GitHub team by slug.
func RemoveCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
		username       string
		dryRun         bool
	)

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"delete", "del", "rm"},
		Short:   "Remove a user from a GitHub team",
		Long:    "Remove a user's membership from a GitHub team by slug.",
		Example: `
			octostate team members remove --token <token> --org <org-name> --slug <team-slug> --username <username>
			octostate team members remove --token <token> --org <org-name> --slug <team-slug> --username <username> --dry-run
			octostate team members remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug> --username <username>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			trimmedOrg := strings.TrimSpace(org)
			trimmedSlug := strings.TrimSpace(slug)
			trimmedUsername := strings.TrimSpace(username)

			if trimmedUsername == "" {
				return fmt.Errorf("username cannot be empty: %w", github.ErrMissingRequiredField)
			}

			if dryRun {
				_, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"Dry run: would remove user %q from team %s/%s\n",
					trimmedUsername,
					trimmedOrg,
					trimmedSlug,
				)
				return err
			}

			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}

			opts := teams.RemoveTeamMemberBySlugOptions{
				Service:  service,
				Org:      trimmedOrg,
				Slug:     trimmedSlug,
				Username: trimmedUsername,
			}

			if err := teams.RemoveTeamMemberBySlug(ctx, opts); err != nil {
				return err
			}

			_, err := fmt.Fprintf(cmd.OutOrStdout(), "Removed user %q from team %s/%s\n", trimmedUsername, trimmedOrg, trimmedSlug)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	cmd.Flags().StringVar(&username, "username", "", "GitHub username to remove from the team")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug", "username")

	return cmd
}
