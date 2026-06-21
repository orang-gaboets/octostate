package plan

import (
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func repositoryCanBeCreated(repository config.RepositorySpec) bool {
	return repository.Template.Owner != "" && repository.Template.Name != ""
}

func repositoryAvailableForTeamRepositoryPermission(owner, name string, actualRepos map[string]state.Repository, desiredRepos map[string]config.RepositorySpec) bool {
	key := repositoryKey(owner, name)

	if _, ok := actualRepos[key]; ok {
		return true
	}

	repository, ok := desiredRepos[key]
	if !ok {
		return false
	}

	return repositoryCanBeCreated(repository)
}
