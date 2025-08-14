package organization

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
	"github.com/orang-gaboets/repo-builder/pkg/github/users"
)

// InviteCmd creates a command to invite a user to an organization.
func InviteCmd(orgSvc organizations.Service, userSvc users.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		userID         int64
		username       string
	)

	cmd := &cobra.Command{
		Use:     "invite",
		Aliases: []string{"inv", "add-user", "invite-user"},
		Short:   "Invite a user to an organization",
		Long:    "Invite a user to a GitHub organization by their user ID or username.",
		Example: `
			repo-builder organization invite --token <token> --org <org-name> --id <user-id>
			repo-builder organization invite --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --username <username>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			userIDProvided := cmd.Flags().Changed("id")
			usernameProvided := cmd.Flags().Changed("username")

			var client auth.Client
			if orgSvc == nil || (!userIDProvided && usernameProvided && userSvc == nil) {
				var err error
				client, err = auth.NewClient(cmd.Context(), token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
			}

			switch {
			case !userIDProvided && !usernameProvided:
				return fmt.Errorf("%w: either --id or --username must be provided to invite a user", github.ErrMissingRequiredField)
			case userIDProvided && usernameProvided:
				return fmt.Errorf("%w: cannot provide both --id and --username", github.ErrConflictingCredentials)
			case !userIDProvided && usernameProvided:
				if userSvc == nil {
					userSvc = client.Users()
				}
				opts := users.GetUserByUsernameOptions{
					Service:  userSvc,
					Username: strings.TrimSpace(username),
				}
				user, err := users.GetUserByUsername(cmd.Context(), opts)
				if err != nil {
					return err
				}
				if user == nil || user.ID == nil {
					return fmt.Errorf("%w: user with username %s not found", github.ErrNotFound, username)
				}
				userID = *user.ID
			default:
				// userID is already provided
			}

			if orgSvc == nil {
				orgSvc = client.Organizations()
			}

			opts := organizations.InviteUserOptions{
				Service: orgSvc,
				OrgName: strings.TrimSpace(org),
				UserID:  userID,
			}
			err := organizations.InviteUser(cmd.Context(), opts)
			return err
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVarP(&org, "org", "o", "", "Name of the organization to invite the user to")
	cmd.Flags().Int64VarP(&userID, "id", "i", 0, "User ID to invite to the organization")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username of the user to invite to the organization")

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}
