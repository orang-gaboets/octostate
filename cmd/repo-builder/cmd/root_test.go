package cmd_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	rootcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/cmd"
)

// mockService implements builder.RepoService for testing.
type mockService struct{}

func (mockService) CreateFromTemplate(ctx context.Context, owner, repo string, req *github.TemplateRepoRequest) (*github.Repository, *github.Response, error) {
	return &github.Repository{}, nil, nil
}
func (mockService) ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func (mockService) ListAllTopics(ctx context.Context, owner, repo string) ([]string, *github.Response, error) {
	return []string{}, nil, nil
}

func TestRequiredFlags(t *testing.T) {
	c := rootcmd.NewRootCmd(mockService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestAllRequiredFlagsProvided(t *testing.T) {
	c := rootcmd.NewRootCmd(mockService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
