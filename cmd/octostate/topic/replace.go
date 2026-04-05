package topic

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/topics"
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
	)

	cmd := &cobra.Command{
		Use:     "replace",
		Aliases: []string{"replace-all"},
		Short:   "Replace all topics in a GitHub repository",
		Long:    "Replace all topics in a GitHub repository. This command replaces the existing topics associated with a specified GitHub repository with new ones.",
		Example: `
			octostate topic replace --token <token> --org <org> --name <name> --topics <topic1,topic2>
			octostate topic replace --org <org> --name <name> --topics <topic1,topic2> --dry-run
			octostate topic replace --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <name> --topics <topic1,topic2>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			var topicsList []string
			if topicsStr != "" {
				topicsList = strings.Split(strings.TrimSpace(topicsStr), ",")
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
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "name", "topics")

	return cmd
}
