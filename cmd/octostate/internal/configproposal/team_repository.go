package configproposal

import (
	"strings"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// FindTeamRepositoryIndex returns the index of a team repository permission
// entry matching owner and name within team. An empty owner on either side
// resolves to organization, and identity is case-insensitive after trimming.
func FindTeamRepositoryIndex(team *gitopsconfig.TeamSpec, organization, owner, name string) (int, bool) {
	if team == nil {
		return -1, false
	}

	trimmedOrganization := strings.TrimSpace(organization)
	wantOwner := strings.TrimSpace(owner)
	if wantOwner == "" {
		wantOwner = trimmedOrganization
	}
	wantName := strings.TrimSpace(name)

	for index, repository := range team.Repositories {
		repositoryOwner := strings.TrimSpace(repository.Owner)
		if repositoryOwner == "" {
			repositoryOwner = trimmedOrganization
		}
		if strings.EqualFold(repositoryOwner, wantOwner) &&
			strings.EqualFold(strings.TrimSpace(repository.Name), wantName) {
			return index, true
		}
	}

	return -1, false
}
