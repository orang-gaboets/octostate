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

	// AddTeamMembershipBySlug adds or updates a user's membership in a team by slug.
	AddTeamMembershipBySlug(ctx context.Context, org, slug, user string, opts *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error)

	// RemoveTeamMembershipBySlug removes a user's membership from a team by slug.
	RemoveTeamMembershipBySlug(ctx context.Context, org, slug, user string) (*gh.Response, error)

	// ListTeamReposBySlug lists repositories accessible by a team, including permissions.
	ListTeamReposBySlug(ctx context.Context, org, slug string, opts *gh.ListOptions) ([]*gh.Repository, *gh.Response, error)

	// AddTeamRepoBySlug adds or updates a team's permission on a repository.
	AddTeamRepoBySlug(ctx context.Context, org, slug, owner, repo string, opts *gh.TeamAddTeamRepoOptions) (*gh.Response, error)

	// RemoveTeamRepoBySlug removes a team's access to a repository.
	RemoveTeamRepoBySlug(ctx context.Context, org, slug, owner, repo string) (*gh.Response, error)

	// ListTeamMembersBySlug lists members of a team within an organization.
	ListTeamMembersBySlug(ctx context.Context, org, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error)

	// ListTeams lists teams in an organization.
	ListTeams(ctx context.Context, org string, opts *gh.ListOptions) ([]*gh.Team, *gh.Response, error)
}
