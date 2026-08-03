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

// AddTopicsCmd creates a new command to add topics to an existing GitHub repository.
func AddTopicsCmd(svc topics.Service) *cobra.Command {
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
		Use:     "add",
		Aliases: []string{"add-topics"},
		Short:   "Add topics to a GitHub repository",
		Long:    "Add topics to a GitHub repository. This command adds new topics to the existing topics associated with a specified GitHub repository.",
		Example: `
			octostate topic add --token <token> --org <org> --name <name> --topics <topic1,topic2>
			octostate topic add --org <org> --name <name> --topics <topic1,topic2> --dry-run
			octostate topic add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <name> --topics <topic1,topic2>`,
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
					fmt.Sprintf("Dry run: would add topics to repository %s/%s (topics=%v)", org, name, topicsList),
					map[string]any{
						"owner":  org,
						"name":   name,
						"topics": topicsList,
					},
				)
			}
			if cmd.Flags().Changed("to-config") {
				normalizedTopics, err := normalizeConfigTopicAddInput(topicsStr)
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
					repository.Topics = github.MergeUnique(repository.Topics, normalizedTopics)
					resultingTopics = repository.Topics
					return nil
				})
				if err != nil {
					return err
				}
				trimmedOrg := strings.TrimSpace(org)
				trimmedName := strings.TrimSpace(name)
				message := fmt.Sprintf("Proposed topics add for repository %s/%s in config", trimmedOrg, trimmedName)
				if !changed {
					message = fmt.Sprintf("No changes needed for add topics %s/%s", trimmedOrg, trimmedName)
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
			opts := topics.AddTopicsOptions{
				Repo:    name,
				Owner:   org,
				Topics:  topicsList,
				Service: service,
			}
			updatedTopics, err := topics.AddTopics(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Added topics to repository %s/%s", org, name),
				map[string]any{
					"owner":  org,
					"name":   name,
					"topics": updatedTopics,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")
	cmd.Flags().StringVar(&topicsStr, "topics", "", "Comma-separated list of topics to add to the repository")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "name", "topics")

	return cmd
}

func normalizeConfigTopicAddInput(value string) ([]string, error) {
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
