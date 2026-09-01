package members

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
)

// ListCmd creates a command to list members of a GitHub team by slug.
func ListCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
		role           string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List members in a GitHub team",
		Long:    "Retrieve and display members belonging to a specified GitHub team.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate team members list --org <org-name> --slug <team-slug>
			octostate team members list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug> --role maintainer`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			trimmedRole := strings.TrimSpace(role)
			if !teams.TeamMemberRole(trimmedRole).IsValid() {
				return github.ErrInvalidFieldValue
			}

			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}

			opts := teams.ListTeamMembersBySlugOptions{
				Service: service,
				Org:     strings.TrimSpace(org),
				Slug:    strings.TrimSpace(slug),
				Role:    teams.TeamMemberRole(trimmedRole),
			}

			members, err := teams.ListTeamMembersBySlug(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, members)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	cmd.Flags().StringVar(&role, "role", string(teams.TeamMemberRoleAll), "Team member role filter: all, member, maintainer")

	github.MarkRequiredFlags(cmd, "org", "slug")

	return cmd
}
