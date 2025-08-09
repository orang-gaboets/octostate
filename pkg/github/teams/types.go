package teams

import (
	"context"

	"github.com/google/go-github/v55/github"
)

// Service defines the subset of GitHub team APIs used by CreateTeam.
type Service interface {
	// Team-related function

	// CreateTeam creates a new team in the specified organization.
	CreateTeam(ctx context.Context, org string, team github.NewTeam) (*github.Team, *github.Response, error)

	// DeleteTeamBySlug deletes a team by its slug within an organization.
	DeleteTeamBySlug(ctx context.Context, org, slug string) (*github.Response, error)

	// GetTeamBySlug retrieves a team by its slug within an organization.
	GetTeamBySlug(ctx context.Context, org, slug string) (*github.Team, *github.Response, error)
}
