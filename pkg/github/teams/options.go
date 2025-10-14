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

// DeleteTeamBySlugOptions defines the options for deleting a team.
type DeleteTeamBySlugOptions struct {
	Service Service
	Org     string
	Slug    string
}

// GetTeamBySlugOptions defines the options for retrieving a team by its slug.
type GetTeamBySlugOptions struct {
	Service Service
	Org     string
	Slug    string
}
