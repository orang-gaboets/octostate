package repo_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	reposcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

type captureCreateRepoFromTemplateService struct {
	lastTemplateRepo string
	lastTemplateOrg  string
	lastRequest      *gh.TemplateRepoRequest
	createCalled     bool
}

func (m *captureCreateRepoFromTemplateService) CreateFromTemplate(_ context.Context, templateOwner, templateRepo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	m.lastTemplateOrg = templateOwner
	m.lastTemplateRepo = templateRepo
	m.lastRequest = req
	m.createCalled = true
	return &gh.Repository{}, nil, nil
}

func (*captureCreateRepoFromTemplateService) Delete(_ context.Context, _, _ string) (*gh.Response, error) {
	return nil, nil
}

func (*captureCreateRepoFromTemplateService) Edit(_ context.Context, _, _ string, _ *gh.Repository) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

func (*captureCreateRepoFromTemplateService) Get(_ context.Context, _, _ string) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

func (*captureCreateRepoFromTemplateService) ListByOrg(_ context.Context, _ string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
	return nil, nil, nil
}

func (*captureCreateRepoFromTemplateService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
	return topics, nil, nil
}

func (*captureCreateRepoFromTemplateService) ListAllTopics(_ context.Context, _, _ string) ([]string, *gh.Response, error) {
	return nil, nil, nil
}

func TestCreateRepoFromTemplateNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestCreateRepoFromTemplateMissingTemplateNameFlag(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for missing template-name flag")
	}
	if !strings.Contains(err.Error(), "template-name") {
		t.Fatalf("expected error mentioning template-name, got %v", err)
	}
}

func TestCreateRepoFromTemplateAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Created repository o/n from template o/temp") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestCreateRepoFromTemplateAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepoFromTemplatePartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestCreateRepoFromTemplateBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestCreateRepoFromTemplateWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestCreateRepoFromTemplateIncludeAllBranchesDefaultsFalse(t *testing.T) {
	svc := &captureCreateRepoFromTemplateService{}
	c := reposcmd.CreateNewRepoFromTemplateCmd(svc)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastRequest == nil || svc.lastRequest.IncludeAllBranches == nil {
		t.Fatalf("expected CreateFromTemplate request with IncludeAllBranches set")
	}
	if *svc.lastRequest.IncludeAllBranches {
		t.Fatalf("expected IncludeAllBranches default false, got true")
	}
}

func TestCreateRepoFromTemplateIncludeAllBranchesCanBeEnabled(t *testing.T) {
	svc := &captureCreateRepoFromTemplateService{}
	c := reposcmd.CreateNewRepoFromTemplateCmd(svc)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n", "--include-all-branches=true"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastRequest == nil || svc.lastRequest.IncludeAllBranches == nil {
		t.Fatalf("expected CreateFromTemplate request with IncludeAllBranches set")
	}
	if !*svc.lastRequest.IncludeAllBranches {
		t.Fatalf("expected IncludeAllBranches true when explicitly set")
	}
}

func TestCreateRepoFromTemplateDryRunSkipsCreateService(t *testing.T) {
	svc := &captureCreateRepoFromTemplateService{}
	c := reposcmd.CreateNewRepoFromTemplateCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n", "--topics", "a,b", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.createCalled {
		t.Fatalf("expected create service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would create repository o/n from template o/temp") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}
