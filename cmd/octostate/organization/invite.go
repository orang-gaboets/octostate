package organization

import (
	"fmt"
	"net/mail"
	"strings"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/configproposal"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	"github.com/orang-gaboets/octostate/pkg/github/users"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// InviteCmd creates a command to invite a user to an organization.
func InviteCmd(orgSvc organizations.Service, userSvc users.Service, teamSvc teams.Service) *cobra.Command {
	var (
		token          string
		appID          int64
		installationID int64
		appKeyPath     string
		org            string
		userID         int64
		username       string
		email          string
		role           string
		teamSlugs      []string
		dryRun         bool
		toConfig       string
	)

	cmd := &cobra.Command{
		Use:     "invite",
		Aliases: []string{"inv", "add-user", "invite-user"},
		Short:   "Invite a user to an organization",
		Long:    "Invite a user to a GitHub organization by user ID, username, or email.",
		Example: `
			OCTOSTATE_GITHUB_TOKEN="<token>" octostate organization invite --org <org-name> --id <user-id>
			octostate organization invite --org <org-name> --username <username> --dry-run
			octostate organization invite --org <org-name> --email <email> --role direct_member --team-slug <slug>
			octostate organization invite --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org-name> --username <username>`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			trimmedOrg := strings.TrimSpace(org)
			trimmedUsername := strings.TrimSpace(username)
			trimmedEmail := strings.TrimSpace(email)
			userIDProvided := cmd.Flags().Changed("id")
			usernameProvided := cmd.Flags().Changed("username")
			emailProvided := cmd.Flags().Changed("email")

			if err := validateInviteIdentity(
				userIDProvided, userID,
				usernameProvided, trimmedUsername,
				emailProvided, trimmedEmail,
			); err != nil {
				return err
			}

			requestedRole := effectiveInviteRole(role)
			if !isSupportedInviteRole(requestedRole) {
				return fmt.Errorf("role %q must be one of admin, direct_member, billing_manager: %w", requestedRole, github.ErrInvalidFieldValue)
			}
			normalizedTeamSlugs, err := normalizeTeamSlugs(teamSlugs)
			if err != nil {
				return err
			}

			if dryRun && cmd.Flags().Changed("to-config") {
				return fmt.Errorf("--to-config cannot be combined with --dry-run")
			}
			if dryRun {
				return printInviteDryRun(cmd, inviteProposal{
					org:          trimmedOrg,
					username:     trimmedUsername,
					email:        trimmedEmail,
					userID:       userID,
					usesUsername: usernameProvided,
					usesEmail:    emailProvided,
					role:         requestedRole,
					teamSlugs:    normalizedTeamSlugs,
				})
			}

			if cmd.Flags().Changed("to-config") {
				return inviteToConfig(cmd, inviteProposal{
					path:         toConfig,
					org:          trimmedOrg,
					username:     trimmedUsername,
					email:        trimmedEmail,
					userID:       userID,
					usesUsername: usernameProvided,
					usesEmail:    emailProvided,
					role:         requestedRole,
					teamSlugs:    normalizedTeamSlugs,
				})
			}

			var client auth.Client
			if orgSvc == nil || (!userIDProvided && usernameProvided && userSvc == nil) || (len(normalizedTeamSlugs) > 0 && teamSvc == nil) {
				var err error
				client, err = auth.NewClient(cmd.Context(), token, appID, installationID, appKeyPath)
				if err != nil {
					return err
				}
			}

			switch {
			case emailProvided:
				// The email identity is sent directly; no user lookup applies.
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
				Role:    requestedRole,
			}
			if emailProvided {
				opts.Email = trimmedEmail
			} else {
				opts.UserID = &userID
			}
			if len(normalizedTeamSlugs) > 0 {
				resolver := teamSvc
				if resolver == nil {
					resolver = client.Teams()
				}
				teamIDs, err := resolveTeamIDs(cmd, resolver, trimmedOrg, normalizedTeamSlugs)
				if err != nil {
					return err
				}
				opts.TeamIDs = teamIDs
			}
			if err := organizations.CreateInvitation(cmd.Context(), opts); err != nil {
				return err
			}

			data := map[string]any{
				"organization": trimmedOrg,
				"role":         requestedRole,
				"team_slugs":   normalizedTeamSlugs,
			}
			switch {
			case emailProvided:
				data["email"] = trimmedEmail
				return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Invited %q to organization %s", trimmedEmail, trimmedOrg), data)
			case usernameProvided:
				data["username"] = trimmedUsername
				data["user_id"] = userID
				return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Invited user %q to organization %s", trimmedUsername, trimmedOrg), data)
			default:
				data["user_id"] = userID
				return cmdoutput.PrintSuccess(cmd, fmt.Sprintf("Invited user ID %d to organization %s", userID, trimmedOrg), data)
			}
		},
	}

	auth.AddFlags(cmd, &token, &appID, &installationID, &appKeyPath)

	cmd.Flags().StringVarP(&org, "org", "o", "", "Name of the organization to invite the user to")
	cmd.Flags().Int64VarP(&userID, "id", "i", 0, "User ID to invite to the organization")
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username of the user to invite to the organization")
	cmd.Flags().StringVarP(&email, "email", "e", "", "Email address to invite to the organization")
	cmd.Flags().StringVar(&role, "role", inviteProposalRole, "Invitation role: admin, direct_member, or billing_manager")
	cmd.Flags().StringArrayVar(&teamSlugs, "team-slug", nil, "Team slug to assign the invitation to; repeat for multiple teams")
	cmd.Flags().StringVar(&toConfig, "to-config", "", "Write the proposal to an organization.yaml file instead of GitHub")
	safety.AddDryRunFlag(cmd, &dryRun)

	github.MarkRequiredFlags(cmd, "org")

	return cmd
}

// inviteProposalRole matches the role GitHub applies when the live invite path
// sends no explicit role.
const inviteProposalRole = "direct_member"

// effectiveInviteRole returns the invitation role to send. An omitted or
// whitespace-only role resolves to direct_member.
func effectiveInviteRole(role string) string {
	if trimmed := strings.TrimSpace(role); trimmed != "" {
		return trimmed
	}
	return inviteProposalRole
}

// isSupportedInviteRole reports whether the role is one the desired-state
// invite schema accepts. Durable membership roles are deliberately not
// accepted here; those belong to organization membership set.
// validateInviteIdentity requires exactly one usable invitation identity.
//
// It validates the value rather than only which flag was supplied, so an
// unusable request fails before dry-run, proposal, and live handling instead of
// being previewed as if it were valid.
func validateInviteIdentity(
	userIDProvided bool, userID int64,
	usernameProvided bool, username string,
	emailProvided bool, email string,
) error {
	identities := 0
	for _, provided := range []bool{userIDProvided, usernameProvided, emailProvided} {
		if provided {
			identities++
		}
	}
	switch {
	case identities == 0:
		return fmt.Errorf("%w: one of --id, --username, or --email must be provided to invite a user", github.ErrMissingRequiredField)
	case identities > 1:
		return fmt.Errorf("%w: provide exactly one of --id, --username, or --email", github.ErrConflictingCredentials)
	}

	switch {
	case userIDProvided && userID <= 0:
		return fmt.Errorf("user ID must be greater than zero: %w", github.ErrMissingRequiredField)
	case usernameProvided && username == "":
		return fmt.Errorf("%w: --username must not be empty", github.ErrMissingRequiredField)
	case emailProvided && email == "":
		return fmt.Errorf("%w: --email must not be empty", github.ErrMissingRequiredField)
	case emailProvided && !isValidInviteEmail(email):
		return fmt.Errorf("email %q is not a valid email address: %w", email, github.ErrInvalidFieldValue)
	}
	return nil
}

// isValidInviteEmail mirrors the desired-state invite email rule so the CLI and
// config validation cannot disagree about what a usable address is. The config
// rule is unexported, so the semantics are matched here rather than widening
// the config package API for a CLI concern.
func isValidInviteEmail(email string) bool {
	address, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	return address.Address == email
}

// printInviteDryRun previews the requested invitation without any GitHub call,
// including for a username identity, so dry run stays a safe preview.
func printInviteDryRun(cmd *cobra.Command, proposal inviteProposal) error {
	data := map[string]any{
		"organization": proposal.org,
		"role":         proposal.role,
		"team_slugs":   proposal.teamSlugs,
	}
	var message string
	switch {
	case proposal.usesUsername:
		data["username"] = proposal.username
		data["username_lookup"] = "skipped"
		message = fmt.Sprintf("Dry run: would invite user %q to organization %s (username lookup skipped)", proposal.username, proposal.org)
	case proposal.usesEmail:
		data["email"] = proposal.email
		message = fmt.Sprintf("Dry run: would invite %q to organization %s", proposal.email, proposal.org)
	default:
		data["user_id"] = proposal.userID
		message = fmt.Sprintf("Dry run: would invite user ID %d to organization %s", proposal.userID, proposal.org)
	}
	return cmdoutput.PrintDryRun(cmd, message, data)
}

func isSupportedInviteRole(role string) bool {
	switch role {
	case "admin", "direct_member", "billing_manager":
		return true
	default:
		return false
	}
}

// normalizeTeamSlugs trims and de-duplicates case-insensitively while
// preserving the caller's order, so the written config matches the
// deterministic shape the config contract expects.
//
// An explicitly supplied blank slug is an error rather than an omitted value:
// silently dropping it would send an invitation without a team the caller asked
// for, and the desired-state validator rejects an empty team_slugs entry too.
func normalizeTeamSlugs(slugs []string) ([]string, error) {
	normalized := make([]string, 0, len(slugs))
	seen := make(map[string]struct{}, len(slugs))
	for _, slug := range slugs {
		trimmed := strings.TrimSpace(slug)
		if trimmed == "" {
			return nil, fmt.Errorf("team slug must not be empty: %w", github.ErrMissingRequiredField)
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}

// resolveTeamIDs turns requested team slugs into the IDs the GitHub invitation
// API needs, failing clearly when a slug does not resolve rather than silently
// sending an invitation without the requested team.
func resolveTeamIDs(cmd *cobra.Command, teamSvc teams.Service, org string, slugs []string) ([]int64, error) {
	ids := make([]int64, 0, len(slugs))
	for _, slug := range slugs {
		team, err := teams.GetTeamBySlug(cmd.Context(), teams.GetTeamBySlugOptions{
			Service: teamSvc,
			Org:     org,
			Slug:    slug,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve team %s/%s: %w", org, slug, err)
		}
		if team.ID == 0 {
			return nil, fmt.Errorf("%w: team %s/%s did not return a usable team ID", github.ErrNotFound, org, slug)
		}
		ids = append(ids, team.ID)
	}
	return ids, nil
}

// inviteProposal carries the requested invitation shape into proposal mode.
type inviteProposal struct {
	path         string
	org          string
	username     string
	email        string
	userID       int64
	usesUsername bool
	usesEmail    bool
	role         string
	teamSlugs    []string
}

func (p inviteProposal) resourceID() string {
	switch {
	case p.usesEmail:
		return "email:" + p.email
	case p.usesUsername:
		return "username:" + p.username
	default:
		return fmt.Sprintf("user_id:%d", p.userID)
	}
}

func (p inviteProposal) find(cfg *gitopsconfig.OrganizationConfig) (int, bool) {
	switch {
	case p.usesEmail:
		return configproposal.FindInviteIndexByEmail(cfg, p.email)
	case p.usesUsername:
		return configproposal.FindInviteIndexByUsername(cfg, p.username)
	default:
		return configproposal.FindInviteIndexByUserID(cfg, p.userID)
	}
}

func (p inviteProposal) spec() gitopsconfig.InviteSpec {
	invite := gitopsconfig.InviteSpec{Role: p.role, TeamSlugs: p.teamSlugs}
	switch {
	case p.usesEmail:
		invite.Email = gitopsconfig.OptionalString{Present: true, Value: p.email}
	case p.usesUsername:
		invite.Username = gitopsconfig.OptionalString{Present: true, Value: p.username}
	default:
		invite.UserID = gitopsconfig.OptionalInt64{Present: true, Value: p.userID}
	}
	return invite
}

func inviteToConfig(cmd *cobra.Command, proposal inviteProposal) error {
	// Reported back to the caller so a no-op describes the invite the config
	// actually retains rather than the one this command would have added.
	role := proposal.role
	teamSlugs := append([]string{}, proposal.teamSlugs...)

	changed, err := configproposal.ApplyToConfigFile(proposal.path, proposal.org, func(cfg *gitopsconfig.OrganizationConfig) error {
		existing, found := proposal.find(cfg)
		if found {
			retained := cfg.Invites[existing]
			role = effectiveInviteRole(retained.Role)
			teamSlugs = append([]string{}, retained.TeamSlugs...)
			return nil
		}

		cfg.Invites = append(cfg.Invites, proposal.spec())
		return nil
	})
	if err != nil {
		return err
	}

	resourceID := proposal.resourceID()
	message := fmt.Sprintf("Proposed organization invite %s in config", resourceID)
	if !changed {
		message = fmt.Sprintf("No changes needed for organization invite %s; config already declares it", resourceID)
	}

	data := map[string]any{
		"organization": proposal.org,
		"role":         role,
		"team_slugs":   teamSlugs,
		"config_path":  proposal.path,
		"changed":      changed,
	}
	switch {
	case proposal.usesEmail:
		data["email"] = proposal.email
	case proposal.usesUsername:
		data["username"] = proposal.username
	default:
		data["user_id"] = proposal.userID
	}
	return cmdoutput.PrintSuccess(cmd, message, data)
}
