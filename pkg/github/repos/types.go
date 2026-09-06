package repos

import (
	"context"

	gh "github.com/google/go-github/v88/github"
)

// RepoType represents the repository type filter for organization listings.
type RepoType string

const (
	// RepoTypeAll indicates all repositories with no filter.
	RepoTypeAll RepoType = "all"
	// RepoTypePublic indicates public repositories.
	RepoTypePublic RepoType = "public"
	// RepoTypePrivate indicates private repositories.
	RepoTypePrivate RepoType = "private"
	// RepoTypeForks indicates forked repositories.
	RepoTypeForks RepoType = "forks"
	// RepoTypeSources indicates source repositories.
	RepoTypeSources RepoType = "sources"
	// RepoTypeMember indicates member repositories.
	RepoTypeMember RepoType = "member"
)

// IsValid reports whether the repo type is one of the allowed values or empty.
func (r RepoType) IsValid() bool {
	switch r {
	case "", RepoTypeAll, RepoTypePublic, RepoTypePrivate, RepoTypeForks, RepoTypeSources, RepoTypeMember:
		return true
	default:
		return false
	}
}

// String returns the string representation of the repo type.
func (r RepoType) String() string {
	return string(r)
}

// Service defines the subset of GitHub repository APIs used by CreateRepo.
type Service interface {
	// Repository-related functions

	// CreateFromTemplate creates a new repository from a template repository.
	CreateFromTemplate(ctx context.Context, templateOwner, templateRepo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error)
	// Create creates a new repository in an organization.
	Create(ctx context.Context, org string, repo *gh.Repository) (*gh.Repository, *gh.Response, error)
	// Delete deletes a repository.
	Delete(ctx context.Context, owner, repo string) (*gh.Response, error)
	// Edit updates a repository's settings.
	Edit(ctx context.Context, owner, repo string, repository *gh.Repository) (*gh.Repository, *gh.Response, error)
	// Get retrieves a repository.
	Get(ctx context.Context, owner, repo string) (*gh.Repository, *gh.Response, error)
	// ListByOrg lists repositories by organization.
	ListByOrg(ctx context.Context, org string, opts *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error)

	// Topics-related functions

	// SetTopics sets the topics for a repository.
	ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *gh.Response, error)
	// ListAllTopics lists all topics for a repository.
	ListAllTopics(ctx context.Context, owner, repo string) ([]string, *gh.Response, error)
}
