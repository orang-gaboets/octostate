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
		toConfig       string
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

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
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

			if cmd.Flags().Changed("to-config") {
				return removeTeamMemberToConfig(cmd, toConfig, trimmedOrg, trimmedSlug, trimmedUsername)
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
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug", "username")

	return cmd
}

func removeTeamMemberToConfig(cmd *cobra.Command, path, org, slug, username string) error {
	reportedUsername := username
	changed, err := configproposal.ApplyToConfigFile(path, org, func(cfg *gitopsconfig.OrganizationConfig) error {
		teamIndex, found := configproposal.FindTeamIndex(cfg, slug)
		if !found {
			return fmt.Errorf("team %s/%s not found in config", org, slug)
		}

		team := &cfg.Teams[teamIndex]
		memberIndex, found := configproposal.FindTeamMemberIndex(team, username)
		if !found {
			return nil
		}
		reportedUsername = strings.TrimSpace(team.Members[memberIndex].Username)
		team.Members = append(team.Members[:memberIndex], team.Members[memberIndex+1:]...)
		return nil
	})
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Proposed member remove for team %s/%s in config", org, slug)
	if !changed {
		message = fmt.Sprintf("No changes needed for remove member %s/%s", org, slug)
	}
	return cmdoutput.PrintSuccess(cmd, message, map[string]any{
		"organization": org,
		"slug":         slug,
		"username":     reportedUsername,
		"config_path":  path,
		"changed":      changed,
	})
}
