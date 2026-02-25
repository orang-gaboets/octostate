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
