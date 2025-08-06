package repos

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v55/github"

	"github.com/orang-gaboets/repo-builder/pkg/github/topics"
)

// CreateRepo creates a repository from a template and optionally sets topics.
func CreateRepo(ctx context.Context, opts RepoCreationOptions) (*github.Repository, error) {
	if opts.Service == nil {
		return nil, fmt.Errorf("repository service is not provided")
	}

	req := &github.TemplateRepoRequest{
		Owner:       github.String(opts.NewRepo.Org),
		Name:        github.String(opts.NewRepo.Name),
		Description: github.String(opts.NewRepo.Description),
		Private:     github.Bool(opts.NewRepo.Private),
	}

	log.Printf("Creating repository %s/%s from template %s/%s", opts.NewRepo.Org, opts.NewRepo.Name, opts.TemplateRepo.Org, opts.TemplateRepo.Name)

	newRepo, _, err := opts.Service.CreateFromTemplate(ctx, opts.TemplateRepo.Org, opts.TemplateRepo.Name, req)
	if err != nil {
		return nil, fmt.Errorf("failed to create repository from template: %w", err)
	}

	newRepoURL := newRepo.GetHTMLURL()
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
		return nil, err
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
		topics, err := topics.ReplaceAllTopics(ctx, newRepoTopicsOptions)
		if err != nil {
			return nil, err
		}
		log.Printf("Topics successfully set for repository %s/%s: %s", opts.NewRepo.Org, opts.NewRepo.Name, strings.Join(topics, ", "))
	}
	return newRepo, nil
}
