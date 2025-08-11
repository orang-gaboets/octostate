package topic

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
)

// ListAllTopicsCmd lists a new command to list all topics of an existing GitHub repository.
func ListAllTopicsCmd(svc topics.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		name           string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"list-all"},
		Short:   "List all topics in a GitHub repository",
		Long:    "List all topics in a GitHub repository. This command retrieves and displays all topics associated with a specified GitHub repository.",
		Example: `
			repo-builder topic list --token <token> --org <org> --name <name>
			repo-builder topic list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <name>`,
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

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}
