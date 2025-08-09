package topic

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitHubClient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
)

// ListAllTopicsCmd lists a new command to list all topics of an existing GitHub repository.
func ListAllTopicsCmd(svc topics.Service) *cobra.Command {
	var (
		token string
		org   string
		name  string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"list-all"},
		Short:   "List all topics in a GitHub repository",
		Long:    "List all topics in a GitHub repository. This command retrieves and displays all topics associated with a specified GitHub repository.",
		Example: "go run ./cmd/repo-builder topic list --token <token> --org <org> --name <name>",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client := gitHubClient.New(ctx, token)
				service = client.Repositories
			}
			opts := topics.ListAllTopicsOptions{
				Repo: github.Repository{
					Org:  org,
					Name: name,
				},
				Service: service,
			}
			_, err := topics.ListAllTopics(ctx, opts)
			return err
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")

	github.MarkRequiredFlags(cmd, "token", "org", "name")

	return cmd
}
