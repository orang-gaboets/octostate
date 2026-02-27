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
	)

	cmd := &cobra.Command{
		Use:     "remove",
		Aliases: []string{"delete", "del", "rm", "revoke"},
		Short:   "Remove repository permission from a GitHub team",
		Long:    "Remove a GitHub team's access to a repository.",
		Example: `
			repo-builder team repo permissions remove --token <token> --org <org-name> --slug <team-slug> --repo <repo-name>
			repo-builder team repo permissions remove --token <token> --org <org-name> --slug <team-slug> --repo-org <repo-org> --repo <repo-name> --dry-run
			repo-builder team repo permissions remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --slug <team-slug> --repo <repo-name>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			trimmedOrg := strings.TrimSpace(org)
			trimmedSlug := strings.TrimSpace(slug)
			trimmedRepoOrg := strings.TrimSpace(repoOrg)
			trimmedRepo := strings.TrimSpace(repo)

			if trimmedRepoOrg == "" {
				trimmedRepoOrg = trimmedOrg
			}
			if trimmedRepo == "" {
				return fmt.Errorf("repo name cannot be empty: %w", github.ErrMissingRequiredField)
			}

			if dryRun {
				_, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"Dry run: would remove team %s/%s access to repository %s/%s\n",
					trimmedOrg,
					trimmedSlug,
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

			_, err := fmt.Fprintf(
				cmd.OutOrStdout(),
				"Removed team %s/%s access to repository %s/%s\n",
				trimmedOrg,
				trimmedSlug,
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
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug", "repo")

	return cmd
}
