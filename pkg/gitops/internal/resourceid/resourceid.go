package resourceid

import (
	"fmt"
	"strings"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

// RepositoryID returns the canonical repository resource ID.
func RepositoryID(owner, name string) string {
	return owner + "/" + name
}

// RepositoryKey returns the case-insensitive repository lookup key.
func RepositoryKey(owner, name string) string {
	return strings.ToLower(RepositoryID(owner, name))
}

// TeamID returns the canonical team resource ID.
func TeamID(slug string) string {
	return slug
}

// TeamKey returns the case-insensitive team lookup key.
func TeamKey(slug string) string {
	return strings.ToLower(slug)
}

// OrganizationMemberID returns the canonical organization member resource ID.
func OrganizationMemberID(username string) string {
	return username
}

// OrganizationMemberKey returns the case-insensitive organization member lookup key.
func OrganizationMemberKey(username string) string {
	return strings.ToLower(OrganizationMemberID(username))
}

// TeamMemberID returns the canonical team membership resource ID.
func TeamMemberID(teamSlug, username string) string {
	return teamSlug + "/" + username
}

// TeamMemberKey returns the case-insensitive team membership lookup key.
func TeamMemberKey(teamSlug, username string) string {
	return strings.ToLower(TeamMemberID(teamSlug, username))
}

// TeamRepositoryPermissionID returns the canonical team repository permission resource ID.
func TeamRepositoryPermissionID(teamSlug, owner, name string) string {
	return teamSlug + "/" + owner + "/" + name
}

// TeamRepositoryPermissionKey returns the case-insensitive team repository permission lookup key.
func TeamRepositoryPermissionKey(teamSlug, owner, name string) string {
	return strings.ToLower(TeamRepositoryPermissionID(teamSlug, owner, name))
}

// PendingInvitationID returns a stable resource ID for a pending invitation.
func PendingInvitationID(invitation state.PendingInvitation) string {
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
