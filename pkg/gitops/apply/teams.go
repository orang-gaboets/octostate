package apply

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
)

func (e *executor) executeTeamCreateGroup(actions []gitopsplan.Action, result *Result) error {
	pending := append([]gitopsplan.Action(nil), actions...)
	for len(pending) > 0 {
		progress := false
		nextPending := make([]gitopsplan.Action, 0, len(pending))
		for _, action := range pending {
			team, ok := e.desiredTeams[action.ResourceID]
			if !ok {
				return fmt.Errorf("desired team %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
			}
			if team.ParentSlug != "" {
				if _, ok := e.teamIDs[teamSlugKey(team.ParentSlug)]; !ok {
					nextPending = append(nextPending, action)
					continue
				}
			}
			if err := e.createTeam(action); err != nil {
				return err
			}
			result.Executed = append(result.Executed, action)
			progress = true
		}
		if progress {
			pending = nextPending
			continue
		}

		unresolved := make([]string, 0, len(nextPending))
		for _, action := range nextPending {
			unresolved = append(unresolved, action.ResourceID)
		}
		return fmt.Errorf("unable to resolve team create dependencies for %s: %w", strings.Join(unresolved, ", "), githubpkg.ErrInvalidFieldValue)
	}
	return nil
}

func (e *executor) executeTeamAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate:
		return e.createTeam(action)
	case gitopsplan.ActionOperationUpdate:
		return e.updateTeam(action)
	default:
		return fmt.Errorf("unsupported team operation %q for %s: %w", action.Operation, action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}
}

func (e *executor) createTeam(action gitopsplan.Action) error {
	team, ok := e.desiredTeams[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired team %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
	}

	privacy, err := teamPrivacyPointer(team.Privacy)
	if err != nil {
		return fmt.Errorf("create team %s: %w", action.ResourceID, err)
	}

	options := teams.CreateTeamOptions{
		Service:     e.teamService,
		Org:         e.organization,
		Name:        team.Name,
		Description: githubpkg.Ptr(team.Description),
		Privacy:     privacy,
	}
	if team.ParentSlug != "" {
		options.ParentTeamSlug = githubpkg.Ptr(team.ParentSlug)
	}

	createdTeam, err := teams.CreateTeam(e.ctx, options)
	if err != nil {
		return err
	}
	if createdTeam == nil || createdTeam.ID <= 0 {
		return fmt.Errorf("created team %s returned no team ID: %w", action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}
	if createdTeam.Slug != "" {
		e.teamIDs[teamSlugKey(createdTeam.Slug)] = createdTeam.ID
	}
	if team.Slug != "" {
		e.teamIDs[teamSlugKey(team.Slug)] = createdTeam.ID
	}
	return nil
}

func (e *executor) updateTeam(action gitopsplan.Action) error {
	team, ok := e.desiredTeams[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired team %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
	}

	options := teams.EditTeamBySlugOptions{
		Service: e.teamService,
		Org:     e.organization,
		Slug:    team.Slug,
	}

	for _, change := range action.Changes {
		switch change.Field {
		case "name":
			options.Name = githubpkg.Ptr(team.Name)
		case "description":
			options.Description = githubpkg.Ptr(team.Description)
		case "privacy":
			privacy, err := teamPrivacyPointer(team.Privacy)
			if err != nil {
				return fmt.Errorf("update team %s: %w", action.ResourceID, err)
			}
			options.Privacy = privacy
		case "parent_slug":
			if strings.TrimSpace(team.ParentSlug) == "" {
				options.RemoveParent = true
			} else {
				options.ParentTeamSlug = githubpkg.Ptr(team.ParentSlug)
			}
		default:
			return fmt.Errorf("unsupported team change field %q for %s: %w", change.Field, action.ResourceID, githubpkg.ErrInvalidFieldValue)
		}
	}

	updatedTeam, err := teams.EditTeamBySlug(e.ctx, options)
	if err != nil {
		return err
	}
	if updatedTeam != nil && updatedTeam.ID > 0 {
		e.teamIDs[teamSlugKey(team.Slug)] = updatedTeam.ID
		if updatedTeam.Slug != "" {
			e.teamIDs[teamSlugKey(updatedTeam.Slug)] = updatedTeam.ID
		}
	}
	return nil
}

func (e *executor) executeTeamMemberAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate, gitopsplan.ActionOperationUpdate:
	default:
		return fmt.Errorf("unsupported team member operation %q for %s: %w", action.Operation, action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}

	member, ok := e.desiredMembers[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired team membership %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
	}

	teamSlug, _, err := splitTeamMemberResourceID(action.ResourceID)
	if err != nil {
		return err
	}

	role := teams.TeamMemberAddRole(member.Role)
	if !role.IsValid() {
		return fmt.Errorf("team membership role %q is invalid for %s: %w", member.Role, action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}

	_, err = teams.AddTeamMemberBySlug(e.ctx, teams.AddTeamMemberBySlugOptions{
		Service:  e.teamService,
		Org:      e.organization,
		Slug:     teamSlug,
		Username: member.Username,
		Role:     role,
	})
	return err
}

func (e *executor) executeTeamRepositoryPermissionAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate, gitopsplan.ActionOperationUpdate:
	default:
		return fmt.Errorf("unsupported team repository permission operation %q for %s: %w", action.Operation, action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}

	permission, ok := e.desiredPermissions[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired team repository permission %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
	}

	teamSlug, _, _, err := splitTeamRepoPermissionResourceID(action.ResourceID)
	if err != nil {
		return err
	}

	repoPermission := teams.TeamRepoPermission(permission.Permission)
	if !repoPermission.IsValid() {
		return fmt.Errorf("team repository permission %q is invalid for %s: %w", permission.Permission, action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}

	return teams.AddTeamRepoPermissionBySlug(e.ctx, teams.AddTeamRepoPermissionBySlugOptions{
		Service:    e.teamService,
		Org:        e.organization,
		Slug:       teamSlug,
		RepoOwner:  permission.Owner,
		RepoName:   permission.Name,
		Permission: repoPermission,
	})
}
