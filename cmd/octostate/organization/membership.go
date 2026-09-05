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
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// MembershipCmd groups durable organization membership operations.
//
// Membership is deliberately separate from `organization invite`: members: is
// durable desired state, while an invitation is the transitional resource that
// precedes it.
func MembershipCmd(orgSvc organizations.Service) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "membership",
		Aliases: []string{"memberships"},
		Short:   "Organization membership operation",
		Long:    "Manage durable organization membership, separate from transitional invitations.",
	}
	cmd.AddCommand(MembershipSetCmd(orgSvc))
	return cmd
}

// MembershipSetCmd creates a command to set an organization membership role.
func MembershipSetCmd(orgSvc organizations.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		username       string
		role           string
		dryRun         bool
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "set",
		Aliases: []string{"add", "update"},
		Short:   "Set an organization membership role",
		Long:    "Set or update a user's durable organization membership role.",
		Example: `
			octostate organization membership set --org <org-name> --username <username> --role member
			octostate organization membership set --org <org-name> --username <username> --role admin --dry-run
			octostate organization membership set --org <org-name> --username <username> --role member --to-config organization.yaml`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trimmedOrg := strings.TrimSpace(org)
			trimmedUsername := strings.TrimSpace(username)
			trimmedRole := strings.TrimSpace(role)

			if trimmedOrg == "" {
				return fmt.Errorf("organization cannot be empty: %w", github.ErrMissingRequiredField)
			}
			if trimmedUsername == "" {
				return fmt.Errorf("username cannot be empty: %w", github.ErrMissingRequiredField)
			}
			// Durable membership roles only. Invitation-only roles such as
			// direct_member and billing_manager belong to organization invite.
			switch organizations.MemberRole(trimmedRole) {
			case organizations.MemberRoleAdmin, organizations.MemberRoleMember:
			default:
				return fmt.Errorf("role %q must be one of admin, member: %w", trimmedRole, github.ErrInvalidFieldValue)
			}

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun {
				return cmdoutput.PrintDryRun(
					cmd,
					fmt.Sprintf("Dry run: would set organization membership %s/%s to role %s", trimmedOrg, trimmedUsername, trimmedRole),
					map[string]any{
						"organization": trimmedOrg,
						"username":     trimmedUsername,
						"role":         trimmedRole,
					},
				)
			}

			if cmd.Flags().Changed("to-config") {
				return membershipToConfig(cmd, toConfig, trimmedOrg, trimmedUsername, trimmedRole)
			}

			service := orgSvc
			if service == nil {
				client, err := auth.NewClient(cmd.Context(), token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
				service = client.Organizations()
			}

			if err := organizations.SetMembership(cmd.Context(), organizations.SetMembershipOptions{
				Service:  service,
				OrgName:  trimmedOrg,
				Username: trimmedUsername,
				Role:     trimmedRole,
			}); err != nil {
				return err
			}

			// GitHub accepting the request does not mean the user is already an
			// active member: for a non-member this creates a pending membership
			// the user must accept. The message states what was requested.
			return cmdoutput.PrintSuccess(
				cmd,
				fmt.Sprintf("Requested organization membership %s/%s with role %s", trimmedOrg, trimmedUsername, trimmedRole),
				map[string]any{
					"organization": trimmedOrg,
					"username":     trimmedUsername,
					"role":         trimmedRole,
				},
			)
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVarP(&org, "org", "o", "", "Name of the organization")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username whose membership is being set")
	cmd.Flags().StringVar(&role, "role", string(organizations.MemberRoleMember), "Organization membership role: admin or member")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org", "username")

	return cmd
}

func membershipToConfig(cmd *cobra.Command, path, org, username, role string) error {
	changed, err := configproposal.ApplyToConfigFile(path, org, func(cfg *gitopsconfig.OrganizationConfig) error {
		if existing, found := configproposal.FindOrganizationMemberIndex(cfg, username); found {
			cfg.Members[existing].Role = role
			return nil
		}
		cfg.Members = append(cfg.Members, gitopsconfig.OrganizationMemberSpec{
			Username: username,
			Role:     role,
		})
		return nil
	})
	if err != nil {
		return err
	}

	message := fmt.Sprintf("Proposed organization membership %s/%s in config", org, username)
	if !changed {
		message = fmt.Sprintf("No changes needed for organization membership %s/%s; config already declares it", org, username)
	}
	return cmdoutput.PrintSuccess(cmd, message, map[string]any{
		"organization": org,
		"username":     username,
		"role":         role,
		"config_path":  path,
		"changed":      changed,
	})
}
