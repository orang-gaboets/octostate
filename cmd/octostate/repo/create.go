package repo

import (
	"context"
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

// CreateNewRepoFromTemplateCmd creates a new command to create a GitHub repository from a template.
func CreateNewRepoFromTemplateCmd(svc repos.Service) *cobra.Command {
	return createRepoCmd(svc, true)
}

// CreateRepoCmd creates the general repository creation command.
func CreateRepoCmd(svc repos.Service) *cobra.Command {
	return createRepoCmd(svc, false)
}

func createRepoCmd(svc repos.Service, templateOnly bool) *cobra.Command {
	var (
		token              string
		appID              int64
		installationID     int64
		appKeyPath         string
		org                string
		templateName       string
		templateOrg        string
		name               string
		desc               string
		topics             string
		private            bool
		visibility         string
		includeAllBranches bool
		dryRun             bool
		toConfig           string
	)

	cmd := &cobra.Command{
		Use:     "create-from-template",
		Aliases: []string{"cft", "new-from-template"},
		Short:   "Create a GitHub repository from a template",
		Long:    "Create a new GitHub repository from a template repository, optionally specifying organization, name, description, topics, and privacy settings.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate repo create-from-template --org <org> --template-name <template-name> --name <new-repo-name> --desc "Repository description" --topics "topic1,topic2" --private=true --include-all-branches=true
			octostate repo create-from-template --org <org> --template-name <template-name> --name <new-repo-name> --dry-run
			octostate repo create-from-template --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --template-name <template-name> --name <new-repo-name> --desc "Repository description" --topics "topic1,topic2" --private=true --include-all-branches=true`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			selected, err := selectedVisibility(cmd, private, visibility)
			if err != nil {
				return err
			}
			var legacyPrivateOption *bool
			if cmd.Flags().Changed("private") {
				legacyPrivateOption = &private
			}
			trimmedOrg := strings.TrimSpace(org)
			trimmedTemplateName := strings.TrimSpace(templateName)
			if cmd.Flags().Changed("template-name") && trimmedTemplateName == "" {
				return fmt.Errorf("--template-name must not be empty")
			}
			trimmedTemplateOrg := strings.TrimSpace(templateOrg)
			trimmedName := strings.TrimSpace(name)
			var topicList []string
			if topics != "" {
				topicList = strings.Split(strings.TrimSpace(topics), ",")
			}
			if trimmedTemplateOrg == "" {
				trimmedTemplateOrg = trimmedOrg
			}
			if !templateOnly {
				hasTemplateName := trimmedTemplateName != ""
				hasTemplateOrg := strings.TrimSpace(templateOrg) != ""
				if hasTemplateOrg && !hasTemplateName {
					return fmt.Errorf("--template-org requires --template-name")
				}
				if !hasTemplateName && includeAllBranches {
					return fmt.Errorf("--include-all-branches requires --template-name")
				}
			}
			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun && selected == "internal" && (templateOnly || trimmedTemplateName != "") {
				return fmt.Errorf("internal visibility is unsupported for template-based repository creation")
			}
			if dryRun {
				if !templateOnly && trimmedTemplateName == "" {
					return cmdoutput.PrintDryRun(cmd, fmt.Sprintf("Dry run: would create repository %s/%s (visibility=%s topics=%v)", trimmedOrg, trimmedName, selected, topicList), map[string]any{"owner": trimmedOrg, "name": trimmedName, "visibility": selected, "private": selected == "private", "topics": topicList})
				}
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf(
						"Dry run: would create repository %s/%s from template %s/%s (visibility=%s include-all-branches=%t topics=%v)",
						trimmedOrg,
						trimmedName,
						trimmedTemplateOrg,
						trimmedTemplateName,
						selected,
						includeAllBranches,
						topicList,
					),
					map[string]any{
						"owner":                trimmedOrg,
						"name":                 trimmedName,
						"template_owner":       trimmedTemplateOrg,
						"template_repo":        trimmedTemplateName,
						"visibility":           selected,
						"private":              selected == "private",
						"include_all_branches": includeAllBranches,
						"topics":               topicList,
					},
				)
			}
			if cmd.Flags().Changed("to-config") {
				normalizedTopics, err := normalizeConfigTopics(topicList)
				if err != nil {
					return err
				}
				_, err = configproposal.ApplyToConfigFile(toConfig, trimmedOrg, func(cfg *gitopsconfig.OrganizationConfig) error {
					if _, exists := configproposal.FindRepositoryIndex(cfg, trimmedOrg, trimmedName); exists {
						return fmt.Errorf("repository %s/%s already exists in config", trimmedOrg, trimmedName)
					}
					repository := gitopsconfig.RepositorySpec{
						Owner:      trimmedOrg,
						Name:       trimmedName,
						Visibility: selected,
						Topics:     normalizedTopics,
					}
					if trimmedTemplateName != "" {
						repository.Template = gitopsconfig.TemplateSpec{Owner: trimmedTemplateOrg, Name: trimmedTemplateName, IncludeAllBranches: includeAllBranches}
					}
					repository.SetManagedDescription(desc)
					cfg.Repositories = append(cfg.Repositories, repository)
					return nil
				})
				if err != nil {
					return err
				}
				result := map[string]any{
					"owner":                trimmedOrg,
					"name":                 trimmedName,
					"config_path":          toConfig,
					"changed":              true,
					"visibility":           selected,
					"private":              selected == "private",
					"include_all_branches": includeAllBranches,
					"topics":               normalizedTopics,
				}
				if trimmedTemplateName != "" {
					result["template_owner"] = trimmedTemplateOrg
					result["template_repo"] = trimmedTemplateName
				}
				return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Proposed repository %s/%s in config", trimmedOrg, trimmedName), result)
			}
			return createRepositoryLive(ctx, cmd, svc, token, appID, installationID, appKeyPath, templateOnly, trimmedOrg, trimmedTemplateOrg, trimmedTemplateName, trimmedName, desc, topicList, selected, legacyPrivateOption, includeAllBranches)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&templateName, "template-name", "", "Template repository name")
	cmd.Flags().StringVar(&templateOrg, "template-org", "", "Template repository organization name (defaults to --org if not set)")
	cmd.Flags().StringVar(&name, "name", "", "New repository name")
	cmd.Flags().StringVar(&desc, "desc", "", "Repository description")
	cmd.Flags().StringVar(&topics, "topics", "", "Comma-separated list of topics")
	cmd.Flags().BoolVar(&private, "private", false, "Create repository as private")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Repository visibility: public, private, or internal")
	cmd.Flags().BoolVar(&includeAllBranches, "include-all-branches", false, "Include all branches from the template repository")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	if templateOnly {
		github.MarkRequiredFlags(cmd, "org", "template-name", "name")
	} else {
		cmd.Use = "create"
		cmd.Aliases = []string{"new"}
		cmd.Short = "Create a GitHub repository, optionally from a template"
		cmd.Long = "Create a new GitHub repository, optionally from a template repository, with organization, name, description, topics, and privacy settings."
		cmd.Example = `
			octostate repo create --org <org> --name <new-repo-name> --desc "Repository description" --topics "topic1,topic2" --private=true
			octostate repo create --org <org> --template-name <template-name> --name <new-repo-name> --include-all-branches=true
			octostate repo create --org <org> --name <new-repo-name> --dry-run`
		github.MarkRequiredFlags(cmd, "org", "name")
	}

	return cmd
}

func createRepositoryLive(ctx context.Context, cmd *cobra.Command, svc repos.Service, token string, appID, installationID int64, appKeyPath string, templateOnly bool, org, templateOrg, templateName, name, desc string, topicList []string, visibility string, legacyPrivate *bool, includeAllBranches bool) error {
	service := svc
	if service == nil {
		client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
		if err != nil {
			return err
		}
		service = client.Repositories()
	}
	if !templateOnly && templateName == "" {
		createdRepo, err := repos.Create(ctx, repos.CreateOptions{Service: service, Name: name, Owner: org, Description: &desc, Visibility: &visibility, Private: legacyPrivate, Topics: topicList})
		if err != nil {
			return err
		}
		return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Created repository %s/%s", org, name), map[string]any{"owner": org, "name": name, "visibility": visibility, "private": visibility == "private", "topics": topicList, "repository": createdRepo})
	}
	if visibility == "internal" {
		return fmt.Errorf("internal visibility is unsupported for template-based repository creation")
	}
	createdRepo, err := repos.CreateFromTemplate(ctx, repos.CreateFromTemplateOptions{
		Name: name, Owner: org, TemplateRepo: templateName, TemplateOwner: templateOrg,
		Description: &desc, Private: github.Ptr(visibility == "private"), Topics: topicList,
		IncludeAllBranches: includeAllBranches, Service: service,
	})
	if err != nil {
		return err
	}
	return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Created repository %s/%s from template %s/%s", org, name, templateOrg, templateName), map[string]any{
		"owner": org, "name": name, "template_owner": templateOrg, "template_repo": templateName,
		"visibility": visibility, "private": visibility == "private", "include_all_branches": includeAllBranches,
		"topics": topicList, "repository": createdRepo,
	})
}

func normalizeConfigTopics(topics []string) ([]string, error) {
	normalized := make([]string, 0, len(topics))
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			return nil, fmt.Errorf("topic cannot be empty")
		}
		normalized = append(normalized, topic)
	}
	return normalized, nil
}
