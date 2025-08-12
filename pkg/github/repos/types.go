package repos

import (
	"context"

	gh "github.com/google/go-github/v55/github"
)

// Service defines the subset of GitHub repository APIs used by CreateRepo.
type Service interface {
	// Repository-related functions

	// CreateFromTemplate creates a new repository from a template repository.
	CreateFromTemplate(ctx context.Context, templateOwner, templateRepo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error)
	// Delete deletes a repository.
	Delete(ctx context.Context, owner, repo string) (*gh.Response, error)
	// Edit updates a repository's settings.
	Edit(ctx context.Context, owner, repo string, repository *gh.Repository) (*gh.Repository, *gh.Response, error)

	// Topics-related functions

	// SetTopics sets the topics for a repository.
	ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *gh.Response, error)
	// ListAllTopics lists all topics for a repository.
	ListAllTopics(ctx context.Context, owner, repo string) ([]string, *gh.Response, error)
}
