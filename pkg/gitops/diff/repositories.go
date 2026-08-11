package diff

import (
	"fmt"
	"slices"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

type repositoryDiffNode struct {
	repository config.RepositorySpec
	action     *Action
	dependency string
}

func (b builder) planRepositories() []Action {
	actual := make(map[string]state.Repository, len(b.actual.Repositories))
	for _, repository := range b.actual.Repositories {
		actual[repositoryKey(repository.Owner, repository.Name)] = repository
	}

	nodes := make(map[string]*repositoryDiffNode, len(b.desired.Repositories))
	keys := make([]string, 0, len(b.desired.Repositories))
	for _, repository := range b.desired.Repositories {
		key := repositoryKey(repository.Owner, repository.Name)
		if _, exists := nodes[key]; !exists {
			keys = append(keys, key)
		}
		node := &repositoryDiffNode{repository: repository}
		if live, ok := actual[key]; ok {
			node.action = repositoryDiffUpdateAction(repository, live)
		} else {
			node.action = repositoryDiffCreateAction(repository)
		}
		nodes[key] = node
	}
	slices.SortFunc(keys, compareStrings)

	organization := strings.TrimSpace(b.desired.Organization)
	for _, key := range keys {
		node := nodes[key]
		if _, exists := actual[key]; exists {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(node.repository.Owner), organization) || !strings.EqualFold(strings.TrimSpace(node.repository.Template.Owner), organization) {
			continue
		}
		dependency := repositoryKey(node.repository.Template.Owner, node.repository.Template.Name)
		if _, ok := nodes[dependency]; ok {
			node.dependency = dependency
		}
	}

	actions := make([]Action, 0, len(nodes)+len(actual))
	emitted := make(map[string]struct{}, len(nodes))
	visiting := make(map[string]bool, len(nodes))
	var emit func(string)
	emit = func(key string) {
		if _, ok := emitted[key]; ok || visiting[key] {
			return
		}
		visiting[key] = true
		node := nodes[key]
		if node.dependency != "" {
			emit(node.dependency)
		}
		visiting[key] = false
		emitted[key] = struct{}{}
		if node.action != nil {
			actions = append(actions, *node.action)
		}
	}
	for _, key := range keys {
		emit(key)
	}

	orphans := make([]state.Repository, 0)
	for key, repository := range actual {
		if _, ok := nodes[key]; !ok {
			orphans = append(orphans, repository)
		}
	}
	slices.SortFunc(orphans, func(a, b state.Repository) int {
		return compareStrings(repositoryKey(a.Owner, a.Name), repositoryKey(b.Owner, b.Name))
	})
	for _, repository := range orphans {
		actions = append(actions, Action{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationDelete, ResourceID: repositoryID(repository.Owner, repository.Name), Message: fmt.Sprintf("repository %s exists in snapshot state but is not declared in desired config", repositoryID(repository.Owner, repository.Name))})
	}
	return actions
}

func repositoryDiffCreateAction(repository config.RepositorySpec) *Action {
	executable := strings.TrimSpace(repository.Template.Owner) != "" && strings.TrimSpace(repository.Template.Name) != ""
	message := fmt.Sprintf("create repository %s", repositoryID(repository.Owner, repository.Name))
	if !executable {
		message = fmt.Sprintf("repository %s cannot be created because template configuration is missing", repositoryID(repository.Owner, repository.Name))
	}
	return &Action{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationCreate, ResourceID: repositoryID(repository.Owner, repository.Name), Executable: executable, Message: message}
}

func repositoryDiffUpdateAction(repository config.RepositorySpec, actual state.Repository) *Action {
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
	if allowForking, managed := repository.ManagedAllowForking(); managed && repository.Visibility != "private" && actual.AllowForking != allowForking {
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
