package cmd_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	rootcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/cmd"
)

// mockRepoCreateService implements repos.Service for testing.
type mockRepoCreateService struct{}

func (mockRepoCreateService) CreateFromTemplate(ctx context.Context, owner, repo string, req *github.TemplateRepoRequest) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}

func (mockRepoCreateService) Edit(ctx context.Context, owner, repo string, repository *github.Repository) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}

func (mockRepoCreateService) ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func (mockRepoCreateService) ListAllTopics(ctx context.Context, owner, repo string) ([]string, *github.Response, error) {
	return []string{}, nil, nil
}

func TestCreateRepoFromTemplateRequiredFlags(t *testing.T) {
	c := rootcmd.CreateNewRepoFromTemplateCmd(mockRepoCreateService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestCreateRepoFromTemplateAllRequiredFlagsProvided(t *testing.T) {
	c := rootcmd.CreateNewRepoFromTemplateCmd(mockRepoCreateService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepoFromTemplateWithInvalidFlags(t *testing.T) {
	c := rootcmd.CreateNewRepoFromTemplateCmd(mockRepoCreateService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}
