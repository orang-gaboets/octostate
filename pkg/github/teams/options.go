package teams

import (
	"fmt"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// CreateTeamOptions defines the options for creating a new team.
type CreateTeamOptions struct {
	Service        Service
	Name           string
	Org            string
	Description    *string
	Privacy        *github.TeamPrivacy
	ParentTeamSlug *string
}

// Validate checks if the CreateTeamOptions are valid.
func (opt *CreateTeamOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.Name == "" || opt.Org == "" {
		return github.ErrMissingRequiredField
	}
	return nil
}

// DeleteTeamBySlugOptions defines the options for deleting a team.
type DeleteTeamBySlugOptions struct {
	Service Service
	Org     string
	Slug    string
}

// Validate checks if the DeleteTeamBySlugOptions are valid.
func (opt *DeleteTeamBySlugOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.Org == "" || opt.Slug == "" {
		return github.ErrMissingRequiredField
	}
	return nil
}

// EditTeamBySlugOptions defines the options for editing a team by slug.
type EditTeamBySlugOptions struct {
	Service        Service
	Org            string
	Slug           string
	Name           *string
	Description    *string
	Privacy        *github.TeamPrivacy
	ParentTeamSlug *string
	RemoveParent   bool
}

// Validate checks if the EditTeamBySlugOptions are valid.
func (opt *EditTeamBySlugOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.Org == "" || opt.Slug == "" {
		return github.ErrMissingRequiredField
	}
	if opt.Name != nil && *opt.Name == "" {
		return fmt.Errorf("team name cannot be empty: %w", github.ErrMissingRequiredField)
	}
	if opt.ParentTeamSlug != nil && *opt.ParentTeamSlug == "" {
		return fmt.Errorf("parent team slug cannot be empty: %w", github.ErrMissingRequiredField)
	}
	if opt.ParentTeamSlug != nil && opt.RemoveParent {
		return fmt.Errorf("cannot set --parent and --clear-parent together: %w", github.ErrValidationFailed)
	}
	return nil
}

// GetTeamBySlugOptions defines the options for retrieving a team by its slug.
type GetTeamBySlugOptions struct {
	Service Service
	Org     string
	Slug    string
}

// Validate checks if the GetTeamBySlugOptions are valid.
func (opt *GetTeamBySlugOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.Org == "" || opt.Slug == "" {
		return github.ErrMissingRequiredField
	}
	return nil
}

// TeamMemberAddRole specifies the role assigned when adding a team member.
type TeamMemberAddRole string

const (
	// TeamMemberAddRoleMember assigns regular team membership.
	TeamMemberAddRoleMember TeamMemberAddRole = "member"
	// TeamMemberAddRoleMaintainer assigns team maintainer privileges.
	TeamMemberAddRoleMaintainer TeamMemberAddRole = "maintainer"
)

// IsValid checks if the TeamMemberAddRole value is valid.
func (r TeamMemberAddRole) IsValid() bool {
	switch r {
	case TeamMemberAddRoleMember, TeamMemberAddRoleMaintainer:
		return true
	default:
		return false
	}
}

// AddTeamMemberBySlugOptions defines the options for adding a user to a team by slug.
type AddTeamMemberBySlugOptions struct {
	Service  Service
	Org      string
	Slug     string
	Username string
	Role     TeamMemberAddRole
}

// Validate checks if the AddTeamMemberBySlugOptions are valid.
func (opt *AddTeamMemberBySlugOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.Org == "" || opt.Slug == "" || opt.Username == "" {
		return github.ErrMissingRequiredField
	}
	if !opt.Role.IsValid() {
		return fmt.Errorf("invalid team member add role %q: %w", opt.Role, github.ErrValidationFailed)
	}
	return nil
}

// TeamMemberRole specifies the membership role filter when listing team members.
type TeamMemberRole string

const (
	// TeamMemberRoleAll includes all team members.
	TeamMemberRoleAll TeamMemberRole = "all"
	// TeamMemberRoleMember includes only regular team members.
	TeamMemberRoleMember TeamMemberRole = "member"
	// TeamMemberRoleMaintainer includes only team maintainers.
	TeamMemberRoleMaintainer TeamMemberRole = "maintainer"
)

// IsValid checks if the TeamMemberRole value is valid.
func (tmr TeamMemberRole) IsValid() bool {
	switch tmr {
	case TeamMemberRoleAll, TeamMemberRoleMember, TeamMemberRoleMaintainer:
		return true
	default:
		return false
	}
}

// ListTeamMembersBySlugOptions defines the options for listing team members by team slug.
type ListTeamMembersBySlugOptions struct {
	Service Service
	Org     string
	Slug    string
	Role    TeamMemberRole
}

// Validate checks if the ListTeamMembersBySlugOptions are valid.
func (opt *ListTeamMembersBySlugOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.Org == "" || opt.Slug == "" {
		return github.ErrMissingRequiredField
	}
	if !opt.Role.IsValid() {
		return fmt.Errorf("invalid team member role %q: %w", opt.Role, github.ErrValidationFailed)
	}
	return nil
}

// ListTeamsOptions defines the options for listing teams in an organization.
type ListTeamsOptions struct {
	Service Service
	Org     string
}

// Validate checks if the ListTeamsOptions are valid.
func (opt *ListTeamsOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.Org == "" {
		return github.ErrMissingRequiredField
	}
	return nil
}
