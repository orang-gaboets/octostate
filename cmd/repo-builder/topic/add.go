package topic

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitHubClient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
)

// AddTopicsCmd creates a new command to add topics to an existing GitHub repository.
func AddTopicsCmd(svc topics.Service) *cobra.Command {
	var (
		token     string
		org       string
		name      string
		topicsStr string
	)

	cmd := &cobra.Command{
		Use:     "add",
		Aliases: []string{"add-topics"},
		Short:   "Add topics to a GitHub repository",
		Long:    "Add topics to a GitHub repository. This command adds new topics to the existing topics associated with a specified GitHub repository.",
		Example: "go run ./cmd/repo-builder topic add --token <token> --org <org> --name <name> --topics <topic1,topic2>",
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
			opts := topics.AddTopicsOptions{
				Repo:    github.Repository{Org: org, Name: name},
				Topics:  topicsList,
				Service: service,
			}
			_, err := topics.AddTopics(ctx, opts)
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")
	cmd.Flags().StringVar(&topicsStr, "topics", "", "Comma-separated list of topics to add to the repository")

	requiredFlags := []string{"token", "org", "name", "topics"}
	for _, flag := range requiredFlags {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			cobra.CheckErr(cmd.MarkFlagRequired(flag))
		}
	}

	return cmd
}
