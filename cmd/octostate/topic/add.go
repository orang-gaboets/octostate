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
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "name", "topics")

	return cmd
}
