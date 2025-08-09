package teams

import "github.com/orang-gaboets/repo-builder/pkg/github"

// CreateTeamOptions defines the options for creating a new team.
type CreateTeamOptions struct {
	Team    github.Team
	Service Service
}

// DeleteTeamOptions defines the options for deleting a team.
type DeleteTeamBySlugOptions struct {
	Org     string
	Slug    string
	Service Service
}

// GetTeamBySlugOptions defines the options for retrieving a team by its slug.
type GetTeamBySlugOptions struct {
	Org     string
	Slug    string
	Service Service
}
