package plan

import (
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func repositoryCanBeCreated(repository config.RepositorySpec) bool {
	return repository.Template.Owner != "" && repository.Template.Name != ""
}

func repositoryAvailableForTeamRepositoryPermission(owner, name string, actualRepos map[string]state.Repository, desiredRepos map[string]config.RepositorySpec) bool {
	if _, ok := actualRepos[repositoryKey(owner, name)]; ok {
		return true
	}

	repository, ok := desiredRepos[repositoryKey(owner, name)]
	if !ok {
		return false
	}

	return repositoryCanBeCreated(repository)
}
