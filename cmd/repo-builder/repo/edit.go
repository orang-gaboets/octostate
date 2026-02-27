package repo

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/safety"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

// EditRepo creates a new command to edit an existing GitHub repository.
func EditRepo(svc repos.Service) *cobra.Command {
	var (
		token           string
		appID           int64
		installationID  int64
		appKeyPath      string
		org             string
		name            string
		newDesc         string
		newHomepage     string
		newPrivate      bool
		newIsTemplate   bool
		newArchived     bool
		newAllowForking bool
		dryRun          bool
	)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit an existing GitHub repository",
		Long:  "Edit an existing GitHub repository by updating its description, homepage, privacy settings, template status, archived status, and forking permissions.",
		Example: `
			repo-builder repo edit --token <token> --org <org> --name <repo-name> --desc "New description" --homepage "https://example.com" --private=true --is-template=false --archived=false --allow-forking=true
			repo-builder repo edit --org <org> --name <repo-name> --desc "New description" --dry-run
			repo-builder repo edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path> --org <org> --name <repo-name> --desc "New description" --homepage "https://example.com" --private=true --is-template=false --archived=false --allow-forking=true`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			var opts = repos.EditOptions{
				Repo:  name,
				Owner: org,
			}
			var changedFields []string
			if cmd.Flags().Changed("desc") {
				opts.Description = &newDesc
				changedFields = append(changedFields, "desc")
			}
			if cmd.Flags().Changed("homepage") {
				opts.Homepage = &newHomepage
				changedFields = append(changedFields, "homepage")
			}
			if cmd.Flags().Changed("private") {
				opts.Private = &newPrivate
				changedFields = append(changedFields, "private")
			}
			if cmd.Flags().Changed("is-template") {
				opts.IsTemplate = &newIsTemplate
				changedFields = append(changedFields, "is-template")
			}
			if cmd.Flags().Changed("archived") {
				opts.Archived = &newArchived
				changedFields = append(changedFields, "archived")
			}
			if cmd.Flags().Changed("allow-forking") {
				opts.AllowForking = &newAllowForking
				changedFields = append(changedFields, "allow-forking")
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf(
						"Dry run: would edit repository %s/%s (changed=%s)",
						org,
						name,
						strings.Join(changedFields, ","),
					),
					map[string]any{
						"owner":          org,
						"name":           name,
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
				service = client.Repositories()
			}
			opts.Service = service

			updatedRepo, err := repos.Edit(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Edited repository %s/%s", org, name),
				map[string]any{
					"owner":          org,
					"name":           name,
					"changed_fields": changedFields,
					"repository":     updatedRepo,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "Name of the repository to edit")
	cmd.Flags().StringVar(&newDesc, "desc", "", "New description for the repository")
	cmd.Flags().StringVar(&newHomepage, "homepage", "", "New homepage URL for the repository")
	cmd.Flags().BoolVar(&newPrivate, "private", false, "Set the repository to private")
	cmd.Flags().BoolVar(&newIsTemplate, "is-template", false, "Set the repository as a template")
	cmd.Flags().BoolVar(&newArchived, "archived", false, "Archive the repository")
	cmd.Flags().BoolVar(&newAllowForking, "allow-forking", false, "Allow private forking of the repository")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}
