package permissions

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

// RemoveCmd creates a command to remove a team's permission on a repository.
func RemoveCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
		repoOrg        string
		repo           string
		dryRun         bool
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"delete", "del", "rm", "revoke"},
		Short:   "Remove repository permission from a GitHub team",
		Long:    "Remove a GitHub team's access to a repository.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate team repo permissions remove --org <org-name> --slug <team-slug> --repo <repo-name>
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate team repo permissions remove --org <org-name> --slug <team-slug> --repo-org <repo-org> --repo <repo-name> --dry-run
			octostate team repo permissions remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug> --repo <repo-name>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			trimmedOrg := strings.TrimSpace(org)
			trimmedSlug := strings.TrimSpace(slug)
			trimmedRepoOrg := strings.TrimSpace(repoOrg)
			trimmedRepo := strings.TrimSpace(repo)

			if trimmedOrg == "" {
				return fmt.Errorf("organization cannot be empty: %w", github.ErrMissingRequiredField)
			}
			if trimmedRepoOrg == "" {
				trimmedRepoOrg = trimmedOrg
			}
			if !gitopsconfig.RepositoryOwnerMatchesOrganization(trimmedRepoOrg, trimmedOrg) {
				return fmt.Errorf("repository owner %q must match organization %q", trimmedRepoOrg, trimmedOrg)
			}
			if trimmedRepo == "" {
				return fmt.Errorf("repo name cannot be empty: %w", github.ErrMissingRequiredField)
			}

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf(
						"Dry run: would remove team %s/%s access to repository %s/%s",
						trimmedOrg,
						trimmedSlug,
						trimmedRepoOrg,
						trimmedRepo,
					),
					map[string]any{
						"organization": trimmedOrg,
						"slug":         trimmedSlug,
						"repo_owner":   trimmedRepoOrg,
						"repo_name":    trimmedRepo,
					},
				)
			}

			if cmd.Flags().Changed("to-config") {
				return removeTeamRepoPermissionToConfig(cmd, toConfig, trimmedOrg, trimmedSlug, trimmedRepoOrg, trimmedRepo)
			}

			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}

			opts := teams.RemoveTeamRepoPermissionBySlugOptions{
				Service:   service,
				Org:       trimmedOrg,
				Slug:      trimmedSlug,
				RepoOwner: trimmedRepoOrg,
				RepoName:  trimmedRepo,
			}

			if err := teams.RemoveTeamRepoPermissionBySlug(ctx, opts); err != nil {
				return err
			}

			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf(
					"Removed team %s/%s access to repository %s/%s",
					trimmedOrg,
					trimmedSlug,
					trimmedRepoOrg,
					trimmedRepo,
				),
				map[string]any{
					"organization": trimmedOrg,
					"slug":         trimmedSlug,
					"repo_owner":   trimmedRepoOrg,
					"repo_name":    trimmedRepo,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	cmd.Flags().StringVar(&repoOrg, "repo-org", "", "Owner organization of the target repository (defaults to --org)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug", "repo")

	return cmd
}

func removeTeamRepoPermissionToConfig(cmd *cobra.Command, path, org, slug, repoOwner, repoName string) error {
	changed, err := configproposal.ApplyToConfigFile(path, org, func(cfg *gitopsconfig.OrganizationConfig) error {
		teamIndex, found := configproposal.FindTeamIndex(cfg, slug)
		if !found {
			return fmt.Errorf("team %s/%s not found in config", org, slug)
		}

		team := &cfg.Teams[teamIndex]
		repositoryIndex, found := configproposal.FindTeamRepositoryIndex(team, cfg.Organization, repoOwner, repoName)
		if !found {
			return nil
		}
		team.Repositories = append(team.Repositories[:repositoryIndex], team.Repositories[repositoryIndex+1:]...)
		return nil
	})
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Proposed repository permission removal for team %s/%s in config", org, slug)
	if !changed {
		message = fmt.Sprintf("No changes needed for repository permission removal %s/%s", org, slug)
	}
	return cmdoutput.PrintSuccess(cmd, message, map[string]any{
		"organization": org,
		"slug":         slug,
		"repo_owner":   repoOwner,
		"repo_name":    repoName,
		"config_path":  path,
		"changed":      changed,
	})
}
