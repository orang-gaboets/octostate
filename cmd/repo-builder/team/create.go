package team

import (
	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	gitHubClient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// CreateTeamCmd creates a new command to create a GitHub team.
func CreateTeamCmd(svc teams.Service) *cobra.Command {
	var (
		token  string
		org    string
		name   string
		desc   string
		secret bool
		parent string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"c", "new"},
		Short:   "Create a new GitHub team",
		Long:    "Create a new team in a GitHub organization.",
		Example: `repo-builder team create --token <token> --org <org> --name <team-name> --desc "Team description" --secret=false --parent <parent-team-slug>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if svc == nil {
				client := gitHubClient.New(ctx, token)
				service = client.Teams
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
	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&name, "name", "", "Team name")
	cmd.Flags().StringVar(&desc, "desc", "", "Team description")
	cmd.Flags().BoolVar(&secret, "secret", false, "Create a secret team (default: false)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent team slug (optional)")

	requiredFlags := []string{"token", "org", "name"}
	for _, flag := range requiredFlags {
		if err := cmd.MarkFlagRequired(flag); err != nil {
			cobra.CheckErr(err)
		}
	}

	return cmd
}
