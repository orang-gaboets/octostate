package apply

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/internal/resourceid"
)

func repositoryResourceID(owner, name string) string {
	return resourceid.RepositoryID(owner, name)
}

func repositoryKey(owner, name string) string {
	return resourceid.RepositoryKey(owner, name)
}

func teamResourceID(slug string) string {
	return slug
}

func organizationMemberResourceID(username string) string {
	return username
}

func teamMemberResourceID(teamSlug, username string) string {
	return teamSlug + "/" + username
}

func teamRepoPermissionResourceID(teamSlug, owner, name string) string {
	return teamSlug + "/" + owner + "/" + name
}

func teamSlugKey(slug string) string {
	return strings.ToLower(strings.TrimSpace(slug))
}

func inviteUsernameKey(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}

func desiredInviteResourceID(invite config.InviteSpec) (string, error) {
	usernamePresent := invite.Username.Present && !invite.Username.Null
	emailPresent := invite.Email.Present && !invite.Email.Null
	userIDPresent := invite.UserID.Present && !invite.UserID.Null

	presentCount := 0
	if usernamePresent {
		presentCount++
	}
	if emailPresent {
		presentCount++
	}
	if userIDPresent {
		presentCount++
	}
	if presentCount != 1 {
		return "", fmt.Errorf("invite identity declaration is invalid: %w", githubpkg.ErrInvalidFieldValue)
	}

	if usernamePresent {
		username := strings.TrimSpace(invite.Username.Value)
		if username == "" {
			return "", fmt.Errorf("invite username is invalid: %w", githubpkg.ErrInvalidFieldValue)
		}
		return "username:" + username, nil
	}
	if emailPresent {
		email := strings.TrimSpace(invite.Email.Value)
		if email == "" {
			return "", fmt.Errorf("invite email is invalid: %w", githubpkg.ErrInvalidFieldValue)
		}
		return "email:" + email, nil
	}
	if invite.UserID.Value <= 0 {
		return "", fmt.Errorf("invite user_id is invalid: %w", githubpkg.ErrInvalidFieldValue)
	}
	return fmt.Sprintf("user_id:%d", invite.UserID.Value), nil
}

func repositoryVisibility(visibility string) (string, error) {
	switch strings.TrimSpace(visibility) {
	case "private":
		return "private", nil
	case "public":
		return "public", nil
	case "internal":
		return "internal", nil
	default:
		return "", fmt.Errorf("repository visibility %q is invalid: %w", visibility, githubpkg.ErrInvalidFieldValue)
	}
}

func teamPrivacyPointer(privacy string) (*githubpkg.TeamPrivacy, error) {
	value := githubpkg.TeamPrivacy(strings.TrimSpace(privacy))
	if !value.IsValid() {
		return nil, fmt.Errorf("team privacy %q is invalid: %w", privacy, githubpkg.ErrInvalidFieldValue)
	}
	return &value, nil
}
