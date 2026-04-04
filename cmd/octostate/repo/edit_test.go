package repo_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	reposcmd "github.com/orang-gaboets/octostate/cmd/octostate/repo"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureEditRepoService struct {
	auth.MockRepoService
	editCalled bool
}

func (s *captureEditRepoService) Edit(_ context.Context, _, _ string, _ *gh.Repository) (*gh.Repository, *gh.Response, error) {
	s.editCalled = true
	return &gh.Repository{}, nil, nil
}

func TestEditRepoNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestEditRepoAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{
		"--token", "t",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Edited repository o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestEditRepoAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{
		"--app-id", "123",
		"--installation-id", "456",
		"--app-key-path", "path/to/key.pem",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditRepoPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{
		"--app-id", "123",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestEditRepoBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{
		"--token", "t",
		"--app-id", "123",
		"--installation-id", "456",
		"--app-key-path", "path/to/key.pem",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestEditRepoWithOptionalFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
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
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
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

func TestEditRepoDryRunSkipsEditService(t *testing.T) {
	svc := &captureEditRepoService{}
	c := reposcmd.EditRepo(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--desc", "d", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.editCalled {
		t.Fatalf("expected edit service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would edit repository o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}
