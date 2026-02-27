package repo_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/safety"
	reposcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

type captureDeleteRepoService struct {
	auth.MockRepoService
	deleteCalled bool
}

func (s *captureDeleteRepoService) Delete(_ context.Context, _, _ string) (*gh.Response, error) {
	s.deleteCalled = true
	return nil, nil
}

func TestDeleteRepoNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestDeleteRepoAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Deleted repository o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestDeleteRepoAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--app-id", "1", "--installation-id", "2", "--app-key-path", "path", "--org", "o", "--name", "n", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteRepoPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--app-id", "1", "--org", "o", "--name", "n", "--yes"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestDeleteRepoBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "1", "--installation-id", "2", "--app-key-path", "path", "--org", "o", "--name", "n", "--yes"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestDeleteRepoWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--yes", "--invalid-flag", "invalid-value"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestDeleteRepoRequiresYesUnlessDryRun(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	err := c.Execute()
	if !errors.Is(err, safety.ErrConfirmationRequired) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestDeleteRepoDryRunSkipsDeleteService(t *testing.T) {
	svc := &captureDeleteRepoService{}
	c := reposcmd.DeleteRepoCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.deleteCalled {
		t.Fatalf("expected delete service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would delete repository o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}
