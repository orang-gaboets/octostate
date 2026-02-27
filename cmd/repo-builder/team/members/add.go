package members

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/safety"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// AddCmd creates a command to add or update a user's membership in a GitHub team by slug.
func AddCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
		username       string
		role           string
		dryRun         bool
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"create", "upsert"},
		Short:   "Add or update a user in a GitHub team",
		Long:    "Add a user to a GitHub team by slug, or update their team membership role.",
		Example: `
			repo-builder team members add --token <token> --org <org-name> --slug <team-slug> --username <username>
			repo-builder team members add --token <token> --org <org-name> --slug <team-slug> --username <username> --role maintainer --dry-run
			repo-builder team members add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug> --username <username> --role member`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			trimmedOrg := strings.TrimSpace(org)
			trimmedSlug := strings.TrimSpace(slug)
			trimmedUsername := strings.TrimSpace(username)
			trimmedRole := strings.TrimSpace(role)

			if trimmedUsername == "" {
				return fmt.Errorf("username cannot be empty: %w", github.ErrMissingRequiredField)
			}
			if !teams.TeamMemberAddRole(trimmedRole).IsValid() {
				return github.ErrInvalidFieldValue
			}

			if dryRun {
				_, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"Dry run: would add user %q to team %s/%s with role %s\n",
					trimmedUsername,
					trimmedOrg,
					trimmedSlug,
					trimmedRole,
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

			opts := teams.AddTeamMemberBySlugOptions{
				Service:  service,
				Org:      trimmedOrg,
				Slug:     trimmedSlug,
				Username: trimmedUsername,
				Role:     teams.TeamMemberAddRole(trimmedRole),
			}

			membership, err := teams.AddTeamMemberBySlug(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, membership)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	cmd.Flags().StringVar(&username, "username", "", "GitHub username to add to the team")
	cmd.Flags().StringVar(&role, "role", string(teams.TeamMemberAddRoleMember), "Team membership role: member, maintainer")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug", "username")

	return cmd
}
