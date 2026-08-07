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

// AddCmd creates a command to grant a team permission on a repository.
func AddCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
		repoOrg        string
		repo           string
		permission     string
		dryRun         bool
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"grant", "set"},
		Short:   "Grant repository permission to a GitHub team",
		Long:    "Grant or update a GitHub team's permission on a repository.",
		Example: `
			octostate team repo permissions add --token <token> --org <org-name> --slug <team-slug> --repo <repo-name> --permission push
			octostate team repo permissions add --token <token> --org <org-name> --slug <team-slug> --repo-org <repo-org> --repo <repo-name> --permission maintain --dry-run
			octostate team repo permissions add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug> --repo <repo-name> --permission pull`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			trimmedOrg := strings.TrimSpace(org)
			trimmedSlug := strings.TrimSpace(slug)
			trimmedRepoOrg := strings.TrimSpace(repoOrg)
			trimmedRepo := strings.TrimSpace(repo)
			trimmedPermission := strings.TrimSpace(permission)

			if trimmedRepoOrg == "" {
				trimmedRepoOrg = trimmedOrg
			}
			if trimmedRepo == "" {
				return fmt.Errorf("repo name cannot be empty: %w", github.ErrMissingRequiredField)
			}
			if !teams.TeamRepoPermission(trimmedPermission).IsValid() {
				return github.ErrInvalidFieldValue
			}

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf(
						"Dry run: would grant team %s/%s permission %s on repository %s/%s",
						trimmedOrg,
						trimmedSlug,
						trimmedPermission,
						trimmedRepoOrg,
						trimmedRepo,
					),
					map[string]any{
						"organization": trimmedOrg,
						"slug":         trimmedSlug,
						"repo_owner":   trimmedRepoOrg,
						"repo_name":    trimmedRepo,
						"permission":   trimmedPermission,
					},
				)
			}

			if cmd.Flags().Changed("to-config") {
				return addTeamRepoPermissionToConfig(cmd, toConfig, trimmedOrg, trimmedSlug, trimmedRepoOrg, trimmedRepo, trimmedPermission)
			}

			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}

			opts := teams.AddTeamRepoPermissionBySlugOptions{
				Service:    service,
				Org:        trimmedOrg,
				Slug:       trimmedSlug,
				RepoOwner:  trimmedRepoOrg,
				RepoName:   trimmedRepo,
				Permission: teams.TeamRepoPermission(trimmedPermission),
			}

			if err := teams.AddTeamRepoPermissionBySlug(ctx, opts); err != nil {
				return err
			}

			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf(
					"Granted team %s/%s permission %s on repository %s/%s",
					trimmedOrg,
					trimmedSlug,
					trimmedPermission,
					trimmedRepoOrg,
					trimmedRepo,
				),
				map[string]any{
					"organization": trimmedOrg,
					"slug":         trimmedSlug,
					"repo_owner":   trimmedRepoOrg,
					"repo_name":    trimmedRepo,
					"permission":   trimmedPermission,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	cmd.Flags().StringVar(&repoOrg, "repo-org", "", "Owner organization of the target repository (defaults to --org)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name")
	cmd.Flags().StringVar(&permission, "permission", string(teams.TeamRepoPermissionPull), "Permission to grant: pull, push, admin, maintain, triage")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug", "repo")

	return cmd
}

func addTeamRepoPermissionToConfig(cmd *cobra.Command, path, org, slug, repoOwner, repoName, permission string) error {
	changed, err := configproposal.ApplyToConfigFile(path, org, func(cfg *gitopsconfig.OrganizationConfig) error {
		teamIndex, found := configproposal.FindTeamIndex(cfg, slug)
		if !found {
			return fmt.Errorf("team %s/%s not found in config", org, slug)
		}

		team := &cfg.Teams[teamIndex]
		if existing, found := configproposal.FindTeamRepositoryIndex(team, cfg.Organization, repoOwner, repoName); found {
			team.Repositories[existing].Permission = permission
			return nil
		}
		team.Repositories = append(team.Repositories, gitopsconfig.TeamRepositorySpec{
			Owner:      repoOwner,
			Name:       repoName,
			Permission: permission,
		})
		return nil
	})
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Proposed repository permission for team %s/%s in config", org, slug)
	if !changed {
		message = fmt.Sprintf("No changes needed for repository permission %s/%s", org, slug)
	}
	return cmdoutput.PrintSuccess(cmd, message, map[string]any{
		"organization": org,
		"slug":         slug,
		"repo_owner":   repoOwner,
		"repo_name":    repoName,
		"permission":   permission,
		"config_path":  path,
		"changed":      changed,
	})
}
