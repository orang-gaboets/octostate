package syncfromlive

import (
	"strings"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func cloneOrganizationState(actual *state.OrganizationState) state.OrganizationState {
	if actual == nil {
		return state.OrganizationState{}
	}

	cloned := state.OrganizationState{
		Organization: strings.TrimSpace(actual.Organization),
		Members:      append([]state.OrganizationMember{}, actual.Members...),
		PendingInvitations: clonePendingInvitations(
			actual.PendingInvitations,
		),
		Repositories: cloneRepositories(actual.Repositories),
		Teams:        append([]state.Team{}, actual.Teams...),
		TeamMembers: append(
			[]state.TeamMember{},
			actual.TeamMembers...,
		),
		TeamRepositoryPermissions: append(
			[]state.TeamRepositoryPermission{},
			actual.TeamRepositoryPermissions...,
		),
	}
	cloned.Normalize()
	return cloned
}

func clonePendingInvitations(invitations []state.PendingInvitation) []state.PendingInvitation {
	cloned := make([]state.PendingInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		cloned = append(cloned, state.PendingInvitation{
			ID:        invitation.ID,
			Username:  invitation.Username,
			Email:     invitation.Email,
			Role:      invitation.Role,
			TeamSlugs: append([]string{}, invitation.TeamSlugs...),
		})
	}
	return cloned
}

func cloneRepositories(repositories []state.Repository) []state.Repository {
	cloned := make([]state.Repository, 0, len(repositories))
	for _, repository := range repositories {
		cloned = append(cloned, state.Repository{
			Owner:        repository.Owner,
			Name:         repository.Name,
			Visibility:   repository.Visibility,
			Description:  repository.Description,
			Homepage:     repository.Homepage,
			Topics:       append([]string{}, repository.Topics...),
			AllowForking: repository.AllowForking,
			Archived:     repository.Archived,
			IsTemplate:   repository.IsTemplate,
		})
	}
	return cloned
}
