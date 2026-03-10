package organizations

import (
	"context"
	"fmt"
	"strconv"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	ghlogging "github.com/orang-gaboets/repo-builder/pkg/github/logging"
)

// Get retrieves an organization by its name.
func Get(ctx context.Context, option GetOptions) (*github.Organization, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	ghlogging.Debugf(ctx, "get organization %s", option.OrgName)
	ghOrg, _, err := option.Service.Get(ctx, option.OrgName)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to get organization %s", option.OrgName))
	}
	if ghOrg == nil {
		return nil, fmt.Errorf("organization %s not found", option.OrgName)
	}
	org := github.OrganizationFromGhOrg(ghOrg)
	ghlogging.Debugf(ctx, "retrieved organization %s", option.OrgName)
	return org, nil
}

// InviteUser invites a user to an organization by their user ID.
func InviteUser(ctx context.Context, option InviteUserOptions) error {
	if err := option.Validate(); err != nil {
		return err
	}

	ghlogging.Debugf(ctx, "invite user %d to organization %s", option.UserID, option.OrgName)
	createOrgInvitationOptoins := &gh.CreateOrgInvitationOptions{
		InviteeID: &option.UserID,
	}
	_, _, err := option.Service.CreateOrgInvitation(ctx, option.OrgName, createOrgInvitationOptoins)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to invite user %d to organization %s", option.UserID, option.OrgName))
	}
	ghlogging.Debugf(ctx, "invited user %d to organization %s", option.UserID, option.OrgName)
	return nil
}

// ListMembers retrieves all members of a GitHub organization.
func ListMembers(ctx context.Context, option ListMembersOptions) ([]*github.User, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	listOptions := &gh.ListMembersOptions{
		Role: string(option.Role),
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}

	var members []*github.User
	for {
		ghMembers, resp, err := option.Service.ListMembers(ctx, option.OrgName, listOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to list members for organization %s", option.OrgName))
		}

		members = append(members, github.UsersFromGhUsers(ghMembers)...)

		if resp == nil || resp.NextPage == 0 {
			break
		}

		listOptions.Page = resp.NextPage
	}

	ghlogging.Debugf(ctx, "listed %d members for organization %s", len(members), option.OrgName)
	return members, nil
}

// ListPendingInvitations retrieves all pending invitations of a GitHub organization.
func ListPendingInvitations(ctx context.Context, option ListPendingInvitationsOptions) ([]*github.OrganizationInvitation, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	listOptions := &gh.ListOptions{
		PerPage: 100,
	}

	var invitations []*github.OrganizationInvitation
	for {
		ghInvitations, resp, err := option.Service.ListPendingOrgInvitations(ctx, option.OrgName, listOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to list pending invitations for organization %s", option.OrgName))
		}

		invitations = append(invitations, github.OrganizationInvitationsFromGhInvitations(ghInvitations)...)

		if resp == nil || resp.NextPage == 0 {
			break
		}

		listOptions.Page = resp.NextPage
	}

	ghlogging.Debugf(ctx, "listed %d pending invitations for organization %s", len(invitations), option.OrgName)
	return invitations, nil
}

// ListInvitationTeams retrieves all teams attached to an organization
// invitation.
func ListInvitationTeams(ctx context.Context, option ListInvitationTeamsOptions) ([]*github.Team, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	listOptions := &gh.ListOptions{
		PerPage: 100,
	}

	var teams []*github.Team
	invitationID := strconv.FormatInt(option.InvitationID, 10)
	for {
		ghTeams, resp, err := option.Service.ListOrgInvitationTeams(ctx, option.OrgName, invitationID, listOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to list invitation teams for organization %s invitation %d", option.OrgName, option.InvitationID))
		}

		teams = append(teams, github.TeamsFromGhTeams(ghTeams)...)

		if resp == nil || resp.NextPage == 0 {
			break
		}

		listOptions.Page = resp.NextPage
	}

	ghlogging.Debugf(ctx, "listed %d invitation teams for organization %s invitation %d", len(teams), option.OrgName, option.InvitationID)
	return teams, nil
}
