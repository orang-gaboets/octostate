package members

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/configproposal"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
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
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"create", "upsert"},
		Short:   "Add or update a user in a GitHub team",
		Long:    "Add a user to a GitHub team by slug, or update their team membership role.",
		Example: `
			octostate team members add --token <token> --org <org-name> --slug <team-slug> --username <username>
			octostate team members add --token <token> --org <org-name> --slug <team-slug> --username <username> --role maintainer --dry-run
			octostate team members add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug> --username <username> --role member`,
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

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
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

			if cmd.Flags().Changed("to-config") {
				return addTeamMemberToConfig(cmd, toConfig, trimmedOrg, trimmedSlug, trimmedUsername, trimmedRole)
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
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug", "username")

	return cmd
}

func addTeamMemberToConfig(cmd *cobra.Command, path, org, slug, username, role string) error {
	canonicalUsername := username
	changed, err := configproposal.ApplyToConfigFile(path, org, func(cfg *gitopsconfig.OrganizationConfig) error {
		teamIndex, found := configproposal.FindTeamIndex(cfg, slug)
		if !found {
			return fmt.Errorf("team %s/%s not found in config", org, slug)
		}
		memberIndex, found := configproposal.FindOrganizationMemberIndex(cfg, username)
		if !found {
			return fmt.Errorf("member %q must be declared in top-level members before joining a team", username)
		}
		canonicalUsername = strings.TrimSpace(cfg.Members[memberIndex].Username)

		team := &cfg.Teams[teamIndex]
		if existing, found := configproposal.FindTeamMemberIndex(team, username); found {
			canonicalUsername = strings.TrimSpace(team.Members[existing].Username)
			team.Members[existing].Role = role
			return nil
		}
		team.Members = append(team.Members, gitopsconfig.TeamMemberSpec{
			Username: canonicalUsername,
			Role:     role,
		})
		return nil
	})
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Proposed member add for team %s/%s in config", org, slug)
	if !changed {
		message = fmt.Sprintf("No changes needed for add member %s/%s", org, slug)
	}
	return cmdoutput.PrintSuccess(cmd, message, map[string]any{
		"organization": org,
		"slug":         slug,
		"username":     canonicalUsername,
		"role":         role,
		"config_path":  path,
		"changed":      changed,
	})
}
