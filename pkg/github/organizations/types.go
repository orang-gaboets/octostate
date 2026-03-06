package organizations

import (
	"context"

	gh "github.com/google/go-github/v55/github"
)

// Service defines the subset of GitHub organization APIs used by the organization package.
type Service interface {
	// Organization-related functions

	// CreateOrgInvitation creates an invitation for a user to join an organization.
	CreateOrgInvitation(ctx context.Context, org string, opts *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error)

	// Get gets an organization by its name.
	Get(ctx context.Context, org string) (*gh.Organization, *gh.Response, error)

	// ListMembers returns the members of an organization.
	ListMembers(ctx context.Context, org string, opts *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error)

	// ListPendingOrgInvitations returns the pending invitations for an organization.
	ListPendingOrgInvitations(ctx context.Context, org string, opts *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error)

	// ListOrgInvitationTeams returns the teams attached to an organization invitation.
	ListOrgInvitationTeams(ctx context.Context, org, invitationID string, opts *gh.ListOptions) ([]*gh.Team, *gh.Response, error)
}
