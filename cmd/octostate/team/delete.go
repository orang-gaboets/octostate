package team

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
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "delete-by-slug",
		Aliases: []string{"delete", "del", "ds"},
		Short:   "Delete a GitHub team by its slug",
		Long:    "Delete a GitHub team by its slug within an organization.",
		Example: `
			# Proposal mode
			octostate team delete-by-slug --org <org> --slug <team-slug> --to-config <path-to-organization.yaml>

			# Live mode
			octostate team delete-by-slug --token <token> --org <org> --slug <team-slug> --yes
			octostate team delete-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --yes

			# Dry-run mode
			octostate team delete-by-slug --org <org> --slug <team-slug> --dry-run`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trimmedOrg := strings.TrimSpace(org)
			trimmedSlug := strings.TrimSpace(slug)

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("Dry run: would delete team %s/%s", trimmedOrg, trimmedSlug),
					map[string]any{
						"organization": trimmedOrg,
						"slug":         trimmedSlug,
					},
				)
			}
			if cmd.Flags().Changed("to-config") {
				return deleteTeamToConfig(cmd, toConfig, trimmedOrg, trimmedSlug)
			}
			if err := safety.RequireYesOrDryRun(yes, dryRun); err != nil {
				return err
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
				Org:     trimmedOrg,
				Slug:    trimmedSlug,
				Service: service,
			}
			if err := teams.DeleteTeamBySlug(ctx, opts); err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Deleted team %s/%s", trimmedOrg, trimmedSlug),
				map[string]any{
					"organization": trimmedOrg,
					"slug":         trimmedSlug,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)
	safety.AddYesFlag(cmd, &yes)

	github.MarkRequiredFlags(cmd, "org", "slug")

	return cmd
}

func deleteTeamToConfig(cmd *cobra.Command, path, org, slug string) error {
	changed, err := configproposal.ApplyToConfigFile(path, org, func(cfg *gitopsconfig.OrganizationConfig) error {
		index, found := configproposal.FindTeamIndex(cfg, slug)
		if !found {
			return fmt.Errorf("team %s/%s not found in config", org, slug)
		}

		blockers, hasChildTeamBlocker := collectTeamDeleteBlockers(cfg, slug)
		if len(blockers) > 0 {
			reason := "while dependencies exist"
			if hasChildTeamBlocker {
				reason = "because it would violate the config validator's child-team invariant"
			}
			return fmt.Errorf(
				"team %s/%s cannot be deleted from config %s: %s",
				org,
				slug,
				reason,
				strings.Join(blockers, ", "),
			)
		}

		cfg.Teams = append(cfg.Teams[:index], cfg.Teams[index+1:]...)
		return nil
	})
	if err != nil {
		return err
	}

	return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Proposed team %s/%s deletion in config", org, slug), map[string]any{
		"organization": org,
		"slug":         slug,
		"config_path":  path,
		"changed":      changed,
	})
}

func collectTeamDeleteBlockers(cfg *gitopsconfig.OrganizationConfig, slug string) ([]string, bool) {
	if cfg == nil {
		return nil, false
	}

	blockers := make([]string, 0)
	hasChildTeamBlocker := false
	for _, team := range cfg.Teams {
		if strings.EqualFold(strings.TrimSpace(team.ParentSlug), slug) {
			hasChildTeamBlocker = true
			blockers = append(blockers, fmt.Sprintf(
				"child team %s(parent_slug=%s)",
				strings.TrimSpace(team.Slug),
				strings.TrimSpace(team.ParentSlug),
			))
		}
	}

	for inviteIndex, invite := range cfg.Invites {
		for _, teamSlug := range invite.TeamSlugs {
			if strings.EqualFold(strings.TrimSpace(teamSlug), slug) {
				blockers = append(blockers, fmt.Sprintf("invite[%d](team_slug=%s)", inviteIndex, strings.TrimSpace(teamSlug)))
			}
		}
	}

	return blockers, hasChildTeamBlocker
}
