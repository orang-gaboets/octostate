package repos

import (
	"context"
	"fmt"
	"log"
	"strings"

	gh "github.com/google/go-github/v55/github"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
)

// CreateFromTemplate creates a repository from a template and optionally sets topics.
func CreateFromTemplate(ctx context.Context, opts CreateFromTemplateOptions) (*gh.Repository, error) {
	if opts.Service == nil {
		return nil, github.ErrNilService
	}

	req := &gh.TemplateRepoRequest{
		Owner:              gh.String(opts.NewRepo.Org),
		Name:               gh.String(opts.NewRepo.Name),
		Description:        gh.String(opts.NewRepo.Description),
		Private:            gh.Bool(opts.NewRepo.Private),
		IncludeAllBranches: gh.Bool(opts.IncludeAllBranches),
	}

	log.Printf("Creating repository %s/%s from template %s/%s", opts.NewRepo.Org, opts.NewRepo.Name, opts.TemplateRepo.Org, opts.TemplateRepo.Name)

	newRepo, _, err := opts.Service.CreateFromTemplate(ctx, opts.TemplateRepo.Org, opts.TemplateRepo.Name, req)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to create repository from template %s/%s", opts.TemplateRepo.Org, opts.TemplateRepo.Name))
	}
	var newRepoURL string
	if newRepo != nil {
		newRepoURL = newRepo.GetHTMLURL()
	}
	if newRepoURL == "" {
		newRepoURL = "https://github.com/" + opts.NewRepo.Org + "/" + opts.NewRepo.Name
	}
	log.Printf("Repository successfully created at: %s", newRepoURL)

	listTemplateTopicsOptions := topics.ListAllTopicsOptions{
		Repo:    opts.TemplateRepo,
		Service: opts.Service,
	}
	templateTopics, err := topics.ListAllTopics(ctx, listTemplateTopicsOptions)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to list template topics for %s/%s", opts.TemplateRepo.Org, opts.TemplateRepo.Name))
	}

	cleanedSet := make(map[string]struct{})
	for _, t := range templateTopics {
		if v := strings.TrimSpace(t); v != "" {
			cleanedSet[v] = struct{}{}
		}
	}
	for _, t := range opts.NewRepo.Topics {
		if v := strings.TrimSpace(t); v != "" {
			cleanedSet[v] = struct{}{}
		}
	}

	cleaned := make([]string, 0, len(cleanedSet))
	for topic := range cleanedSet {
		cleaned = append(cleaned, topic)
	}

	if len(cleaned) > 0 {
		newRepoTopicsOptions := topics.ReplaceAllTopicsOptions{
			Repo:    opts.NewRepo,
			Service: opts.Service,
			Topics:  cleaned,
		}
		_, err := topics.ReplaceAllTopics(ctx, newRepoTopicsOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to set topics for new repository %s/%s", opts.NewRepo.Org, opts.NewRepo.Name))
		}
	}
	return newRepo, nil
}

// Delete removes a repository from GitHub.
func Delete(ctx context.Context, opts DeleteOptions) error {
	if opts.Service == nil {
		return github.ErrNilService
	}

	log.Printf("Deleting repository %s/%s", opts.Repository.Org, opts.Repository.Name)

	_, err := opts.Service.Delete(ctx, opts.Repository.Org, opts.Repository.Name)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to delete repository %s/%s", opts.Repository.Org, opts.Repository.Name))
	}

	log.Printf("Repository %s/%s successfully deleted", opts.Repository.Org, opts.Repository.Name)
	return nil
}

// Edit updates the properties of an existing repository.
func Edit(ctx context.Context, opts EditOptions) (*gh.Repository, error) {
	if opts.Service == nil {
		return nil, github.ErrNilService
	}

	repo := &gh.Repository{
		Description:  opts.Description,
		Homepage:     opts.Homepage,
		Private:      opts.Private,
		IsTemplate:   opts.IsTemplate,
		Archived:     opts.Archived,
		AllowForking: opts.AllowForking,
	}

	log.Printf("Editing repository %s/%s with options: %+v", opts.Repository.Org, opts.Repository.Name, repo)

	updatedRepo, _, err := opts.Service.Edit(ctx, opts.Repository.Org, opts.Repository.Name, repo)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to edit repository %s/%s", opts.Repository.Org, opts.Repository.Name))
	}
	var updatedRepoURL string
	if updatedRepo != nil {
		updatedRepoURL = updatedRepo.GetHTMLURL()
	}
	if updatedRepoURL == "" {
		updatedRepoURL = "https://github.com/" + opts.Repository.Org + "/" + opts.Repository.Name
	}

	log.Printf("Repository successfully edited: %s", updatedRepoURL)
	return updatedRepo, nil
}
