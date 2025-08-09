package topic

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitHubClient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
)

// ReplaceAllTopicsCmd creates a new command to replace all topics of an existing GitHub repository.
func ReplaceAllTopicsCmd(svc topics.Service) *cobra.Command {
	var (
		token     string
		org       string
		name      string
		topicsStr string
	)

	cmd := &cobra.Command{
		Use:     "replace",
		Aliases: []string{"replace-all"},
		Short:   "Replace all topics in a GitHub repository",
		Long:    "Replace all topics in a GitHub repository. This command replaces the existing topics associated with a specified GitHub repository with new ones.",
		Example: "go run ./cmd/repo-builder topic replace --token <token> --org <org> --name <name> --topics <topic1,topic2>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client := gitHubClient.New(ctx, token)
				service = client.Repositories
			}
			var topicsList []string
			if topicsStr != "" {
				topicsList = strings.Split(topicsStr, ",")
			}
			opts := topics.ReplaceAllTopicsOptions{
				Repo: github.Repository{
					Org:  org,
					Name: name,
				},
				Topics:  topicsList,
				Service: service,
			}
			_, err := topics.ReplaceAllTopics(ctx, opts)
			return err
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")
	cmd.Flags().StringVar(&topicsStr, "topics", "", "Comma-separated list of topics to replace in the repository")

	github.MarkRequiredFlags(cmd, "token", "org", "name", "topics")

	return cmd
}
