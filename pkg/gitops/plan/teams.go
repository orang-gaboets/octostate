package plan

import (
	"fmt"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func (p planner) planTeams() []Action {
	actions := make([]Action, 0)
	actualTeams := make(map[string]state.Team, len(p.actual.Teams))
	for _, team := range p.actual.Teams {
		actualTeams[teamKey(team.Slug)] = team
	}

	desiredTeams := make(map[string]config.TeamSpec, len(p.desired.Teams))
	for _, team := range p.desired.Teams {
		key := teamKey(team.Slug)
		desiredTeams[key] = team
		actualTeam, ok := actualTeams[key]
		if !ok {
			actions = append(actions, Action{
				ResourceType: ActionResourceTypeTeam,
				Operation:    ActionOperationCreate,
				ResourceID:   teamID(team.Slug),
				Executable:   true,
				Message:      fmt.Sprintf("create team %s", teamID(team.Slug)),
			})
			continue
		}

		changes := make([]FieldChange, 0, 4)
		if actualTeam.Name != team.Name {
			changes = append(changes, FieldChange{Field: "name", From: actualTeam.Name, To: team.Name})
		}
		if actualTeam.Description != team.Description {
			changes = append(changes, FieldChange{Field: "description", From: actualTeam.Description, To: team.Description})
		}
		if actualTeam.Privacy != team.Privacy {
			changes = append(changes, FieldChange{Field: "privacy", From: actualTeam.Privacy, To: team.Privacy})
		}
		if actualTeam.ParentSlug != team.ParentSlug {
			changes = append(changes, FieldChange{Field: "parent_slug", From: actualTeam.ParentSlug, To: team.ParentSlug})
		}
		if len(changes) == 0 {
			continue
		}

		actions = append(actions, Action{
			ResourceType: ActionResourceTypeTeam,
			Operation:    ActionOperationUpdate,
			ResourceID:   teamID(team.Slug),
			Executable:   true,
			Message:      fmt.Sprintf("update team %s", teamID(team.Slug)),
			Changes:      changes,
		})
	}

	for _, team := range p.actual.Teams {
		key := teamKey(team.Slug)
		if _, ok := desiredTeams[key]; ok {
			continue
		}
		actions = append(actions, Action{
			ResourceType: ActionResourceTypeTeam,
			Operation:    ActionOperationDelete,
			ResourceID:   teamID(team.Slug),
			Executable:   false,
			Message:      fmt.Sprintf("team %s exists in live state but is not declared in desired config", teamID(team.Slug)),
		})
	}

	return actions
}

func (p planner) planTeamMembers() []Action {
	actions := make([]Action, 0)
	actualMembers := make(map[string]state.TeamMember, len(p.actual.TeamMembers))
	for _, member := range p.actual.TeamMembers {
		actualMembers[teamMemberKey(member.TeamSlug, member.Username)] = member
	}
	actualOrganizationMembers := make(map[string]struct{}, len(p.actual.Members))
	for _, member := range p.actual.Members {
		actualOrganizationMembers[organizationMemberKey(member.Username)] = struct{}{}
	}

	desiredMembers := make(map[string]config.TeamMemberSpec)
	for _, team := range p.desired.Teams {
		for _, member := range team.Members {
			key := teamMemberKey(team.Slug, member.Username)
			desiredMembers[key] = member
			actualMember, ok := actualMembers[key]
			if !ok {
				_, organizationMemberExists := actualOrganizationMembers[organizationMemberKey(member.Username)]
				executable := organizationMemberExists
				message := fmt.Sprintf("add team membership %s", teamMemberID(team.Slug, member.Username))
				if !organizationMemberExists {
					message = fmt.Sprintf(
						"team membership %s requires organization member %s to exist first",
						teamMemberID(team.Slug, member.Username),
						organizationMemberID(member.Username),
					)
				}
				actions = append(actions, Action{
					ResourceType: ActionResourceTypeTeamMember,
					Operation:    ActionOperationCreate,
					ResourceID:   teamMemberID(team.Slug, member.Username),
					Executable:   executable,
					Message:      message,
				})
				continue
			}
			if actualMember.Role == member.Role {
				continue
			}
			actions = append(actions, Action{
				ResourceType: ActionResourceTypeTeamMember,
				Operation:    ActionOperationUpdate,
				ResourceID:   teamMemberID(team.Slug, member.Username),
				Executable:   true,
				Message:      fmt.Sprintf("update team membership %s", teamMemberID(team.Slug, member.Username)),
				Changes: []FieldChange{{
					Field: "role",
					From:  actualMember.Role,
					To:    member.Role,
				}},
			})
		}
	}

	for _, member := range p.actual.TeamMembers {
		key := teamMemberKey(member.TeamSlug, member.Username)
		if _, ok := desiredMembers[key]; ok {
			continue
		}
		actions = append(actions, Action{
			ResourceType: ActionResourceTypeTeamMember,
			Operation:    ActionOperationRemove,
			ResourceID:   teamMemberID(member.TeamSlug, member.Username),
			Executable:   false,
			Message:      fmt.Sprintf("team membership %s exists in live state but is not declared in desired config", teamMemberID(member.TeamSlug, member.Username)),
		})
	}

	return actions
}

func (p planner) planTeamRepositoryPermissions() []Action {
	actions := make([]Action, 0)
	actualPermissions := make(map[string]state.TeamRepositoryPermission, len(p.actual.TeamRepositoryPermissions))
	for _, permission := range p.actual.TeamRepositoryPermissions {
		actualPermissions[teamRepositoryPermissionKey(permission.TeamSlug, permission.Owner, permission.Name)] = permission
	}
	actualRepos := make(map[string]state.Repository, len(p.actual.Repositories))
	for _, repository := range p.actual.Repositories {
		actualRepos[repositoryKey(repository.Owner, repository.Name)] = repository
	}

	desiredRepos := make(map[string]config.RepositorySpec, len(p.desired.Repositories))
	for _, repository := range p.desired.Repositories {
		desiredRepos[repositoryKey(repository.Owner, repository.Name)] = repository
	}

	desiredPermissions := make(map[string]config.TeamRepositorySpec)
	for _, team := range p.desired.Teams {
		for _, permission := range team.Repositories {
			key := teamRepositoryPermissionKey(team.Slug, permission.Owner, permission.Name)
			desiredPermissions[key] = permission
			actualPermission, ok := actualPermissions[key]
			if !ok {
				executable := repositoryAvailableForTeamRepositoryPermission(permission.Owner, permission.Name, actualRepos, desiredRepos)
				message := fmt.Sprintf("create team repository permission %s", teamRepositoryPermissionID(team.Slug, permission.Owner, permission.Name))
				if !executable {
					message = fmt.Sprintf(
						"team repository permission %s requires repository %s/%s to exist or be created earlier in the same plan",
						teamRepositoryPermissionID(team.Slug, permission.Owner, permission.Name),
						permission.Owner,
						permission.Name,
					)
				}
				actions = append(actions, Action{
					ResourceType: ActionResourceTypeTeamRepositoryPermission,
					Operation:    ActionOperationCreate,
					ResourceID:   teamRepositoryPermissionID(team.Slug, permission.Owner, permission.Name),
					Executable:   executable,
					Message:      message,
				})
				continue
			}
			if actualPermission.Permission == permission.Permission {
				continue
			}
			executable := repositoryAvailableForTeamRepositoryPermission(permission.Owner, permission.Name, actualRepos, desiredRepos)
			message := fmt.Sprintf("update team repository permission %s", teamRepositoryPermissionID(team.Slug, permission.Owner, permission.Name))
			if !executable {
				message = fmt.Sprintf(
					"team repository permission %s requires repository %s/%s to exist or be created earlier in the same plan",
					teamRepositoryPermissionID(team.Slug, permission.Owner, permission.Name),
					permission.Owner,
					permission.Name,
				)
			}
			actions = append(actions, Action{
				ResourceType: ActionResourceTypeTeamRepositoryPermission,
				Operation:    ActionOperationUpdate,
				ResourceID:   teamRepositoryPermissionID(team.Slug, permission.Owner, permission.Name),
				Executable:   executable,
				Message:      message,
				Changes: []FieldChange{{
					Field: "permission",
					From:  actualPermission.Permission,
					To:    permission.Permission,
				}},
			})
		}
	}

	for _, permission := range p.actual.TeamRepositoryPermissions {
		key := teamRepositoryPermissionKey(permission.TeamSlug, permission.Owner, permission.Name)
		if _, ok := desiredPermissions[key]; ok {
			continue
		}
		actions = append(actions, Action{
			ResourceType: ActionResourceTypeTeamRepositoryPermission,
			Operation:    ActionOperationRemove,
			ResourceID:   teamRepositoryPermissionID(permission.TeamSlug, permission.Owner, permission.Name),
			Executable:   false,
			Message:      fmt.Sprintf("team repository permission %s exists in live state but is not declared in desired config", teamRepositoryPermissionID(permission.TeamSlug, permission.Owner, permission.Name)),
		})
	}

	return actions
}
