package apply

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
)

func splitTeamMemberResourceID(resourceID string) (string, string, error) {
	parts := strings.SplitN(resourceID, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid team member resource ID %q: %w", resourceID, githubpkg.ErrInvalidFieldValue)
	}
	return parts[0], parts[1], nil
}

func splitTeamRepoPermissionResourceID(resourceID string) (string, string, string, error) {
	parts := strings.SplitN(resourceID, "/", 3)
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("invalid team repository permission resource ID %q: %w", resourceID, githubpkg.ErrInvalidFieldValue)
	}
	return parts[0], parts[1], parts[2], nil
}
