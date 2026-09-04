package repo_test

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	reposcmd "github.com/orang-gaboets/octostate/cmd/octostate/repo"
)

var errRepoCommandDependency = errors.New("repo command dependency failed")

type failingRepoService struct {
	auth.MockRepoService
}

func (failingRepoService) CreateFromTemplate(context.Context, string, string, *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	return nil, nil, errRepoCommandDependency
}

func (failingRepoService) Create(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
	return nil, nil, errRepoCommandDependency
}

func (failingRepoService) Get(context.Context, string, string) (*gh.Repository, *gh.Response, error) {
	return nil, nil, errRepoCommandDependency
}

func (failingRepoService) Edit(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
	return nil, nil, errRepoCommandDependency
}

func (failingRepoService) Delete(context.Context, string, string) (*gh.Response, error) {
	return nil, errRepoCommandDependency
}

func TestCreateRepoFromTemplatePropagatesServiceError(t *testing.T) {
	cmd := reposcmd.CreateNewRepoFromTemplateCmd(failingRepoService{})
	cmd.SetArgs([]string{"--org", "o", "--template-name", "template", "--name", "n"})
	if err := cmd.Execute(); !errors.Is(err, errRepoCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestGetRepoCmdPropagatesServiceError(t *testing.T) {
	cmd := reposcmd.GetRepoCmd(failingRepoService{})
	cmd.SetArgs([]string{"--org", "o", "--name", "n"})
	if err := cmd.Execute(); !errors.Is(err, errRepoCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestEditRepoCmdPropagatesServiceError(t *testing.T) {
	cmd := reposcmd.EditRepo(failingRepoService{})
	cmd.SetArgs([]string{"--org", "o", "--name", "n", "--desc", "new description"})
	if err := cmd.Execute(); !errors.Is(err, errRepoCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestDeleteRepoCmdPropagatesServiceError(t *testing.T) {
	cmd := reposcmd.DeleteRepoCmd(failingRepoService{})
	cmd.SetArgs([]string{"--org", "o", "--name", "n", "--yes"})
	if err := cmd.Execute(); !errors.Is(err, errRepoCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}
