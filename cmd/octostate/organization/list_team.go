package organization

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
)

// ListOrgTeamsCmd creates a command to list all teams in a GitHub organization.
func ListOrgTeamsCmd(svc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
	)

	cmd := &cobra.Command{
		Use:     "list-teams",
		Aliases: []string{"teams", "list-team"},
		Short:   "List teams in a GitHub organization",
		Long:    "Retrieve and display all teams belonging to a specified GitHub organization.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate organization list-teams --org <org-name>
			octostate organization list-teams --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name>`,
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

			opts := teams.ListTeamsOptions{
				Service: service,
				Org:     strings.TrimSpace(org),
			}

			allTeams, err := teams.ListTeams(ctx, opts)
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, allTeams)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}
