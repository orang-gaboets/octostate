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
		toConfig        string
	)

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit an existing GitHub repository",
		Long:  "Edit an existing GitHub repository by updating its description, homepage, privacy settings, template status, archived status, and forking permissions.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate repo edit --org <org> --name <repo-name> --desc "New description" --homepage "https://example.com" --private=true --is-template=false --archived=false --allow-forking=true
			octostate repo edit --org <org> --name <repo-name> --desc "New description" --dry-run
			octostate repo edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path> --org <org> --name <repo-name> --desc "New description" --homepage "https://example.com" --private=true --is-template=false --archived=false --allow-forking=true`,
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
			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
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
			if cmd.Flags().Changed("to-config") {
				return editRepoToConfig(cmd, toConfig, org, name, editConfigValues{
					desc:         newDesc,
					homepage:     newHomepage,
					private:      newPrivate,
					isTemplate:   newIsTemplate,
					archived:     newArchived,
					allowForking: newAllowForking,
				})
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
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}

type editConfigValues struct {
	desc         string
	homepage     string
	private      bool
	isTemplate   bool
	archived     bool
	allowForking bool
}

func editRepoToConfig(cmd *cobra.Command, path, org, name string, values editConfigValues) error {
	trimmedOrg := strings.TrimSpace(org)
	trimmedName := strings.TrimSpace(name)
	changedFields := []string{}
	changed, err := configproposal.ApplyToConfigFile(path, trimmedOrg, func(cfg *gitopsconfig.OrganizationConfig) error {
		index, found := configproposal.FindRepositoryIndex(cfg, trimmedOrg, trimmedName)
		if !found {
			return fmt.Errorf("repository %s/%s not found in config", trimmedOrg, trimmedName)
		}
		repository := &cfg.Repositories[index]
		before := *repository
		if cmd.Flags().Changed("desc") {
			repository.SetManagedDescription(values.desc)
		}
		if cmd.Flags().Changed("homepage") {
			repository.SetManagedHomepage(values.homepage)
		}
		if cmd.Flags().Changed("private") {
			if values.private {
				repository.Visibility = "private"
			} else {
				repository.Visibility = "public"
			}
		}
		if cmd.Flags().Changed("is-template") {
			repository.SetManagedIsTemplate(values.isTemplate)
		}
		if cmd.Flags().Changed("archived") {
			repository.SetManagedArchived(values.archived)
		}
		if cmd.Flags().Changed("allow-forking") {
			repository.SetManagedAllowForking(values.allowForking)
		}
		if cmd.Flags().Changed("desc") && before.DescriptionOption() != repository.DescriptionOption() {
			changedFields = append(changedFields, "desc")
		}
		if cmd.Flags().Changed("homepage") && before.HomepageOption() != repository.HomepageOption() {
			changedFields = append(changedFields, "homepage")
		}
		if cmd.Flags().Changed("private") && before.Visibility != repository.Visibility {
			changedFields = append(changedFields, "private")
		}
		if cmd.Flags().Changed("is-template") && before.IsTemplateOption() != repository.IsTemplateOption() {
			changedFields = append(changedFields, "is-template")
		}
		if cmd.Flags().Changed("archived") && before.ArchivedOption() != repository.ArchivedOption() {
			changedFields = append(changedFields, "archived")
		}
		if cmd.Flags().Changed("allow-forking") && before.AllowForkingOption() != repository.AllowForkingOption() {
			changedFields = append(changedFields, "allow-forking")
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !changed {
		changedFields = []string{}
	}
	message := fmt.Sprintf("Proposed repository %s/%s edit in config", trimmedOrg, trimmedName)
	if !changed {
		message = fmt.Sprintf("No changes needed for edit %s/%s", trimmedOrg, trimmedName)
	}
	return cmdoutput.PrintSuccess(cmd, message, map[string]any{
		"owner":          trimmedOrg,
		"name":           trimmedName,
		"config_path":    path,
		"changed":        changed,
		"changed_fields": changedFields,
	})
}
