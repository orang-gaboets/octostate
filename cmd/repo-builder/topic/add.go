package topic

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
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
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"add-topics"},
		Short:   "Add topics to a GitHub repository",
		Long:    "Add topics to a GitHub repository. This command adds new topics to the existing topics associated with a specified GitHub repository.",
		Example: `
			repo-builder topic add --token <token> --org <org> --name <name> --topics <topic1,topic2>
			repo-builder topic add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <name> --topics <topic1,topic2>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Repositories()
			}
			var topicsList []string
			if topicsStr != "" {
				topicsList = strings.Split(topicsStr, ",")
			}
			opts := topics.AddTopicsOptions{
				Repo:    github.Repository{Org: org, Name: name},
				Topics:  topicsList,
				Service: service,
			}
			_, err := topics.AddTopics(ctx, opts)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")
	cmd.Flags().StringVar(&topicsStr, "topics", "", "Comma-separated list of topics to add to the repository")

	github.MarkRequiredFlags(cmd, "org", "name", "topics")

	return cmd
}
