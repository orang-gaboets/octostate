package organizations

import (
	"context"
	"fmt"
	"log"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// Get retrieves an organization by its name.
func Get(ctx context.Context, option GetOptions) (*github.Organization, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	log.Printf("Retrieving organization: %s", option.OrgName)
	ghOrg, _, err := option.Service.Get(ctx, option.OrgName)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to get organization %s", option.OrgName))
	}
	if ghOrg == nil {
		return nil, fmt.Errorf("organization %s not found", option.OrgName)
	}
	org := github.OrganizationFromGhOrg(ghOrg)
	log.Printf("Organization %s retrieved successfully: %s", option.OrgName, *org)
	return org, nil
}

// InviteUser invites a user to an organization by their user ID.
func InviteUser(ctx context.Context, option InviteUserOptions) error {
	if err := option.Validate(); err != nil {
		return err
	}

	log.Printf("Inviting user %d to organization: %s", option.UserID, option.OrgName)
	createOrgInvitationOptoins := &gh.CreateOrgInvitationOptions{
		InviteeID: &option.UserID,
	}
	invitation, _, err := option.Service.CreateOrgInvitation(ctx, option.OrgName, createOrgInvitationOptoins)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to invite user %d to organization %s", option.UserID, option.OrgName))
	}
	log.Printf("User %d invited to organization %s successfully (invitation ID: %v)", option.UserID, option.OrgName, invitation.GetID())
	return nil
}
