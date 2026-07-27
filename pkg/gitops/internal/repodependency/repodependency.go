package repodependency

import (
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/internal/resourceid"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// CanCreateRepository reports whether a desired repository declares the
// template configuration required to create it.
func CanCreateRepository(repository config.RepositorySpec) bool {
	return repository.Template.Owner != "" && repository.Template.Name != ""
}

// RepositoryAvailable reports whether a repository referenced by a team
// repository permission exists in actual state or can be created earlier in
// the same plan.
func RepositoryAvailable(owner, name string, actualRepos map[string]state.Repository, desiredRepos map[string]config.RepositorySpec) bool {
	key := resourceid.RepositoryKey(owner, name)

	if _, ok := actualRepos[key]; ok {
		return true
	}

	repository, ok := desiredRepos[key]
	if !ok {
		return false
	}

	return CanCreateRepository(repository)
}
