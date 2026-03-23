package diff

import (
	"slices"
	"strings"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/internal/resourceid"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func repositoryID(owner, name string) string {
	return resourceid.RepositoryID(owner, name)
}

func repositoryKey(owner, name string) string {
	return resourceid.RepositoryKey(owner, name)
}

func teamID(slug string) string {
	return resourceid.TeamID(slug)
}

func teamKey(slug string) string {
	return resourceid.TeamKey(slug)
}

func organizationMemberID(username string) string {
	return resourceid.OrganizationMemberID(username)
}

func organizationMemberKey(username string) string {
	return resourceid.OrganizationMemberKey(username)
}

func teamMemberID(teamSlug, username string) string {
	return resourceid.TeamMemberID(teamSlug, username)
}

func teamMemberKey(teamSlug, username string) string {
	return resourceid.TeamMemberKey(teamSlug, username)
}

func teamRepositoryPermissionID(teamSlug, owner, name string) string {
	return resourceid.TeamRepositoryPermissionID(teamSlug, owner, name)
}

func teamRepositoryPermissionKey(teamSlug, owner, name string) string {
	return resourceid.TeamRepositoryPermissionKey(teamSlug, owner, name)
}

func pendingInvitationID(invitation state.PendingInvitation) string {
	return resourceid.PendingInvitationID(invitation)
}

func clonePendingInvitations(invitations []state.PendingInvitation) []state.PendingInvitation {
	result := make([]state.PendingInvitation, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, state.PendingInvitation{
			ID:        invitation.ID,
			Username:  invitation.Username,
			Email:     invitation.Email,
			Role:      invitation.Role,
			TeamSlugs: append([]string{}, invitation.TeamSlugs...),
		})
	}
	return result
}

func cloneRepositories(repositories []state.Repository) []state.Repository {
	result := make([]state.Repository, 0, len(repositories))
	for _, repository := range repositories {
		result = append(result, state.Repository{
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
	return result
}

func equalStringSets(a, b []string) bool {
	aValues := sortedUniqueStrings(a)
	bValues := sortedUniqueStrings(b)
	if len(aValues) != len(bValues) {
		return false
	}
	for i := range aValues {
		if compareStrings(aValues[i], bValues[i]) != 0 {
			return false
		}
	}
	return true
}

func sortedUniqueStrings(values []string) []string {
	sorted := sortedStrings(values)
	if len(sorted) == 0 {
		return sorted
	}

	unique := make([]string, 0, len(sorted))
	last := sorted[0]
	unique = append(unique, last)
	for i := 1; i < len(sorted); i++ {
		if compareStrings(sorted[i], last) == 0 {
			continue
		}
		last = sorted[i]
		unique = append(unique, last)
	}
	return unique
}

func sortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := append([]string{}, values...)
	slices.SortFunc(result, compareStrings)
	return result
}

func compareStrings(a, b string) int {
	aKey := strings.ToLower(a)
	bKey := strings.ToLower(b)
	if aKey < bKey {
		return -1
	}
	if aKey > bKey {
		return 1
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
