package cmd

import (
	"context"
	"log"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/builder"
)

// NewRootCmd returns the root cobra command. If svc is nil, a GitHub client will
// be created from the provided token at runtime.
func NewRootCmd(svc builder.RepoService) *cobra.Command {
	var (
		token        string
		org          string
		TemplateName string
		TemplateOrg  string
		name         string
		desc         string
		topics       string
		private      bool
	)

	cmd := &cobra.Command{
		Use:   "repo-builder",
		Short: "Create GitHub repositories from a template",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			service := svc
			if service == nil {
				client := builder.NewGitHubClient(ctx, token)
				service = client.Repositories
			}
			topicList := []string{}
			if topics != "" {
				topicList = strings.Split(topics, ",")
			}
			if TemplateOrg == "" {
				TemplateOrg = org
			}
			opts := builder.RepoCreationOptions{
				Org:          org,
				Name:         name,
				Private:      private,
				Description:  desc,
				Topics:       topicList,
				TemplateName: TemplateName,
				TemplateOrg:  TemplateOrg,
				Service:      service,
			}
			_, err := builder.CreateRepo(ctx, opts)
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&TemplateName, "templateName", "", "Template repository name")
	cmd.Flags().StringVar(&name, "name", "", "New repository name")
	cmd.Flags().StringVar(&desc, "desc", "", "Repository description")
	cmd.Flags().StringVar(&topics, "topics", "", "Comma-separated list of topics")
	cmd.Flags().BoolVar(&private, "private", false, "Create repository as private")

	cmd.MarkFlagRequired("token")
	cmd.MarkFlagRequired("org")
	cmd.MarkFlagRequired("template")
	cmd.MarkFlagRequired("name")

	return cmd
}

// Execute executes the command.
func Execute() {
	if err := NewRootCmd(nil).Execute(); err != nil {
		log.Fatal(err)
	}
}
