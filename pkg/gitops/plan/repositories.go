package plan

import (
	"fmt"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func (p planner) planRepositories() []Action {
	actions := make([]Action, 0)
	actualRepos := make(map[string]state.Repository, len(p.actual.Repositories))
	for _, repository := range p.actual.Repositories {
		actualRepos[repositoryKey(repository.Owner, repository.Name)] = repository
	}

	desiredRepos := make(map[string]config.RepositorySpec, len(p.desired.Repositories))
	for _, repository := range p.desired.Repositories {
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
		if description, managed := repository.ManagedDescription(); managed && actualRepository.Description != description {
			changes = append(changes, FieldChange{Field: "description", From: actualRepository.Description, To: description})
		}
		if homepage, managed := repository.ManagedHomepage(); managed && actualRepository.Homepage != homepage {
			changes = append(changes, FieldChange{Field: "homepage", From: actualRepository.Homepage, To: homepage})
		}
		if !equalStringSets(actualRepository.Topics, repository.Topics) {
			changes = append(changes, FieldChange{Field: "topics", From: sortedStrings(actualRepository.Topics), To: sortedStrings(repository.Topics)})
		}
		if allowForking, managed := repository.ManagedAllowForking(); managed && repository.Visibility != "private" && actualRepository.AllowForking != allowForking {
			changes = append(changes, FieldChange{Field: "allow_forking", From: actualRepository.AllowForking, To: allowForking})
		}
		if archived, managed := repository.ManagedArchived(); managed && actualRepository.Archived != archived {
			changes = append(changes, FieldChange{Field: "archived", From: actualRepository.Archived, To: archived})
		}
		if isTemplate, managed := repository.ManagedIsTemplate(); managed && actualRepository.IsTemplate != isTemplate {
			changes = append(changes, FieldChange{Field: "is_template", From: actualRepository.IsTemplate, To: isTemplate})
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

	for _, repository := range p.actual.Repositories {
		key := repositoryKey(repository.Owner, repository.Name)
		if _, ok := desiredRepos[key]; ok {
			continue
		}
		actions = append(actions, Action{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationDelete,
			ResourceID:   repositoryID(repository.Owner, repository.Name),
			Executable:   false,
			Message:      fmt.Sprintf("repository %s exists in live state but is not declared in desired config", repositoryID(repository.Owner, repository.Name)),
		})
	}

	return actions
}
