package teams

import "github.com/orang-gaboets/repo-builder/pkg/github"

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
