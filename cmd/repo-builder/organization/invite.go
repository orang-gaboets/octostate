package organization

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/safety"
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
		dryRun         bool
	)

	cmd := &cobra.Command{
		Use:     "invite",
		Aliases: []string{"inv", "add-user", "invite-user"},
		Short:   "Invite a user to an organization",
		Long:    "Invite a user to a GitHub organization by their user ID or username.",
		Example: `
			repo-builder organization invite --token <token> --org <org-name> --id <user-id>
			repo-builder organization invite --org <org-name> --username <username> --dry-run
			repo-builder organization invite --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --username <username>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trimmedOrg := strings.TrimSpace(org)
			trimmedUsername := strings.TrimSpace(username)
			userIDProvided := cmd.Flags().Changed("id")
			usernameProvided := cmd.Flags().Changed("username")

			switch {
			case !userIDProvided && !usernameProvided:
				return fmt.Errorf("%w: either --id or --username must be provided to invite a user", github.ErrMissingRequiredField)
			case userIDProvided && usernameProvided:
				return fmt.Errorf("%w: cannot provide both --id and --username", github.ErrConflictingCredentials)
			}

			if dryRun {
				if usernameProvided {
					return cmdoutput.PrintDryRun(
						cmd,
						fmt.Sprintf("Dry run: would invite user %q to organization %s (username lookup skipped)", trimmedUsername, trimmedOrg),
						map[string]any{
							"organization":    trimmedOrg,
							"username":        trimmedUsername,
							"username_lookup": "skipped",
						},
					)
				}
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("Dry run: would invite user ID %d to organization %s", userID, trimmedOrg),
					map[string]any{
						"organization": trimmedOrg,
						"user_id":      userID,
					},
				)
			}

			if userIDProvided && userID <= 0 {
				return fmt.Errorf("user ID must be greater than zero: %w", github.ErrMissingRequiredField)
			}

			var client auth.Client
			if orgSvc == nil || (!userIDProvided && usernameProvided && userSvc == nil) {
				var err error
				client, err = auth.NewClient(cmd.Context(), token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
			}

			switch {
			case !userIDProvided && usernameProvided:
				if userSvc == nil {
					userSvc = client.Users()
				}
				opts := users.GetUserByUsernameOptions{
					Service:  userSvc,
					Username: trimmedUsername,
				}
				user, err := users.GetUserByUsername(cmd.Context(), opts)
				if err != nil {
					return err
				}
				if user == nil || user.ID == nil {
					return fmt.Errorf("%w: user with username %s not found", github.ErrNotFound, trimmedUsername)
				}
				userID = *user.ID
			default:
				// userID is already provided
			}

			if orgSvc == nil {
				orgSvc = client.Organizations()
			}

			opts := organizations.CreateInvitationOptions{
				Service: orgSvc,
				OrgName: trimmedOrg,
				UserID:  &userID,
			}
			if err := organizations.CreateInvitation(cmd.Context(), opts); err != nil {
				return err
			}

			if usernameProvided {
				return cmdoutput.PrintSuccess(
					cmd,
					fmt.Sprintf("Invited user %q to organization %s", trimmedUsername, trimmedOrg),
					map[string]any{
						"organization": trimmedOrg,
						"username":     trimmedUsername,
						"user_id":      userID,
					},
				)
			}

			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Invited user ID %d to organization %s", userID, trimmedOrg),
				map[string]any{
					"organization": trimmedOrg,
					"user_id":      userID,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVarP(&org, "org", "o", "", "Name of the organization to invite the user to")
	cmd.Flags().Int64VarP(&userID, "id", "i", 0, "User ID to invite to the organization")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username of the user to invite to the organization")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}
