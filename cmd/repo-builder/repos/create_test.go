package repos_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	reposcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/repos"
)

// mockRepoCreateService implements repos.Service for testing.
type mockRepoCreateService struct{}

func (mockRepoCreateService) CreateFromTemplate(_ context.Context, _, _ string, _ *github.TemplateRepoRequest) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}

func (mockRepoCreateService) Edit(_ context.Context, _, _ string, _ *github.Repository) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}

func (mockRepoCreateService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func (mockRepoCreateService) ListAllTopics(_ context.Context, _, _ string) ([]string, *github.Response, error) {
	return []string{}, nil, nil
}

func TestCreateRepoFromTemplateRequiredFlags(t *testing.T) {
	c := reposcmd.CreateNewRepoFromTemplateCmd(mockRepoCreateService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestCreateRepoFromTemplateAllRequiredFlagsProvided(t *testing.T) {
	c := reposcmd.CreateNewRepoFromTemplateCmd(mockRepoCreateService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepoFromTemplateWithInvalidFlags(t *testing.T) {
	c := reposcmd.CreateNewRepoFromTemplateCmd(mockRepoCreateService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}
