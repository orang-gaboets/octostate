package diff

import (
	"fmt"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func (b builder) planRepositories() []Action {
	actions := make([]Action, 0)
	actualRepos := make(map[string]state.Repository, len(b.actual.Repositories))
	for _, repository := range b.actual.Repositories {
		actualRepos[repositoryKey(repository.Owner, repository.Name)] = repository
	}

	desiredRepos := make(map[string]config.RepositorySpec, len(b.desired.Repositories))
	for _, repository := range b.desired.Repositories {
		key := repositoryKey(repository.Owner, repository.Name)
		desiredRepos[key] = repository
		actualRepository, ok := actualRepos[key]
		if !ok {
			executable := repository.Template.Owner != "" && repository.Template.Name != ""
			message := fmt.Sprintf("create repository %s", repositoryID(repository.Owner, repository.Name))
			if !executable {
				message = fmt.Sprintf("repository %s cannot be created because template configuration is missing", repositoryID(repository.Owner, repository.Name))
			}
			actions = append(actions, Action{
				ResourceType: ActionResourceTypeRepository,
				Operation:    ActionOperationCreate,
				ResourceID:   repositoryID(repository.Owner, repository.Name),
				Executable:   executable,
				Message:      message,
			})
			continue
		}

		changes := make([]FieldChange, 0, 7)
		if actualRepository.Visibility != repository.Visibility {
			changes = append(changes, FieldChange{Field: "visibility", From: actualRepository.Visibility, To: repository.Visibility})
		}
		if actualRepository.Description != repository.Description {
			changes = append(changes, FieldChange{Field: "description", From: actualRepository.Description, To: repository.Description})
		}
		if actualRepository.Homepage != repository.Homepage {
			changes = append(changes, FieldChange{Field: "homepage", From: actualRepository.Homepage, To: repository.Homepage})
		}
		if !equalStringSets(actualRepository.Topics, repository.Topics) {
			changes = append(changes, FieldChange{Field: "topics", From: sortedStrings(actualRepository.Topics), To: sortedStrings(repository.Topics)})
		}
		if repository.Visibility != "private" && actualRepository.AllowForking != repository.AllowForking {
			changes = append(changes, FieldChange{Field: "allow_forking", From: actualRepository.AllowForking, To: repository.AllowForking})
		}
		if actualRepository.Archived != repository.Archived {
			changes = append(changes, FieldChange{Field: "archived", From: actualRepository.Archived, To: repository.Archived})
		}
		if actualRepository.IsTemplate != repository.IsTemplate {
			changes = append(changes, FieldChange{Field: "is_template", From: actualRepository.IsTemplate, To: repository.IsTemplate})
		}
		if len(changes) == 0 {
			continue
		}

		actions = append(actions, Action{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationUpdate,
			ResourceID:   repositoryID(repository.Owner, repository.Name),
			Executable:   true,
			Message:      fmt.Sprintf("update repository %s", repositoryID(repository.Owner, repository.Name)),
			Changes:      changes,
		})
	}

	for _, repository := range b.actual.Repositories {
		key := repositoryKey(repository.Owner, repository.Name)
		if _, ok := desiredRepos[key]; ok {
			continue
		}
		actions = append(actions, Action{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationDelete,
			ResourceID:   repositoryID(repository.Owner, repository.Name),
			Executable:   false,
			Message:      fmt.Sprintf("repository %s exists in snapshot state but is not declared in desired config", repositoryID(repository.Owner, repository.Name)),
		})
	}

	return actions
}
