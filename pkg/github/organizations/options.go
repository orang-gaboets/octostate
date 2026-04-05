package organizations

import (
	"fmt"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/github"
)

// GetOptions defines the options for retrieving organization details.
type GetOptions struct {
	Service Service
	OrgName string
}

// Validate checks if the GetOptions are valid.
func (opt *GetOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}
	return nil
}

// CreateInvitationOptions defines the options for creating an organization invitation.
type CreateInvitationOptions struct {
	Service Service
	OrgName string
	UserID  *int64
	Email   string
	Role    string
	TeamIDs []int64
}

// Validate checks if the CreateInvitationOptions are valid.
func (opt *CreateInvitationOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	opt.OrgName = strings.TrimSpace(opt.OrgName)
	opt.Email = strings.TrimSpace(opt.Email)
	opt.Role = strings.TrimSpace(opt.Role)
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}

	userIDProvided := opt.UserID != nil
	emailProvided := opt.Email != ""
	switch {
	case userIDProvided && emailProvided:
		return fmt.Errorf("exactly one invitation identity must be provided: %w", github.ErrConflictingCredentials)
	case !userIDProvided && !emailProvided:
		return fmt.Errorf("either user ID or email must be provided: %w", github.ErrMissingRequiredField)
	}

	if userIDProvided && *opt.UserID <= 0 {
		return fmt.Errorf("user ID must be greater than zero: %w", github.ErrInvalidFieldValue)
	}
	for _, teamID := range opt.TeamIDs {
		if teamID <= 0 {
			return fmt.Errorf("team ID must be greater than zero: %w", github.ErrInvalidFieldValue)
		}
	}

	return nil
}

// SetMembershipOptions defines the options for setting or updating an
// organization membership.
type SetMembershipOptions struct {
	Service  Service
	OrgName  string
	Username string
	Role     string
}

// Validate checks if the SetMembershipOptions are valid.
func (opt *SetMembershipOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	opt.OrgName = strings.TrimSpace(opt.OrgName)
	opt.Username = strings.TrimSpace(opt.Username)
	opt.Role = strings.TrimSpace(opt.Role)
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}
	if opt.Username == "" {
		return github.ErrMissingRequiredField
	}
	switch MemberRole(opt.Role) {
	case MemberRoleAdmin, MemberRoleMember:
		return nil
	default:
		return fmt.Errorf("invalid membership role %q: %w", opt.Role, github.ErrValidationFailed)
	}
}

// MemberRole specifies the membership role to filter by when listing organization members.
type MemberRole string

const (
	// MemberRoleAll includes all members regardless of role.
	MemberRoleAll MemberRole = "all"
	// MemberRoleAdmin includes only organization owners.
	MemberRoleAdmin MemberRole = "admin"
	// MemberRoleMember includes only non-owner members.
	MemberRoleMember MemberRole = "member"
)

// IsValid checks if the MemberRole value is valid.
func (mr MemberRole) IsValid() bool {
	switch mr {
	case MemberRoleAll, MemberRoleAdmin, MemberRoleMember:
		return true
	default:
		return false
	}
}

// ListMembersOptions defines the options for listing organization members.
type ListMembersOptions struct {
	Service Service
	OrgName string
	Role    MemberRole
}

// Validate checks if the ListMembersOptions are valid.
func (opt *ListMembersOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}
	if !opt.Role.IsValid() {
		return fmt.Errorf("invalid member role %q: %w", opt.Role, github.ErrValidationFailed)
	}
	return nil
}

// ListPendingInvitationsOptions defines the options for listing pending
// organization invitations.
type ListPendingInvitationsOptions struct {
	Service Service
	OrgName string
}

// Validate checks if the ListPendingInvitationsOptions are valid.
func (opt *ListPendingInvitationsOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}
	return nil
}

// ListInvitationTeamsOptions defines the options for listing the teams
// attached to an organization invitation.
type ListInvitationTeamsOptions struct {
	Service      Service
	OrgName      string
	InvitationID int64
}

// Validate checks if the ListInvitationTeamsOptions are valid.
func (opt *ListInvitationTeamsOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}
	if opt.InvitationID <= 0 {
		return fmt.Errorf("invitation ID must be greater than zero: %w", github.ErrMissingRequiredField)
	}
	return nil
}
