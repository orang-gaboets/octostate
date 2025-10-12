package repos

import (
	"context"
	"fmt"
	"log"

	gh "github.com/google/go-github/v55/github"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
)

// CreateFromTemplate creates a repository from a template and optionally sets topics.
func CreateFromTemplate(ctx context.Context, option CreateFromTemplateOptions) (*gh.Repository, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	req := &gh.TemplateRepoRequest{
		Owner:              &option.Owner,
		Name:               &option.Name,
		Description:        option.Description,
		Private:            option.Private,
		IncludeAllBranches: &option.IncludeAllBranches,
	}

	log.Printf("Creating repository %s/%s from template %s/%s", option.Owner, option.Name, option.TemplateOwner, option.TemplateRepo)

	newRepo, _, err := option.Service.CreateFromTemplate(ctx, option.TemplateOwner, option.TemplateRepo, req)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to create repository from template %s/%s", option.TemplateOwner, option.TemplateRepo))
	}
	var newRepoURL string
	if newRepo != nil {
		newRepoURL = newRepo.GetHTMLURL()
	}
	if newRepoURL == "" {
		newRepoURL = "https://github.com/" + option.Owner + "/" + option.Name
	}
	log.Printf("Repository successfully created at: %s", newRepoURL)

	listTemplateTopicsOptions := topics.ListAllTopicsOptions{
		Owner:   option.TemplateOwner,
		Repo:    option.TemplateRepo,
		Service: option.Service,
	}
	templateTopics, err := topics.ListAllTopics(ctx, listTemplateTopicsOptions)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to list template topics for %s/%s", option.TemplateOwner, option.TemplateRepo))
	}

	uniqueTopics := github.MergeUnique(option.Topics, templateTopics)

	if len(uniqueTopics) > 0 {
		newRepoTopicsOptions := topics.ReplaceAllTopicsOptions{
			Owner:   option.Owner,
			Repo:    option.Name,
			Service: option.Service,
			Topics:  uniqueTopics,
		}
		_, err := topics.ReplaceAllTopics(ctx, newRepoTopicsOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to set topics for new repository %s/%s", option.Owner, option.Name))
		}
	}
	return newRepo, nil
}

// Delete removes a repository from GitHub.
func Delete(ctx context.Context, option DeleteOptions) error {
	if err := option.Validate(); err != nil {
		return err
	}

	log.Printf("Deleting repository %s/%s", option.Owner, option.Repo)

	_, err := option.Service.Delete(ctx, option.Owner, option.Repo)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to delete repository %s/%s", option.Owner, option.Repo))
	}

	log.Printf("Repository %s/%s successfully deleted", option.Owner, option.Repo)
	return nil
}

// Edit updates the properties of an existing repository.
func Edit(ctx context.Context, option EditOptions) (*gh.Repository, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	repo := &gh.Repository{
		Description:  option.Description,
		Homepage:     option.Homepage,
		Private:      option.Private,
		IsTemplate:   option.IsTemplate,
		Archived:     option.Archived,
		AllowForking: option.AllowForking,
	}

	log.Printf("Editing repository %s/%s with options: %+v", option.Owner, option.Repo, repo)

	updatedRepo, _, err := option.Service.Edit(ctx, option.Owner, option.Repo, repo)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to edit repository %s/%s", option.Owner, option.Repo))
	}
	var updatedRepoURL string
	if updatedRepo != nil {
		updatedRepoURL = updatedRepo.GetHTMLURL()
	}
	if updatedRepoURL == "" {
		updatedRepoURL = "https://github.com/" + option.Owner + "/" + option.Repo
	}

	log.Printf("Repository successfully edited: %s", updatedRepoURL)
	return updatedRepo, nil
}
