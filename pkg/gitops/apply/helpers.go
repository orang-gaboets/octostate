package apply

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
)

func repositoryResourceID(owner, name string) string {
	return owner + "/" + name
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

func visibilityPrivateFlag(visibility string) (bool, error) {
	switch strings.TrimSpace(visibility) {
	case "private":
		return true, nil
	case "public":
		return false, nil
	case "internal":
		return false, fmt.Errorf("repository visibility %q is not supported by apply yet: %w", visibility, githubpkg.ErrInvalidFieldValue)
	default:
		return false, fmt.Errorf("repository visibility %q is invalid: %w", visibility, githubpkg.ErrInvalidFieldValue)
	}
}

func teamPrivacyPointer(privacy string) (*githubpkg.TeamPrivacy, error) {
	value := githubpkg.TeamPrivacy(strings.TrimSpace(privacy))
	if !value.IsValid() {
		return nil, fmt.Errorf("team privacy %q is invalid: %w", privacy, githubpkg.ErrInvalidFieldValue)
	}
	return &value, nil
}
