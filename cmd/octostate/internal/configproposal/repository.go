package configproposal

import (
	"strings"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// FindRepositoryIndex returns the index of a repository matching owner and name.
// Repository identity is case-insensitive after trimming both values.
func FindRepositoryIndex(cfg *gitopsconfig.OrganizationConfig, owner, name string) (int, bool) {
	if cfg == nil {
		return -1, false
	}

	wantOwner := strings.TrimSpace(owner)
	wantName := strings.TrimSpace(name)
	for index, repository := range cfg.Repositories {
		if strings.EqualFold(strings.TrimSpace(repository.Owner), wantOwner) &&
			strings.EqualFold(strings.TrimSpace(repository.Name), wantName) {
			return index, true
		}
	}

	return -1, false
}
