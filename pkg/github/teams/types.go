package teams

import (
	"context"

	gh "github.com/google/go-github/v55/github"
)

// Service defines the subset of GitHub team APIs used by team commands.
type Service interface {
	// Team-related function

	// CreateTeam creates a new team in the specified organization.
	CreateTeam(ctx context.Context, org string, team gh.NewTeam) (*gh.Team, *gh.Response, error)

	// EditTeamBySlug edits a team by its slug within an organization.
	EditTeamBySlug(ctx context.Context, org, slug string, team gh.NewTeam, removeParent bool) (*gh.Team, *gh.Response, error)

	// DeleteTeamBySlug deletes a team by its slug within an organization.
	DeleteTeamBySlug(ctx context.Context, org, slug string) (*gh.Response, error)

	// GetTeamBySlug retrieves a team by its slug within an organization.
	GetTeamBySlug(ctx context.Context, org, slug string) (*gh.Team, *gh.Response, error)

	// ListTeams lists teams in an organization.
	ListTeams(ctx context.Context, org string, opts *gh.ListOptions) ([]*gh.Team, *gh.Response, error)
}
