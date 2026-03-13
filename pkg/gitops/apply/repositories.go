package apply

import (
	"fmt"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
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
	if repository.Template.Owner == "" || repository.Template.Name == "" {
		return fmt.Errorf("repository %s cannot be created without a template: %w", action.ResourceID, githubpkg.ErrInvalidFieldValue)
	}

	private, err := visibilityPrivateFlag(repository.Visibility)
	if err != nil {
		return fmt.Errorf("create repository %s: %w", action.ResourceID, err)
	}

	_, err = repos.CreateFromTemplate(e.ctx, repos.CreateFromTemplateOptions{
		Service:            e.repositoryService,
		Name:               repository.Name,
		Owner:              repository.Owner,
		TemplateOwner:      repository.Template.Owner,
		TemplateRepo:       repository.Template.Name,
		Description:        githubpkg.Ptr(repository.Description),
		Private:            githubpkg.Ptr(private),
		Topics:             repository.Topics,
		IncludeAllBranches: repository.Template.IncludeAllBranches,
	})
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
			private, err := visibilityPrivateFlag(repository.Visibility)
			if err != nil {
				return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
			}
			editOptions.Private = githubpkg.Ptr(private)
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
			private, err := visibilityPrivateFlag(repository.Visibility)
			if err != nil {
				return fmt.Errorf("update repository %s: %w", action.ResourceID, err)
			}
			if !private {
				editOptions.AllowForking = githubpkg.Ptr(repository.AllowForking)
				editNeeded = true
			}
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
	private, err := visibilityPrivateFlag(repository.Visibility)
	if err != nil {
		return err
	}

	editOptions := repos.EditOptions{
		Service:     e.repositoryService,
		Owner:       repository.Owner,
		Repo:        repository.Name,
		Description: githubpkg.Ptr(repository.Description),
		Homepage:    githubpkg.Ptr(repository.Homepage),
		Private:     githubpkg.Ptr(private),
		IsTemplate:  githubpkg.Ptr(repository.IsTemplate),
		Archived:    githubpkg.Ptr(repository.Archived),
	}
	if !private {
		editOptions.AllowForking = githubpkg.Ptr(repository.AllowForking)
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
