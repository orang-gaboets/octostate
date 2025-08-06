package builder

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/go-github/v55/github"
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

	templateTopics, _, err := opts.Service.ListAllTopics(ctx, opts.TemplateRepo.Org, opts.TemplateRepo.Name)
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
		// Clean empty topics resulting from consecutive commas
		log.Printf("Setting topics for repository %s/%s", opts.NewRepo.Org, opts.NewRepo.Name)
		topics, _, err := opts.Service.ReplaceAllTopics(ctx, opts.NewRepo.Org, opts.NewRepo.Name, cleaned)
		if err != nil {
			return nil, err
		}
		log.Printf("Topics successfully set for repository %s/%s: %s", opts.NewRepo.Org, opts.NewRepo.Name, strings.Join(topics, ", "))
	}
	return newRepo, nil
}

func ListAllTopics(ctx context.Context, svc RepoService, repository Repository) ([]string, error) {
	if svc == nil {
		return nil, fmt.Errorf("repo service is nil")
	}

	log.Printf("Listing topics for repository %s/%s", repository.Org, repository.Name)
	topics, _, err := svc.ListAllTopics(ctx, repository.Org, repository.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}
	return topics, nil
}

func ReplaceAllTopics(ctx context.Context, svc RepoService, repository Repository, topics []string) ([]string, error) {
	if svc == nil {
		return nil, fmt.Errorf("repo service is nil")
	}

	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics to set")
	}

	cleaned := make([]string, 0, len(topics))
	for _, t := range topics {
		if v := strings.TrimSpace(t); v != "" {
			cleaned = append(cleaned, v)
		}
	}

	log.Printf("Setting topics for repository %s/%s: %v", repository.Org, repository.Name, cleaned)
	topics, _, err := svc.ReplaceAllTopics(ctx, repository.Org, repository.Name, cleaned)
	if err != nil {
		return nil, fmt.Errorf("replace topics: %w", err)
	}
	return topics, nil
}

func AddTopics(ctx context.Context, svc RepoService, repository Repository, topics []string) ([]string, error) {
	if svc == nil {
		return nil, fmt.Errorf("repo service is nil")
	}

	if len(repository.Topics) == 0 {
		return nil, fmt.Errorf("no topics to add")
	}

	oldTopics, _, err := svc.ListAllTopics(ctx, repository.Org, repository.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to list existing topics: %w", err)
	}

	log.Printf("Current topics for repository %s/%s: %v", repository.Org, repository.Name, oldTopics)

	cleanedSet := make(map[string]struct{})
	for _, t := range oldTopics {
		if v := strings.TrimSpace(t); v != "" {
			cleanedSet[v] = struct{}{}
		}
	}
	for _, t := range topics {
		if v := strings.TrimSpace(t); v != "" {
			cleanedSet[v] = struct{}{}
		}
	}

	cleaned := make([]string, 0, len(cleanedSet))
	for topic := range cleanedSet {
		cleaned = append(cleaned, topic)
	}

	log.Printf("Adding topics to repository %s/%s: %v", repository.Org, repository.Name, cleaned)
	topics, _, err = svc.ReplaceAllTopics(ctx, repository.Org, repository.Name, cleaned)
	if err != nil {
		return nil, fmt.Errorf("failed to add topics: %w", err)
	}
	return topics, nil
}
