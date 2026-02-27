package permissions

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/safety"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
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
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"grant", "set"},
		Short:   "Grant repository permission to a GitHub team",
		Long:    "Grant or update a GitHub team's permission on a repository.",
		Example: `
			repo-builder team repo permissions add --token <token> --org <org-name> --slug <team-slug> --repo <repo-name> --permission push
			repo-builder team repo permissions add --token <token> --org <org-name> --slug <team-slug> --repo-org <repo-org> --repo <repo-name> --permission maintain --dry-run
			repo-builder team repo permissions add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug> --repo <repo-name> --permission pull`,
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

			if dryRun {
				_, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"Dry run: would grant team %s/%s permission %s on repository %s/%s\n",
					trimmedOrg,
					trimmedSlug,
					trimmedPermission,
					trimmedRepoOrg,
					trimmedRepo,
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

			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"Granted team %s/%s permission %s on repository %s/%s\n",
				trimmedOrg,
				trimmedSlug,
				trimmedPermission,
				trimmedRepoOrg,
				trimmedRepo,
			)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	cmd.Flags().StringVar(&repoOrg, "repo-org", "", "Owner organization of the target repository (defaults to --org)")
	cmd.Flags().StringVar(&repo, "repo", "", "Repository name")
	cmd.Flags().StringVar(&permission, "permission", string(teams.TeamRepoPermissionPull), "Permission to grant: pull, push, admin, maintain, triage")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug", "repo")

	return cmd
}
