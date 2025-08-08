package cmd

import (
	"context"
	"log"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitHubClient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

func CreateNewRepoFromTemplateCmd(svc repos.Service) *cobra.Command {
	var (
		token              string
		org                string
		templateName       string
		templateOrg        string
		name               string
		desc               string
		topics             string
		private            bool
		includeAllBranches bool
	)

	cmd := &cobra.Command{
		Use:     "create-repo-template",
		Short:   "Create GitHub repositories from a template",
		Long:    "Create a new GitHub repository from a template repository, optionally specifying organization, name, description, topics, and privacy settings.",
		Example: `repo-builder create-repo-template --token <token> --org <org> --template-name <template-name> --name <new-repo-name> --desc "Repository description" --topics "topic1,topic2" --private=true --include-all-branches=true`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			service := svc
			if service == nil {
				client := gitHubClient.New(ctx, token)
				service = client.Repositories
			}
			var topicList []string
			if topics != "" {
				topicList = strings.Split(topics, ",")
			}
			if templateOrg == "" {
				templateOrg = org
			}
			opts := repos.CreateFromTemplateOptions{
				NewRepo: github.Repository{
					Org:         org,
					Name:        name,
					Private:     private,
					Description: desc,
					Topics:      topicList,
				},
				TemplateRepo: github.Repository{
					Org:  templateOrg,
					Name: templateName,
				},
				IncludeAllBranches: includeAllBranches,
				Service:            service,
			}
			_, err := repos.CreateFromTemplate(ctx, opts)
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&templateName, "template-name", "", "Template repository name")
	cmd.Flags().StringVar(&templateOrg, "template-org", "", "Template repository organization name (defaults to --org if not set)")
	cmd.Flags().StringVar(&name, "name", "", "New repository name")
	cmd.Flags().StringVar(&desc, "desc", "", "Repository description")
	cmd.Flags().StringVar(&topics, "topics", "", "Comma-separated list of topics")
	cmd.Flags().BoolVar(&private, "private", false, "Create repository as private")
	cmd.Flags().BoolVar(&includeAllBranches, "include-all-branches", true, "Include all branches from the template repository")

	requiredFlags := []string{"token", "org", "name", "template-name"}
	for _, flag := range requiredFlags {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			log.Fatalf("Failed to mark flag %s as required: %v", flag, err)
		}
	}

	return cmd
}
