package apply

import (
	"fmt"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	"github.com/orang-gaboets/octostate/pkg/github/topics"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
)

func (e *executor) executeRepositoryAction(action gitopsplan.Action) error {
	switch action.Operation {
	case gitopsplan.ActionOperationCreate:
		return e.createRepository(action)
	case gitopsplan.ActionOperationUpdate:
		return e.updateRepository(action)
	default:
		return fmt.Errorf("unsupported repository operation %q for %s: %w", action.Operation, action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}
}

func (e *executor) createRepository(action gitopsplan.Action) error {
	repository, ok := e.desiredRepositories[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired repository %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
	}
	visibility, err := repositoryVisibility(repository.Visibility)
	if err != nil {
		return fmt.Errorf("create repository %s: %w", action.ResourceID, err)
	}

	var description *string
	if value, managed := repository.ManagedDescription(); managed {
		description = githubpkg.Ptr(value)
	}

	if repository.Template.Owner != "" || repository.Template.Name != "" {
		if visibility == "internal" {
			return fmt.Errorf("create repository %s: internal visibility is unsupported for template-based creation: %w", action.ResourceID, githubpkg.ErrInvalidFieldValue)
		}
		if repository.Template.Owner == "" || repository.Template.Name == "" {
			return fmt.Errorf("repository %s has incomplete template configuration: %w", action.ResourceID, githubpkg.ErrInvalidFieldValue)
		}
		_, err = repos.CreateFromTemplate(e.ctx, repos.CreateFromTemplateOptions{
			Service:            e.repositoryService,
			Name:               repository.Name,
			Owner:              repository.Owner,
			TemplateOwner:      repository.Template.Owner,
			TemplateRepo:       repository.Template.Name,
			Description:        description,
			Private:            githubpkg.Ptr(visibility == "private"),
			SkipTopicSync:      true,
			IncludeAllBranches: repository.Template.IncludeAllBranches,
		})
	} else {
		_, err = repos.Create(e.ctx, repos.CreateOptions{
			Service:     e.repositoryService,
			Name:        repository.Name,
			Owner:       repository.Owner,
			Description: description,
			Visibility:  githubpkg.Ptr(visibility),
			Private:     githubpkg.Ptr(visibility == "private"),
		})
	}
	if err != nil {
		return err
	}

	if err := e.applyExactRepositorySettings(repository); err != nil {
		return err
	}
	return e.replaceRepositoryTopics(repository)
}

func (e *executor) updateRepository(action gitopsplan.Action) error {
	repository, ok := e.desiredRepositories[action.ResourceID]
	if !ok {
		return fmt.Errorf("desired repository %s not found: %w", action.ResourceID, githubpkg.ErrNotFound)
	}

	editOptions := repos.EditOptions{
		Service: e.repositoryService,
		Owner:   repository.Owner,
		Repo:    repository.Name,
	}
	editNeeded := false
	topicsChanged := false

	for _, change := range action.Changes {
		switch change.Field {
		case "visibility":
			visibility, err := repositoryVisibility(repository.Visibility)
			if err != nil {
				return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
			}
			editOptions.Visibility = githubpkg.Ptr(visibility)
			editOptions.Private = githubpkg.Ptr(visibility == "private")
			editNeeded = true
		case "description":
			editOptions.Description = githubpkg.Ptr(repository.Description)
			editNeeded = true
		case "homepage":
			editOptions.Homepage = githubpkg.Ptr(repository.Homepage)
			editNeeded = true
		case "topics":
			topicsChanged = true
		case "allow_forking":
			if _, err := repositoryVisibility(repository.Visibility); err != nil {
				return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
			}
			if !config.IsPrivateVisibility(repository.Visibility) {
				break
			}
			editOptions.AllowForking = githubpkg.Ptr(repository.AllowForking)
			editNeeded = true
		case "archived":
			editOptions.Archived = githubpkg.Ptr(repository.Archived)
			editNeeded = true
		case "is_template":
			editOptions.IsTemplate = githubpkg.Ptr(repository.IsTemplate)
			editNeeded = true
		default:
			return fmt.Errorf("unsupported repository change field %q for %s: %w", change.Field, action.ResourceID, githubpkg.ErrInvalidFieldValue)
		}
	}

	if editNeeded {
		if _, err := repos.Edit(e.ctx, editOptions); err != nil {
			return err
		}
	}
	if topicsChanged {
		return e.replaceRepositoryTopics(repository)
	}
	return nil
}

func (e *executor) applyExactRepositorySettings(repository config.RepositorySpec) error {
	visibility, err := repositoryVisibility(repository.Visibility)
	if err != nil {
		return err
	}

	editOptions := repos.EditOptions{
		Service:    e.repositoryService,
		Owner:      repository.Owner,
		Repo:       repository.Name,
		Visibility: githubpkg.Ptr(visibility),
		Private:    githubpkg.Ptr(visibility == "private"),
	}
	if description, managed := repository.ManagedDescription(); managed {
		editOptions.Description = githubpkg.Ptr(description)
	}
	if homepage, managed := repository.ManagedHomepage(); managed {
		editOptions.Homepage = githubpkg.Ptr(homepage)
	}
	if archived, managed := repository.ManagedArchived(); managed {
		editOptions.Archived = githubpkg.Ptr(archived)
	}
	if isTemplate, managed := repository.ManagedIsTemplate(); managed {
		editOptions.IsTemplate = githubpkg.Ptr(isTemplate)
	}
	if config.IsPrivateVisibility(repository.Visibility) {
		if allowForking, managed := repository.ManagedAllowForking(); managed {
			editOptions.AllowForking = githubpkg.Ptr(allowForking)
		}
	}

	_, err = repos.Edit(e.ctx, editOptions)
	return err
}

func (e *executor) replaceRepositoryTopics(repository config.RepositorySpec) error {
	_, err := topics.ReplaceAllTopics(e.ctx, topics.ReplaceAllTopicsOptions{
		Service: e.repositoryService,
		Owner:   repository.Owner,
		Repo:    repository.Name,
		Topics:  repository.Topics,
	})
	return err
}
