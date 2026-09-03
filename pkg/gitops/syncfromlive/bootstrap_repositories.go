package syncfromlive

import (
	"strings"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func bootstrapRepositories(organization string, repositories []state.Repository) []config.RepositorySpec {
	desired := make([]config.RepositorySpec, 0, len(repositories))

	for _, repository := range repositories {
		repo := config.RepositorySpec{
			Owner:      bootstrapOwner(organization, repository.Owner),
			Name:       strings.TrimSpace(repository.Name),
			Visibility: strings.TrimSpace(repository.Visibility),
			Topics:     append([]string{}, repository.Topics...),
		}
		repo.SetManagedDescription(repository.Description)
		repo.SetManagedHomepage(repository.Homepage)
		if config.IsPrivateVisibility(repository.Visibility) {
			repo.SetManagedAllowForking(repository.AllowForking)
		}
		repo.SetManagedArchived(repository.Archived)
		repo.SetManagedIsTemplate(repository.IsTemplate)

		desired = append(desired, repo)
	}

	return desired
}

func bootstrapOwner(organization, owner string) string {
	owner = config.ResolveRepositoryOwner(owner, organization)
	if config.RepositoryOwnerMatchesOrganization(owner, organization) {
		return ""
	}
	return owner
}
