package team

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// CreateTeamCmd creates a new command to create a GitHub team.
func CreateTeamCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		name           string
		desc           string
		secret         bool
		parent         string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"c", "new"},
		Short:   "Create a new GitHub team",
		Long:    "Create a new team in a GitHub organization.",
		Example: `
			repo-builder team create --token <token> --org <org> --name <team-name> --desc "Team description" --secret=false --parent <parent-team-slug>
			repo-builder team create --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <team-name> --desc "Team description" --secret=false --parent <parent-team-slug>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Teams()
			}

			privacy := github.PrivacyFromBool(secret)

			opts := teams.CreateTeamOptions{
				Org:         org,
				Name:        name,
				Description: &desc,
				Privacy:     &privacy,
				Service:     service,
			}
			if cmd.Flags().Changed("parent") {
				opts.ParentTeamSlug = &parent
			}

			_, err := teams.CreateTeam(ctx, opts)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "Team name")
	cmd.Flags().StringVar(&desc, "desc", "", "Team description")
	cmd.Flags().BoolVar(&secret, "secret", false, "Create a secret team (default: false)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent team slug (optional)")

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}
