package apply

import (
	"context"
	"fmt"
	"strings"

	"github.com/orang-gaboets/repo-builder/internal/orderedtasks"
	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
	ghusers "github.com/orang-gaboets/repo-builder/pkg/github/users"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
)

const inviteUsernameResolutionConcurrency = 8

type inviteUsernameTarget struct {
	key      string
	username string
}

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
		userID, err := e.resolveInviteUserID(strings.TrimSpace(invite.Username.Value))
		if err != nil {
			return organizations.CreateInvitationOptions{}, err
		}
		options.UserID = githubpkg.Ptr(userID)
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

// Invite username pre-resolution uses concurrent read-only user lookups; the
// surrounding apply loop still performs GitHub write operations sequentially.
func (e *executor) preResolveInviteUsernames(actions []gitopsplan.Action) error {
	if e.inviteUsersResolved {
		return nil
	}
	e.inviteUsersResolved = true

	targets := e.collectInviteUsernameTargets(actions)
	if len(targets) == 0 {
		return nil
	}

	resolvedUserIDs := make([]int64, len(targets))
	tasks := make([]orderedtasks.Task, 0, len(targets))
	for i, target := range targets {
		tasks = append(tasks, func(ctx context.Context) error {
			userID, err := e.lookupInviteUserID(ctx, target.username)
			if err != nil {
				return err
			}
			resolvedUserIDs[i] = userID
			return nil
		})
	}

	if err := orderedtasks.Run(e.ctx, inviteUsernameResolutionConcurrency, tasks); err != nil {
		return err
	}

	for i, target := range targets {
		e.resolvedInviteUserIDs[target.key] = resolvedUserIDs[i]
	}
	return nil
}

func (e *executor) collectInviteUsernameTargets(actions []gitopsplan.Action) []inviteUsernameTarget {
	seen := make(map[string]struct{}, len(actions))
	targets := make([]inviteUsernameTarget, 0, len(actions))

	for _, action := range actions {
		if !action.Executable ||
			action.ResourceType != gitopsplan.ActionResourceTypeInvite ||
			action.Operation != gitopsplan.ActionOperationCreate {
			continue
		}

		invite, ok := e.desiredInvites[action.ResourceID]
		if !ok || !invite.Username.Present || invite.Username.Null {
			continue
		}

		username := strings.TrimSpace(invite.Username.Value)
		key := inviteUsernameKey(username)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, inviteUsernameTarget{
			key:      key,
			username: username,
		})
	}

	return targets
}

func (e *executor) resolveInviteUserID(username string) (int64, error) {
	key := inviteUsernameKey(username)
	if userID, ok := e.resolvedInviteUserIDs[key]; ok && userID > 0 {
		return userID, nil
	}

	userID, err := e.lookupInviteUserID(e.ctx, strings.TrimSpace(username))
	if err != nil {
		return 0, err
	}
	e.resolvedInviteUserIDs[key] = userID
	return userID, nil
}

func (e *executor) lookupInviteUserID(ctx context.Context, username string) (int64, error) {
	user, err := ghusers.GetUserByUsername(ctx, ghusers.GetUserByUsernameOptions{
		Service:  e.userService,
		Username: username,
	})
	if err != nil {
		return 0, err
	}
	if user == nil || user.ID == nil || *user.ID <= 0 {
		return 0, fmt.Errorf("resolved invite username %q without a valid user ID: %w", username, githubpkg.ErrInvalidFieldValue)
	}
	return *user.ID, nil
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
