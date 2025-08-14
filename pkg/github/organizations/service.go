package organizations

import (
	"context"
	"fmt"
	"log"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// Get retrieves an organization by its name.
func Get(ctx context.Context, opts GetOptions) (*github.Organization, error) {
	if opts.Service == nil {
		return nil, github.ErrNilService
	}

	if opts.OrgName == "" {
		return nil, github.ErrMissingRequiredField
	}

	log.Printf("Retrieving organization: %s", opts.OrgName)
	ghOrg, _, err := opts.Service.Get(ctx, opts.OrgName)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to get organization %s", opts.OrgName))
	}
	if ghOrg == nil {
		return nil, fmt.Errorf("organization %s not found", opts.OrgName)
	}
	org := github.OrganizationFromGhOrg(ghOrg)
	log.Printf("Organization %s retrieved successfully: %s", opts.OrgName, *org)
	return org, nil
}

// InviteUser invites a user to an organization by their user ID.
func InviteUser(ctx context.Context, opts InviteUserOptions) error {
	if opts.Service == nil {
		return github.ErrNilService
	}

	if opts.OrgName == "" {
		return github.ErrMissingRequiredField
	}

	if opts.UserID <= 0 {
		return fmt.Errorf("invalid user ID: %d", opts.UserID)
	}

	log.Printf("Inviting user %d to organization: %s", opts.UserID, opts.OrgName)
	createOrgInvitationOptoins := &gh.CreateOrgInvitationOptions{
		InviteeID: &opts.UserID,
	}
	invitation, _, err := opts.Service.CreateOrgInvitation(ctx, opts.OrgName, createOrgInvitationOptoins)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to invite user %d to organization %s", opts.UserID, opts.OrgName))
	}
	log.Printf("User %d invited to organization %s successfully (invitation ID: %v)", opts.UserID, opts.OrgName, invitation.GetID())
	return nil
}
