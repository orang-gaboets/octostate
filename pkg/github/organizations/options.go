package organizations

import (
	"fmt"

	"github.com/orang-gaboets/repo-builder/pkg/github"
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

// InviteUserOptions defines the options for inviting a user to an organization.
type InviteUserOptions struct {
	Service Service
	OrgName string
	UserID  int64
}

// Validate checks if the InviteUserOptions are valid.
func (opt *InviteUserOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.OrgName == "" {
		return github.ErrMissingRequiredField
	}
	if opt.UserID <= 0 {
		return fmt.Errorf("user ID must be greater than zero: %w", github.ErrMissingRequiredField)
	}
	return nil
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
