package organization

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/configproposal"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/users"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
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
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "invite",
		Aliases: []string{"inv", "add-user", "invite-user"},
		Short:   "Invite a user to an organization",
		Long:    "Invite a user to a GitHub organization by their user ID or username.",
		Example: `
			octostate organization invite --token <token> --org <org-name> --id <user-id>
			octostate organization invite --org <org-name> --username <username> --dry-run
			octostate organization invite --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --username <username>`,
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

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
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

			if cmd.Flags().Changed("to-config") {
				return inviteToConfig(cmd, toConfig, trimmedOrg, trimmedUsername, userID, usernameProvided)
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
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}

// inviteProposalRole matches the role GitHub applies when the live invite path
// sends no explicit role.
const inviteProposalRole = "direct_member"

func inviteToConfig(cmd *cobra.Command, path, org, username string, userID int64, usernameProvided bool) error {
	resourceID := fmt.Sprintf("user_id:%d", userID)
	if usernameProvided {
		resourceID = "username:" + username
	}

	changed, err := configproposal.ApplyToConfigFile(path, org, func(cfg *gitopsconfig.OrganizationConfig) error {
		invite := gitopsconfig.InviteSpec{Role: inviteProposalRole}
		if usernameProvided {
			if _, exists := configproposal.FindInviteIndexByUsername(cfg, username); exists {
				return nil
			}
			invite.Username = gitopsconfig.OptionalString{Present: true, Value: username}
		} else {
			if _, exists := configproposal.FindInviteIndexByUserID(cfg, userID); exists {
				return nil
			}
			invite.UserID = gitopsconfig.OptionalInt64{Present: true, Value: userID}
		}
		cfg.Invites = append(cfg.Invites, invite)
		return nil
	})
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Proposed organization invite %s in config", resourceID)
	if !changed {
		message = fmt.Sprintf("No changes needed for organization invite %s", resourceID)
	}

	data := map[string]any{
		"organization": org,
		"role":         inviteProposalRole,
		"config_path":  path,
		"changed":      changed,
	}
	if usernameProvided {
		data["username"] = username
	} else {
		data["user_id"] = userID
	}
	return cmdoutput.PrintSuccess(cmd, message, data)
}
