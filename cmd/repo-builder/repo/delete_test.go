package repo_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	reposcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"
)

// mockRepoDeleteService implements repos.Service for testing.
type mockRepoDeleteService struct{}

func (mockRepoDeleteService) CreateFromTemplate(_ context.Context, _, _ string, _ *github.TemplateRepoRequest) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}

func (mockRepoDeleteService) Delete(_ context.Context, _, _ string) (*github.Response, error) {
	return nil, nil
}

func (mockRepoDeleteService) Edit(_ context.Context, _, _ string, _ *github.Repository) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}

func (mockRepoDeleteService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func (mockRepoDeleteService) ListAllTopics(_ context.Context, _, _ string) ([]string, *github.Response, error) {
	return []string{}, nil, nil
}

func TestDeleteRepoNoRequiredFlags(t *testing.T) {
	c := reposcmd.DeleteRepoCmd(mockRepoDeleteService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestDeleteRepoAllRequiredFlagsProvided(t *testing.T) {
	c := reposcmd.DeleteRepoCmd(mockRepoDeleteService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
