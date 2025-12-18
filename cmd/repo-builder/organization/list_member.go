package organization

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
)

// ListOrgMembersCmd creates a command to list all members of a GitHub organization.
func ListOrgMembersCmd(svc organizations.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		role           string
	)

	cmd := &cobra.Command{
		Use:     "list-members",
		Aliases: []string{"members", "list-member"},
		Short:   "List members in a GitHub organization",
		Long:    "Retrieve and display all members belonging to a specified GitHub organization.",
		Example: `
			repo-builder organization list-members --token <token> --org <org-name>
			repo-builder organization list-members --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --role all`,
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

			if !organizations.MemberRole(strings.TrimSpace(role)).IsValid() {
				return github.ErrInvalidFieldValue
			}

			opts := organizations.ListMembersOptions{
				Service: service,
				OrgName: strings.TrimSpace(org),
				Role:    organizations.MemberRole(strings.TrimSpace(role)),
			}

			_, err := organizations.ListMembers(ctx, opts)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVar(&org, "org", "", "GitHub organization name")
	cmd.Flags().StringVar(&role, "role", string(organizations.MemberRoleAll), "Member role filter: all, admin, member")

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}
