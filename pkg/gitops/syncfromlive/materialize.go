package syncfromlive

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// MaterializeOptions defines the inputs required to fill unmanaged repository
// fields from live GitHub state back into an existing desired GitOps config.
type MaterializeOptions struct {
	Desired config.OrganizationConfig
	Actual  *state.OrganizationState
}

// Validate checks whether the materialize inputs are usable.
func (opt *MaterializeOptions) Validate() error {
	desiredOrganization := strings.TrimSpace(opt.Desired.Organization)
	switch {
	case desiredOrganization == "":
		return fmt.Errorf("desired organization is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Actual == nil:
		return fmt.Errorf("actual state is required: %w", githubpkg.ErrMissingRequiredField)
	case strings.TrimSpace(opt.Actual.Organization) == "":
		return fmt.Errorf("actual organization is required: %w", githubpkg.ErrMissingRequiredField)
	case !strings.EqualFold(strings.TrimSpace(opt.Actual.Organization), desiredOrganization):
		return fmt.Errorf(
			"actual organization %q does not match desired organization %q: %w",
			opt.Actual.Organization,
			opt.Desired.Organization,
			githubpkg.ErrInvalidFieldValue,
		)
	}

	if err := config.ValidateAndError(opt.Desired); err != nil {
		return err
	}
	return nil
}

// BuildMaterializeConfig fills currently unmanaged repository optional fields
// from live GitHub state without adopting new resources or removing config.
func BuildMaterializeConfig(opt MaterializeOptions) (config.OrganizationConfig, error) {
	if err := opt.Validate(); err != nil {
		return config.OrganizationConfig{}, err
	}

	desired := cloneDesiredConfig(opt.Desired)
	actual := cloneOrganizationState(opt.Actual)
	organization := strings.TrimSpace(desired.Organization)

	desired.Organization = organization
	desired.Repositories = materializeRepositories(organization, desired.Repositories, actual.Repositories)

	return desired, nil
}

func materializeRepositories(
	organization string,
	desired []config.RepositorySpec,
	actual []state.Repository,
) []config.RepositorySpec {
	materialized := cloneDesiredRepositories(desired)

	actualByRepository := make(map[string]state.Repository, len(actual))
	for _, repository := range actual {
		actualByRepository[repositoryAdoptKey(organization, repository.Owner, repository.Name)] = repository
	}

	for i, repository := range materialized {
		actualRepository, ok := actualByRepository[repositoryAdoptKey(organization, repository.Owner, repository.Name)]
		if !ok {
			continue
		}
		materialized[i] = materializeRepository(repository, actualRepository)
	}

	return materialized
}

func materializeRepository(
	desired config.RepositorySpec,
	actual state.Repository,
) config.RepositorySpec {
	materialized := desired

	if _, managed := desired.ManagedDescription(); !managed {
		materialized.SetManagedDescription(actual.Description)
	}
	if _, managed := desired.ManagedHomepage(); !managed {
		materialized.SetManagedHomepage(actual.Homepage)
	}
	if config.IsPrivateVisibility(desired.Visibility) {
		if _, managed := desired.ManagedAllowForking(); !managed {
			materialized.SetManagedAllowForking(actual.AllowForking)
		}
	}
	if _, managed := desired.ManagedArchived(); !managed {
		materialized.SetManagedArchived(actual.Archived)
	}
	if _, managed := desired.ManagedIsTemplate(); !managed {
		materialized.SetManagedIsTemplate(actual.IsTemplate)
	}

	return materialized
}
