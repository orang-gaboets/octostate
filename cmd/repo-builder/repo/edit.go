package repo

import (
	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitHubClient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
	"github.com/spf13/cobra"
)

// EditRepo creates a new command to edit an existing GitHub repository.
func EditRepo(svc repos.Service) *cobra.Command {
	var (
		token           string
		org             string
		name            string
		newDesc         string
		newHomepage     string
		newPrivate      bool
		newIsTemplate   bool
		newArchived     bool
		newAllowForking bool
	)

	cmd := &cobra.Command{
		Use:     "edit",
		Short:   "Edit an existing GitHub repository",
		Long:    "Edit an existing GitHub repository by updating its description, homepage, privacy settings, template status, archived status, and forking permissions.",
		Example: `repo-builder repo edit --token <token> --org <org> --name <repo-name> --desc "New description" --homepage "https://example.com" --private=true --is-template=false --archived=false --allow-forking=true`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client := gitHubClient.New(ctx, token)
				service = client.Repositories
			}
			var opts repos.EditOptions
			opts.Repository = github.Repository{
				Org:  org,
				Name: name,
			}
			opts.Service = service
			if cmd.Flags().Changed("desc") {
				opts.Description = &newDesc
			}
			if cmd.Flags().Changed("homepage") {
				opts.Homepage = &newHomepage
			}
			if cmd.Flags().Changed("private") {
				opts.Private = &newPrivate
			}
			if cmd.Flags().Changed("is-template") {
				opts.IsTemplate = &newIsTemplate
			}
			if cmd.Flags().Changed("archived") {
				opts.Archived = &newArchived
			}
			if cmd.Flags().Changed("allow-forking") {
				opts.AllowForking = &newAllowForking
			}

			_, err := repos.Edit(ctx, opts)
			if err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "Name of the repository to edit")
	cmd.Flags().StringVar(&newDesc, "desc", "", "New description for the repository")
	cmd.Flags().StringVar(&newHomepage, "homepage", "", "New homepage URL for the repository")
	cmd.Flags().BoolVar(&newPrivate, "private", false, "Set the repository to private")
	cmd.Flags().BoolVar(&newIsTemplate, "is-template", false, "Set the repository as a template")
	cmd.Flags().BoolVar(&newArchived, "archived", false, "Archive the repository")
	cmd.Flags().BoolVar(&newAllowForking, "allow-forking", false, "Allow private forking of the repository")

	requiredFlags := []string{"token", "org", "name"}

	for _, flag := range requiredFlags {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			cobra.CheckErr(cmd.MarkFlagRequired(flag))
		}
	}

	return cmd
}
