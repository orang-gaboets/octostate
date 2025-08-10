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

			opts := teams.CreateTeamOptions{
				Team: github.Team{
					Org:         org,
					Name:        name,
					Description: desc,
					Privacy:     github.PrivacyFromBool(secret),
				},
				Service: service,
			}
			if cmd.Flags().Changed("parent") {
				opts.Team.ParentTeam = &github.Team{
					Org:  org,
					Slug: parent,
				}
			}

			_, err := teams.CreateTeam(ctx, opts)
			return err
		},
	}

	cmd.Flags().StringVar(&token, "token", "", "GitHub access token")
	cmd.Flags().Int64Var(&appID, "app-id", 0, "GitHub App ID for authentication")
	cmd.Flags().Int64Var(&installationID, "installation-id", 0, "GitHub App installation ID for authentication")
	cmd.Flags().StringVar(&appKeyPath, "app-key-path", "", "Path to the GitHub App private key file")
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "Team name")
	cmd.Flags().StringVar(&desc, "desc", "", "Team description")
	cmd.Flags().BoolVar(&secret, "secret", false, "Create a secret team (default: false)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent team slug (optional)")

	github.MarkRequiredFlags(cmd, "org", "name")

	return cmd
}
