package repo

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/safety"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
)

// CreateNewRepoFromTemplateCmd creates a new command to create a GitHub repository from a template.
func CreateNewRepoFromTemplateCmd(svc repos.Service) *cobra.Command {
	var (
		token              string
		appID              int64
		installationID     int64
		appKeyPath         string
		org                string
		templateName       string
		templateOrg        string
		name               string
		desc               string
		topics             string
		private            bool
		includeAllBranches bool
		dryRun             bool
	)

	cmd := &cobra.Command{
		Use:     "create-from-template",
		Aliases: []string{"cft", "new-from-template", "create", "new"},
		Short:   "Create GitHub repositories from a template",
		Long:    "Create a new GitHub repository from a template repository, optionally specifying organization, name, description, topics, and privacy settings.",
		Example: `
			repo-builder repo create-from-template --token <token> --org <org> --template-name <template-name> --name <new-repo-name> --desc "Repository description" --topics "topic1,topic2" --private=true --include-all-branches=true
			repo-builder repo create-from-template --org <org> --template-name <template-name> --name <new-repo-name> --dry-run
			repo-builder repo create-from-template --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --template-name <template-name> --name <new-repo-name> --desc "Repository description" --topics "topic1,topic2" --private=true --include-all-branches=true`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			var topicList []string
			if topics != "" {
				topicList = strings.Split(strings.TrimSpace(topics), ",")
			}
			if templateOrg == "" {
				templateOrg = org
			}
			if dryRun {
				_, err := fmt.Fprintf(
					cmd.OutOrStdout(),
					"Dry run: would create repository %s/%s from template %s/%s (private=%t include-all-branches=%t topics=%v)\n",
					org,
					name,
					templateOrg,
					templateName,
					private,
					includeAllBranches,
					topicList,
				)
				return err
			}
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Repositories()
			}
			opts := repos.CreateFromTemplateOptions{
				Name:               strings.TrimSpace(name),
				Owner:              strings.TrimSpace(org),
				TemplateRepo:       strings.TrimSpace(templateName),
				TemplateOwner:      strings.TrimSpace(templateOrg),
				Description:        &desc,
				Private:            &private,
				Topics:             topicList,
				IncludeAllBranches: includeAllBranches,
				Service:            service,
			}
			_, err := repos.CreateFromTemplate(ctx, opts)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&templateName, "template-name", "", "Template repository name")
	cmd.Flags().StringVar(&templateOrg, "template-org", "", "Template repository organization name (defaults to --org if not set)")
	cmd.Flags().StringVar(&name, "name", "", "New repository name")
	cmd.Flags().StringVar(&desc, "desc", "", "Repository description")
	cmd.Flags().StringVar(&topics, "topics", "", "Comma-separated list of topics")
	cmd.Flags().BoolVar(&private, "private", false, "Create repository as private")
	cmd.Flags().BoolVar(&includeAllBranches, "include-all-branches", false, "Include all branches from the template repository")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "template-name", "name")

	return cmd
}
