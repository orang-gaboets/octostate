package plan

import (
	"fmt"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func repositoryCreateAction(repository config.RepositorySpec) *Action {
	return &Action{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationCreate, ResourceID: repositoryID(repository.Owner, repository.Name), Message: fmt.Sprintf("create repository %s", repositoryID(repository.Owner, repository.Name))}
}

func repositoryUpdateAction(repository config.RepositorySpec, actual state.Repository) *Action {
	changes := make([]FieldChange, 0, 7)
	if actual.Visibility != repository.Visibility {
		changes = append(changes, FieldChange{Field: "visibility", From: actual.Visibility, To: repository.Visibility})
	}
	if description, managed := repository.ManagedDescription(); managed && actual.Description != description {
		changes = append(changes, FieldChange{Field: "description", From: actual.Description, To: description})
	}
	if homepage, managed := repository.ManagedHomepage(); managed && actual.Homepage != homepage {
		changes = append(changes, FieldChange{Field: "homepage", From: actual.Homepage, To: homepage})
	}
	if !equalStringSets(actual.Topics, repository.Topics) {
		changes = append(changes, FieldChange{Field: "topics", From: sortedStrings(actual.Topics), To: sortedStrings(repository.Topics)})
	}
	if allowForking, managed := repository.ManagedAllowForking(); managed && config.SupportsAllowForking(repository.Visibility) && actual.AllowForking != allowForking {
		changes = append(changes, FieldChange{Field: "allow_forking", From: actual.AllowForking, To: allowForking})
	}
	if archived, managed := repository.ManagedArchived(); managed && actual.Archived != archived {
		changes = append(changes, FieldChange{Field: "archived", From: actual.Archived, To: archived})
	}
	if isTemplate, managed := repository.ManagedIsTemplate(); managed && actual.IsTemplate != isTemplate {
		changes = append(changes, FieldChange{Field: "is_template", From: actual.IsTemplate, To: isTemplate})
	}
	if len(changes) == 0 {
		return nil
	}
	return &Action{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationUpdate, ResourceID: repositoryID(repository.Owner, repository.Name), Executable: true, Message: fmt.Sprintf("update repository %s", repositoryID(repository.Owner, repository.Name)), Changes: changes}
}
