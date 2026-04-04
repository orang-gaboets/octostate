package team

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
)

// EditTeamCmd creates a new command to edit a GitHub team by slug.
func EditTeamCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		slug           string
		name           string
		desc           string
		secret         bool
		parent         string
		clearParent    bool
		dryRun         bool
	)

	cmd := &cobra.Command{
		Use:     "edit",
		Aliases: []string{"update", "patch"},
		Short:   "Edit an existing GitHub team",
		Long:    "Edit an existing GitHub team by updating its name, description, privacy, and parent team.",
		Example: `
			octostate team edit --token <token> --org <org> --slug <team-slug> --desc "Updated description"
			octostate team edit --token <token> --org <org> --slug <team-slug> --parent <parent-team-slug>
			octostate team edit --token <token> --org <org> --slug <team-slug> --clear-parent --dry-run
			octostate team edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --name <new-team-name> --secret=false`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			nameChanged := cmd.Flags().Changed("name")
			descChanged := cmd.Flags().Changed("desc")
			secretChanged := cmd.Flags().Changed("secret")
			parentChanged := cmd.Flags().Changed("parent")

			trimmedOrg := strings.TrimSpace(org)
			trimmedSlug := strings.TrimSpace(slug)
			trimmedName := strings.TrimSpace(name)
			trimmedParent := strings.TrimSpace(parent)

			var namePtr *string
			if nameChanged {
				namePtr = &trimmedName
			}
			var descPtr *string
			if descChanged {
				descPtr = &desc
			}
			var privacyPtr *github.TeamPrivacy
			if secretChanged {
				privacy := github.PrivacyFromBool(secret)
				privacyPtr = &privacy
			}
			var parentPtr *string
			if parentChanged {
				parentPtr = &trimmedParent
			}

			opts := teams.EditTeamBySlugOptions{
				Org:            trimmedOrg,
				Slug:           trimmedSlug,
				Name:           namePtr,
				Description:    descPtr,
				Privacy:        privacyPtr,
				ParentTeamSlug: parentPtr,
				RemoveParent:   clearParent,
			}

			if parentChanged && clearParent {
				return fmt.Errorf("cannot set --parent and --clear-parent together: %w", github.ErrValidationFailed)
			}
			if nameChanged && trimmedName == "" {
				return fmt.Errorf("team name cannot be empty: %w", github.ErrMissingRequiredField)
			}
			if parentChanged && trimmedParent == "" {
				return fmt.Errorf("parent team slug cannot be empty: %w", github.ErrMissingRequiredField)
			}

			changedFields := make([]string, 0, 5)
			if nameChanged {
				changedFields = append(changedFields, "name")
			}
			if descChanged {
				changedFields = append(changedFields, "desc")
			}
			if secretChanged {
				changedFields = append(changedFields, "secret")
			}
			if parentChanged {
				changedFields = append(changedFields, "parent")
			}
			if clearParent {
				changedFields = append(changedFields, "clear-parent")
			}
			if len(changedFields) == 0 {
				changedFields = append(changedFields, "<none>")
			}

			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("Dry run: would edit team %s/%s (changed=%s)", trimmedOrg, trimmedSlug, strings.Join(changedFields, ",")),
					map[string]any{
						"organization":   trimmedOrg,
						"slug":           trimmedSlug,
						"changed_fields": changedFields,
					},
				)
			}

			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}

			opts.Service = service
			teamInfo, err := teams.EditTeamBySlug(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Edited team %s/%s", trimmedOrg, trimmedSlug),
				map[string]any{
					"organization":   trimmedOrg,
					"slug":           trimmedSlug,
					"changed_fields": changedFields,
					"team":           teamInfo,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&slug, "slug", "", "Team slug")
	cmd.Flags().StringVar(&name, "name", "", "New team name")
	cmd.Flags().StringVar(&desc, "desc", "", "New team description")
	cmd.Flags().BoolVar(&secret, "secret", false, "Set team privacy to secret when true, closed when false")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent team slug to assign (optional)")
	cmd.Flags().BoolVar(&clearParent, "clear-parent", false, "Remove the parent team relationship")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "slug")

	return cmd
}
