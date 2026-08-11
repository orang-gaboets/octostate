package plan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

type repositoryPlan struct {
	actions      []Action
	availability map[string]repositoryAvailability
}

type repositoryAvailability struct {
	executable       bool
	usableAsTemplate bool
	diagnostic       string
}

type repositoryPlanNode struct {
	repository config.RepositorySpec
	actual     *state.Repository
	action     *Action
	dependency string
}

func (p planner) computeRepositoryPlan() repositoryPlan {
	actual := make(map[string]state.Repository, len(p.actual.Repositories))
	for _, repository := range p.actual.Repositories {
		actual[repositoryKey(repository.Owner, repository.Name)] = repository
	}

	nodes := make(map[string]*repositoryPlanNode, len(p.desired.Repositories))
	keys := make([]string, 0, len(p.desired.Repositories))
	for _, repository := range p.desired.Repositories {
		key := repositoryKey(repository.Owner, repository.Name)
		if _, exists := nodes[key]; !exists {
			keys = append(keys, key)
		}
		node := &repositoryPlanNode{repository: repository}
		if live, ok := actual[key]; ok {
			node.actual = &live
			node.action = repositoryUpdateAction(repository, live)
		} else {
			node.action = repositoryCreateAction(repository)
		}
		nodes[key] = node
	}
	slices.SortFunc(keys, compareStrings)

	organization := strings.TrimSpace(p.desired.Organization)
	for _, key := range keys {
		node := nodes[key]
		if node.actual != nil || !strings.EqualFold(strings.TrimSpace(node.repository.Owner), organization) || strings.TrimSpace(node.repository.Template.Owner) == "" || strings.TrimSpace(node.repository.Template.Name) == "" || !strings.EqualFold(strings.TrimSpace(node.repository.Template.Owner), organization) {
			continue
		}
		dependency := repositoryKey(node.repository.Template.Owner, node.repository.Template.Name)
		if _, ok := nodes[dependency]; ok {
			node.dependency = dependency
		}
	}

	availability := make(map[string]repositoryAvailability, len(nodes))
	colors := make(map[string]uint8, len(nodes))
	stack := make([]string, 0, len(nodes))
	var visit func(string)
	visit = func(key string) {
		switch colors[key] {
		case 2:
			return
		case 1:
			start := slices.Index(stack, key)
			cycle := append(append([]string{}, stack[start:]...), key)
			diagnostic := fmt.Sprintf("template dependency cycle: %s", strings.Join(cycle, " -> "))
			for _, cycleKey := range stack[start:] {
				availability[cycleKey] = repositoryAvailability{diagnostic: diagnostic}
			}
			return
		}

		colors[key] = 1
		stack = append(stack, key)
		node := nodes[key]
		if node.dependency != "" {
			visit(node.dependency)
		}
		if _, set := availability[key]; !set {
			availability[key] = repositoryNodeAvailability(node, availability[node.dependency])
		}
		colors[key] = 2
		stack = stack[:len(stack)-1]
	}
	for _, key := range keys {
		visit(key)
	}

	actions := make([]Action, 0, len(nodes)+len(actual))
	emitted := make(map[string]struct{}, len(nodes))
	var emit func(string)
	emit = func(key string) {
		if _, ok := emitted[key]; ok {
			return
		}
		emitted[key] = struct{}{}
		node := nodes[key]
		if node.dependency != "" {
			emit(node.dependency)
		}
		if node.action == nil {
			return
		}
		action := *node.action
		if action.Operation == ActionOperationCreate {
			status := availability[key]
			action.Executable = status.executable
			if !status.executable {
				action.Message = repositoryUnavailableMessage(node.repository, status.diagnostic)
			}
		}
		actions = append(actions, action)
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
		actions = append(actions, Action{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationDelete, ResourceID: repositoryID(repository.Owner, repository.Name), Message: fmt.Sprintf("repository %s exists in live state but is not declared in desired config", repositoryID(repository.Owner, repository.Name))})
	}

	return repositoryPlan{actions: actions, availability: availability}
}

func repositoryNodeAvailability(node *repositoryPlanNode, dependency repositoryAvailability) repositoryAvailability {
	if node.actual != nil {
		usable := node.actual.IsTemplate
		if isTemplate, managed := node.repository.ManagedIsTemplate(); managed {
			usable = isTemplate
		}
		availability := repositoryAvailability{executable: true, usableAsTemplate: usable}
		if !usable {
			availability.diagnostic = fmt.Sprintf("repository %s is not a template", repositoryID(node.repository.Owner, node.repository.Name))
		}
		return availability
	}
	if strings.TrimSpace(node.repository.Template.Owner) == "" || strings.TrimSpace(node.repository.Template.Name) == "" {
		return repositoryAvailability{diagnostic: "template configuration is missing"}
	}
	if node.dependency != "" && (!dependency.executable || !dependency.usableAsTemplate) {
		return repositoryAvailability{diagnostic: fmt.Sprintf("required template %s is unavailable: %s", node.dependency, dependency.diagnostic)}
	}
	isTemplate, managed := node.repository.ManagedIsTemplate()
	availability := repositoryAvailability{executable: true, usableAsTemplate: managed && isTemplate}
	if !availability.usableAsTemplate {
		availability.diagnostic = fmt.Sprintf("repository %s will not be a template", repositoryID(node.repository.Owner, node.repository.Name))
	}
	return availability
}

func repositoryUnavailableMessage(repository config.RepositorySpec, diagnostic string) string {
	if diagnostic == "template configuration is missing" {
		return fmt.Sprintf("repository %s cannot be created because template configuration is missing", repositoryID(repository.Owner, repository.Name))
	}
	return fmt.Sprintf("repository %s cannot be created because %s", repositoryID(repository.Owner, repository.Name), diagnostic)
}
