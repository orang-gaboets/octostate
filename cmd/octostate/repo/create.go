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

// CreateNewRepoFromTemplateCmd creates a new command to create a GitHub repository from a template.
func CreateNewRepoFromTemplateCmd(svc repos.Service) *cobra.Command {
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
		includeAllBranches bool
		dryRun             bool
		toConfig           string
	)

	cmd := &cobra.Command{
		Use:     "create-from-template",
		Aliases: []string{"cft", "new-from-template", "create", "new"},
		Short:   "Create GitHub repositories from a template",
		Long:    "Create a new GitHub repository from a template repository, optionally specifying organization, name, description, topics, and privacy settings.",
		Example: `
			octostate repo create-from-template --token <token> --org <org> --template-name <template-name> --name <new-repo-name> --desc "Repository description" --topics "topic1,topic2" --private=true --include-all-branches=true
			octostate repo create-from-template --org <org> --template-name <template-name> --name <new-repo-name> --dry-run
			octostate repo create-from-template --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --template-name <template-name> --name <new-repo-name> --desc "Repository description" --topics "topic1,topic2" --private=true --include-all-branches=true`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			trimmedOrg := strings.TrimSpace(org)
			trimmedTemplateName := strings.TrimSpace(templateName)
			trimmedTemplateOrg := strings.TrimSpace(templateOrg)
			trimmedName := strings.TrimSpace(name)
			var topicList []string
			if topics != "" {
				topicList = strings.Split(strings.TrimSpace(topics), ",")
			}
			if trimmedTemplateOrg == "" {
				trimmedTemplateOrg = trimmedOrg
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf(
						"Dry run: would create repository %s/%s from template %s/%s (private=%t include-all-branches=%t topics=%v)",
						trimmedOrg,
						trimmedName,
						trimmedTemplateOrg,
						trimmedTemplateName,
						private,
						includeAllBranches,
						topicList,
					),
					map[string]any{
						"owner":                trimmedOrg,
						"name":                 trimmedName,
						"template_owner":       trimmedTemplateOrg,
						"template_repo":        trimmedTemplateName,
						"private":              private,
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
				visibility := "public"
				if private {
					visibility = "private"
				}
				_, err = configproposal.ApplyToConfigFile(toConfig, trimmedOrg, func(cfg *gitopsconfig.OrganizationConfig) error {
					if _, exists := configproposal.FindRepositoryIndex(cfg, trimmedOrg, trimmedName); exists {
						return fmt.Errorf("repository %s/%s already exists in config", trimmedOrg, trimmedName)
					}
					repository := gitopsconfig.RepositorySpec{
						Owner: trimmedOrg,
						Name:  trimmedName,
						Template: gitopsconfig.TemplateSpec{
							Owner:              trimmedTemplateOrg,
							Name:               trimmedTemplateName,
							IncludeAllBranches: includeAllBranches,
						},
						Visibility: visibility,
						Topics:     normalizedTopics,
					}
					repository.SetManagedDescription(desc)
					cfg.Repositories = append(cfg.Repositories, repository)
					return nil
				})
				if err != nil {
					return err
				}
				return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Proposed repository %s/%s in config", trimmedOrg, trimmedName), map[string]any{
					"owner":                trimmedOrg,
					"name":                 trimmedName,
					"config_path":          toConfig,
					"changed":              true,
					"template_owner":       trimmedTemplateOrg,
					"template_repo":        trimmedTemplateName,
					"private":              private,
					"include_all_branches": includeAllBranches,
					"topics":               normalizedTopics,
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
			opts := repos.CreateFromTemplateOptions{
				Name:               trimmedName,
				Owner:              trimmedOrg,
				TemplateRepo:       trimmedTemplateName,
				TemplateOwner:      trimmedTemplateOrg,
				Description:        &desc,
				Private:            &private,
				Topics:             topicList,
				IncludeAllBranches: includeAllBranches,
				Service:            service,
			}
			createdRepo, err := repos.CreateFromTemplate(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf(
					"Created repository %s/%s from template %s/%s",
					trimmedOrg,
					trimmedName,
					trimmedTemplateOrg,
					trimmedTemplateName,
				),
				map[string]any{
					"owner":                trimmedOrg,
					"name":                 trimmedName,
					"template_owner":       trimmedTemplateOrg,
					"template_repo":        trimmedTemplateName,
					"private":              private,
					"include_all_branches": includeAllBranches,
					"topics":               topicList,
					"repository":           createdRepo,
				},
			)
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
	cmd.Flags().BoolVar(&includeAllBranches, "include-all-branches", false, "Include all branches from the template repository")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "template-name", "name")

	return cmd
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
