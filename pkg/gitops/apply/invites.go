package apply

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
	ghusers "github.com/orang-gaboets/repo-builder/pkg/github/users"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
)

func (e *executor) executeInviteAction(action gitopsplan.Action) error {
	if action.Operation != gitopsplan.ActionOperationCreate {
		return fmt.Errorf("unsupported invite operation %q for %s: %w", action.Operation, action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}

	invite, ok := e.desiredInvites[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired invite %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
	}

	options, err := e.createInvitationOptions(invite)
	if err != nil {
		return fmt.Errorf("create invite %s: %w", action.ResourceID, err)
	}

	return organizations.CreateInvitation(e.ctx, options)
}

func (e *executor) createInvitationOptions(invite config.InviteSpec) (organizations.CreateInvitationOptions, error) {
	options := organizations.CreateInvitationOptions{
		Service: e.organizationService,
		OrgName: e.organization,
		Role:    strings.TrimSpace(invite.Role),
	}

	if len(invite.TeamSlugs) > 0 {
		teamIDs, err := e.resolveInvitationTeamIDs(invite.TeamSlugs)
		if err != nil {
			return organizations.CreateInvitationOptions{}, err
		}
		options.TeamIDs = teamIDs
	}

	switch {
	case invite.Username.Present && !invite.Username.Null:
		username := strings.TrimSpace(invite.Username.Value)
		user, err := ghusers.GetUserByUsername(e.ctx, ghusers.GetUserByUsernameOptions{
			Service:  e.userService,
			Username: username,
		})
		if err != nil {
			return organizations.CreateInvitationOptions{}, err
		}
		if user == nil || user.ID == nil || *user.ID <= 0 {
			return organizations.CreateInvitationOptions{}, fmt.Errorf("resolved invite username %q without a valid user ID: %w", username, githubpkg.ErrInvalidFieldValue)
		}
		options.UserID = user.ID
	case invite.Email.Present && !invite.Email.Null:
		options.Email = strings.TrimSpace(invite.Email.Value)
	case invite.UserID.Present && !invite.UserID.Null:
		userID := invite.UserID.Value
		options.UserID = &userID
	default:
		return organizations.CreateInvitationOptions{}, fmt.Errorf("invite identity declaration is invalid: %w", githubpkg.ErrInvalidFieldValue)
	}

	return options, nil
}

func (e *executor) resolveInvitationTeamIDs(teamSlugs []string) ([]int64, error) {
	teamIDs := make([]int64, 0, len(teamSlugs))
	for _, teamSlug := range teamSlugs {
		id, ok := e.teamIDs[teamSlugKey(teamSlug)]
		if !ok || id <= 0 {
			return nil, fmt.Errorf("team slug %q does not have a resolvable team ID: %w", teamSlug, githubpkg.ErrNotFound)
		}
		teamIDs = append(teamIDs, id)
	}
	return teamIDs, nil
}
