package plan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func repositoryID(owner, name string) string {
	return owner + "/" + name
}

func repositoryKey(owner, name string) string {
	return strings.ToLower(repositoryID(owner, name))
}

func teamID(slug string) string {
	return slug
}

func teamKey(slug string) string {
	return strings.ToLower(slug)
}

func teamMemberID(teamSlug, username string) string {
	return teamSlug + "/" + username
}

func teamMemberKey(teamSlug, username string) string {
	return strings.ToLower(teamMemberID(teamSlug, username))
}

func teamRepositoryPermissionID(teamSlug, owner, name string) string {
	return teamSlug + "/" + owner + "/" + name
}

func teamRepositoryPermissionKey(teamSlug, owner, name string) string {
	return strings.ToLower(teamRepositoryPermissionID(teamSlug, owner, name))
}

func pendingInvitationID(invitation state.PendingInvitation) string {
	if invitation.Username != "" {
		return "username:" + invitation.Username
	}
	if invitation.Email != "" {
		return "email:" + invitation.Email
	}
	if invitation.ID > 0 {
		return fmt.Sprintf("invitation:%d", invitation.ID)
	}
	return "invitation"
}

func equalStringSets(a, b []string) bool {
	aValues := sortedStrings(a)
	bValues := sortedStrings(b)
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

func sortedStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := append([]string{}, values...)
	slices.SortFunc(result, compareStrings)
	return result
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
