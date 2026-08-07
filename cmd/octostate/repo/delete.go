package repo

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/configproposal"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// DeleteRepoCmd creates a new command to delete a GitHub repository.
func DeleteRepoCmd(svc repos.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		name           string
		dryRun         bool
		yes            bool
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"del", "remove"},
		Short:   "Delete a GitHub repository",
		Long:    "Delete a specified GitHub repository by providing the organization and repository name.",
		Example: `
			# Proposal mode (--to-config; not with --dry-run)
			octostate repo delete --org <org> --name <repo-name> --to-config <path-to-organization.yaml>

			# Live mode (auth required; --yes required)
			octostate repo delete --token <token> --org <org> --name <repo-name> --yes
			octostate repo delete --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --yes

			# Dry-run mode (--dry-run; not with --to-config)
			octostate repo delete --org <org> --name <repo-name> --dry-run`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trimmedOrg := strings.TrimSpace(org)
			trimmedName := strings.TrimSpace(name)

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("Dry run: would delete repository %s/%s", trimmedOrg, trimmedName),
					map[string]any{
						"owner": trimmedOrg,
						"name":  trimmedName,
					},
				)
			}
			if cmd.Flags().Changed("to-config") {
				return deleteRepoToConfig(cmd, toConfig, trimmedOrg, trimmedName)
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
				service = client.Repositories()
			}

			opts := repos.DeleteOptions{
				Repo:    trimmedName,
				Owner:   trimmedOrg,
				Service: service,
			}

			if err := repos.Delete(ctx, opts); err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Deleted repository %s/%s", trimmedOrg, trimmedName),
				map[string]any{
					"owner": trimmedOrg,
					"name":  trimmedName,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name to delete")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)
	safety.AddYesFlag(cmd, &yes)

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}

func deleteRepoToConfig(cmd *cobra.Command, path, org, name string) error {
	changed, err := configproposal.ApplyToConfigFile(path, org, func(cfg *gitopsconfig.OrganizationConfig) error {
		index, found := configproposal.FindRepositoryIndex(cfg, org, name)
		if !found {
			return fmt.Errorf("repository %s/%s not found in config", org, name)
		}

		repository := cfg.Repositories[index]
		blockers := collectRepositoryDeleteBlockers(cfg, repository)
		if len(blockers) > 0 {
			return fmt.Errorf(
				"repository %s/%s cannot be deleted from config while team permissions exist: %s",
				repositoryDeleteOwner(cfg.Organization, repository.Owner),
				strings.TrimSpace(repository.Name),
				strings.Join(blockers, ", "),
			)
		}

		cfg.Repositories = append(cfg.Repositories[:index], cfg.Repositories[index+1:]...)
		return nil
	})
	if err != nil {
		return err
	}

	return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Proposed repository %s/%s deletion in config", org, name), map[string]any{
		"owner":       org,
		"name":        name,
		"config_path": path,
		"changed":     changed,
	})
}

func collectRepositoryDeleteBlockers(cfg *gitopsconfig.OrganizationConfig, repository gitopsconfig.RepositorySpec) []string {
	if cfg == nil {
		return nil
	}

	blockers := make([]string, 0)
	for _, team := range cfg.Teams {
		teamCopy := team
		repositoryIndex, found := configproposal.FindTeamRepositoryIndex(&teamCopy, cfg.Organization, repository.Owner, repository.Name)
		if !found {
			continue
		}

		repoPermission := teamCopy.Repositories[repositoryIndex]
		blockers = append(blockers, fmt.Sprintf(
			"%s(%s/%s:%s)",
			strings.TrimSpace(teamCopy.Slug),
			repositoryDeleteOwner(cfg.Organization, repoPermission.Owner),
			strings.TrimSpace(repoPermission.Name),
			strings.TrimSpace(repoPermission.Permission),
		))
	}
	return blockers
}

func repositoryDeleteOwner(organization, owner string) string {
	trimmedOwner := strings.TrimSpace(owner)
	if trimmedOwner == "" {
		return strings.TrimSpace(organization)
	}
	return trimmedOwner
}
