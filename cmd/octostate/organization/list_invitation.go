package organization

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
)

// ListOrgInvitationsCmd creates a command to list pending invitations for a
// GitHub organization.
func ListOrgInvitationsCmd(svc organizations.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
	)

	cmd := &cobra.Command{
		Use:     "list-invitations",
		Aliases: []string{"list-invitation", "invitations"},
		Short:   "List pending invitations in a GitHub organization",
		Long:    "Retrieve and display all pending invitations for a specified GitHub organization.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate organization list-invitations --org <org-name>
			octostate organization list-invitations --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			service := svc
			if service == nil {
				client, err := auth.NewClient(ctx, token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Organizations()
			}

			invitations, err := organizations.ListPendingInvitations(ctx, organizations.ListPendingInvitationsOptions{
				Service: service,
				OrgName: strings.TrimSpace(org),
			})
			if err != nil {
				return err
			}
			return cmdoutput.PrintJSON(cmd, invitations)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}
