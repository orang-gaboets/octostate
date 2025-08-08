package repo_test

import (
	"context"
	"testing"

	reposcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"

	"github.com/google/go-github/v55/github"
)

// mockRepoEditService implements repos.Service for testing.
type mockRepoEditService struct{}

func (mockRepoEditService) CreateFromTemplate(_ context.Context, _, _ string, _ *github.TemplateRepoRequest) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}

func (mockRepoEditService) Edit(_ context.Context, _, _ string, _ *github.Repository) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}

func (mockRepoEditService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func (mockRepoEditService) ListAllTopics(_ context.Context, _, _ string) ([]string, *github.Response, error) {
	return []string{}, nil, nil
}

func TestEditRepoNoRequiredFlags(t *testing.T) {
	c := reposcmd.EditRepo(mockRepoEditService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestEditRepoAllRequiredFlagsProvided(t *testing.T) {
	c := reposcmd.EditRepo(mockRepoEditService{})
	c.SetArgs([]string{
		"--token", "t",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditRepoWithOptionalFlags(t *testing.T) {
	c := reposcmd.EditRepo(mockRepoEditService{})
	c.SetArgs([]string{
		"--token", "t",
		"--org", "o",
		"--name", "n",
		"--desc", "New description",
		"--homepage", "https://example.com",
		"--private=true",
		"--is-template=false",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditRepoWithInvalidFlags(t *testing.T) {
	c := reposcmd.EditRepo(mockRepoEditService{})
	c.SetArgs([]string{
		"--token", "t",
		"--org", "o",
		"--name", "n",
		"--private=invalid", // Invalid boolean value
	})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag value")
	}
}
