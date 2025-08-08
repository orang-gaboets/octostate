package repos

import (
	"context"

	"github.com/google/go-github/v55/github"
)

// Service defines the subset of GitHub repository APIs used by CreateRepo.
type Service interface {
	// Repository-related functions

	// CreateFromTemplate creates a new repository from a template repository.
	CreateFromTemplate(ctx context.Context, templateOwner, templateRepo string, req *github.TemplateRepoRequest) (*github.Repository, *github.Response, error)
	// Edit updates a repository's settings.
	Edit(ctx context.Context, owner, repo string, repository *github.Repository) (*github.Repository, *github.Response, error)

	// Topics-related functions

	// SetTopics sets the topics for a repository.
	ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *github.Response, error)
	// ListAllTopics lists all topics for a repository.
	ListAllTopics(ctx context.Context, owner, repo string) ([]string, *github.Response, error)
}
