package builder

import (
	"context"

	"github.com/google/go-github/v55/github"
)

// RepoService defines the subset of GitHub repository APIs used by CreateRepo.
type RepoService interface {
	CreateFromTemplate(ctx context.Context, templateOwner, templateRepo string, req *github.TemplateRepoRequest) (*github.Repository, *github.Response, error)
	ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *github.Response, error)
}
