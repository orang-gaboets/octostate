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

// CreateTeamCmd creates a new command to create a GitHub team.
func CreateTeamCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		name           string
		desc           string
		secret         bool
		parent         string
		dryRun         bool
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"c", "new"},
		Short:   "Create a new GitHub team",
		Long:    "Create a new team in a GitHub organization.",
		Example: `
			octostate team create --token <token> --org <org> --name <team-name> --desc "Team description" --secret=false --parent <parent-team-slug>
			octostate team create --org <org> --name <team-name> --dry-run
			octostate team create --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <team-name> --desc "Team description" --secret=false --parent <parent-team-slug>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			privacy := github.PrivacyFromBool(secret)
			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf(
						"Dry run: would create team %s/%s (privacy=%s parent=%s)",
						org,
						name,
						privacy,
						parent,
					),
					map[string]any{
						"organization": org,
						"name":         name,
						"privacy":      privacy,
						"parent_slug":  parent,
					},
				)
			}

			if cmd.Flags().Changed("to-config") {
				return createTeamToConfig(cmd, toConfig, org, name, desc, string(privacy), parent)
			}

			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}

			opts := teams.CreateTeamOptions{
				Org:         org,
				Name:        name,
				Description: &desc,
				Privacy:     &privacy,
				Service:     service,
			}
			if cmd.Flags().Changed("parent") {
				opts.ParentTeamSlug = &parent
			}

			createdTeam, err := teams.CreateTeam(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Created team %s/%s", org, name),
				map[string]any{
					"organization": org,
					"name":         name,
					"privacy":      privacy,
					"parent_slug":  parent,
					"team":         createdTeam,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "Team name")
	cmd.Flags().StringVar(&desc, "desc", "", "Team description")
	cmd.Flags().BoolVar(&secret, "secret", false, "Create a secret team (default: false)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent team slug (optional)")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}

func createTeamToConfig(cmd *cobra.Command, path, org, name, desc, privacy, parent string) error {
	trimmedOrg := strings.TrimSpace(org)
	trimmedName := strings.TrimSpace(name)
	trimmedDesc := strings.TrimSpace(desc)
	slug := gitopsconfig.NormalizeTeamName(trimmedName)
	if slug == "" {
		return fmt.Errorf("team name %q does not produce a valid team slug: %w", name, github.ErrInvalidFieldValue)
	}

	parentProvided := cmd.Flags().Changed("parent")
	trimmedParent := strings.TrimSpace(parent)
	if parentProvided && trimmedParent == "" {
		return fmt.Errorf("parent team slug cannot be empty: %w", github.ErrMissingRequiredField)
	}

	parentSlug := ""
	_, err := configproposal.ApplyToConfigFile(path, trimmedOrg, func(cfg *gitopsconfig.OrganizationConfig) error {
		if _, exists := configproposal.FindTeamIndex(cfg, slug); exists {
			return fmt.Errorf("team %s already exists in config", slug)
		}
		if parentProvided {
			parentIndex, found := configproposal.FindTeamIndex(cfg, trimmedParent)
			if !found {
				return fmt.Errorf("parent team %s not found in config", trimmedParent)
			}
			parentSlug = strings.TrimSpace(cfg.Teams[parentIndex].Slug)
		}
		cfg.Teams = append(cfg.Teams, gitopsconfig.TeamSpec{
			Slug:        slug,
			Name:        trimmedName,
			Description: trimmedDesc,
			Privacy:     privacy,
			ParentSlug:  parentSlug,
		})
		return nil
	})
	if err != nil {
		return err
	}

	return cmdoutput.PrintSuccess(
		cmd,
		fmt.Sprintf("Proposed team %s/%s in config", trimmedOrg, slug),
		map[string]any{
			"organization": trimmedOrg,
			"name":         trimmedName,
			"slug":         slug,
			"privacy":      privacy,
			"parent_slug":  parentSlug,
			"config_path":  path,
			"changed":      true,
		},
	)
}
