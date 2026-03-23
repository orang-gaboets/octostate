package syncfromlive

import (
	"strings"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
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
		if !strings.EqualFold(strings.TrimSpace(repository.Visibility), "private") {
			repo.SetManagedAllowForking(repository.AllowForking)
		}
		repo.SetManagedArchived(repository.Archived)
		repo.SetManagedIsTemplate(repository.IsTemplate)

		desired = append(desired, repo)
	}

	return desired
}

func bootstrapOwner(organization, owner string) string {
	owner = strings.TrimSpace(owner)
	if owner == "" || strings.EqualFold(owner, strings.TrimSpace(organization)) {
		return ""
	}
	return owner
}
