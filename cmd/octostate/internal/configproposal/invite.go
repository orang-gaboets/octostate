package configproposal

import (
	"strings"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// FindInviteIndexByUsername returns the index of a declared invite whose
// identity is the given username. Identity is case-insensitive after trimming.
func FindInviteIndexByUsername(cfg *gitopsconfig.OrganizationConfig, username string) (int, bool) {
	if cfg == nil {
		return -1, false
	}

	wantUsername := strings.TrimSpace(username)
	for index, invite := range cfg.Invites {
		if !invite.Username.Present || invite.Username.Null {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(invite.Username.Value), wantUsername) {
			return index, true
		}
	}

	return -1, false
}

// FindInviteIndexByUserID returns the index of a declared invite whose identity
// is the given user ID.
func FindInviteIndexByUserID(cfg *gitopsconfig.OrganizationConfig, userID int64) (int, bool) {
	if cfg == nil {
		return -1, false
	}

	for index, invite := range cfg.Invites {
		if !invite.UserID.Present || invite.UserID.Null {
			continue
		}
		if invite.UserID.Value == userID {
			return index, true
		}
	}

	return -1, false
}
