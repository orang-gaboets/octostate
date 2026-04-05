package topic

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/topics"
)

// ListAllTopicsCmd creates a command to list all topics of an existing GitHub repository.
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
			octostate topic list --token <token> --org <org> --name <name>
			octostate topic list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <name>`,
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
				Repo:    name,
				Owner:   org,
				Service: service,
			}
			allTopics, err := topics.ListAllTopics(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, allTopics)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "GitHub repository name")

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}
