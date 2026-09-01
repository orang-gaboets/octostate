package topic

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/configproposal"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/topics"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// ReplaceAllTopicsCmd creates a new command to replace all topics of an existing GitHub repository.
func ReplaceAllTopicsCmd(svc topics.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		name           string
		topicsStr      string
		dryRun         bool
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "replace",
		Aliases: []string{"replace-all"},
		Short:   "Replace all topics in a GitHub repository",
		Long:    "Replace all topics in a GitHub repository. This command replaces the existing topics associated with a specified GitHub repository with new ones.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate topic replace --org <org> --name <name> --topics <topic1,topic2>
			octostate topic replace --org <org> --name <name> --topics <topic1,topic2> --dry-run
			octostate topic replace --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <name> --topics <topic1,topic2>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			var topicsList []string
			if topicsStr != "" {
				topicsList = strings.Split(strings.TrimSpace(topicsStr), ",")
			}
			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("Dry run: would replace topics for repository %s/%s (topics=%v)", org, name, topicsList),
					map[string]any{
						"owner":  org,
						"name":   name,
						"topics": topicsList,
					},
				)
			}
			if cmd.Flags().Changed("to-config") {
				normalizedTopics, err := normalizeConfigTopicReplaceInput(topicsStr)
				if err != nil {
					return err
				}
				var resultingTopics []string
				changed, err := configproposal.ApplyToConfigFile(toConfig, strings.TrimSpace(org), func(cfg *gitopsconfig.OrganizationConfig) error {
					index, found := configproposal.FindRepositoryIndex(cfg, org, name)
					if !found {
						return fmt.Errorf("repository %s/%s not found in config", strings.TrimSpace(org), strings.TrimSpace(name))
					}
					repository := &cfg.Repositories[index]
					replacement := github.Unique(normalizedTopics)
					current := github.Unique(repository.Topics)
					if github.EqualSets(github.ToSet(current), github.ToSet(replacement)) {
						replacement = current
					}
					repository.Topics = replacement
					resultingTopics = repository.Topics
					return nil
				})
				if err != nil {
					return err
				}
				trimmedOrg := strings.TrimSpace(org)
				trimmedName := strings.TrimSpace(name)
				message := fmt.Sprintf("Proposed topics replace for repository %s/%s in config", trimmedOrg, trimmedName)
				if !changed {
					message = fmt.Sprintf("No changes needed for replace topics %s/%s", trimmedOrg, trimmedName)
				}
				return cmdoutput.PrintSuccess(cmd, message, map[string]any{
					"owner":       trimmedOrg,
					"name":        trimmedName,
					"config_path": toConfig,
					"changed":     changed,
					"topics":      resultingTopics,
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
			opts := topics.ReplaceAllTopicsOptions{
				Repo:    name,
				Owner:   org,
				Topics:  topicsList,
				Service: service,
			}
			replacedTopics, err := topics.ReplaceAllTopics(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Replaced topics for repository %s/%s", org, name),
				map[string]any{
					"owner":  org,
					"name":   name,
					"topics": replacedTopics,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")
	cmd.Flags().StringVar(&topicsStr, "topics", "", "Comma-separated list of topics to replace in the repository")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "name", "topics")

	return cmd
}

func normalizeConfigTopicReplaceInput(value string) ([]string, error) {
	if value == "" {
		return []string{}, nil
	}
	parts := strings.Split(value, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		topic := strings.TrimSpace(part)
		if topic == "" {
			return nil, fmt.Errorf("topic cannot be empty")
		}
		normalized = append(normalized, topic)
	}
	return normalized, nil
}
