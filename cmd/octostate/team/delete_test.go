package team_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	teamcmd "github.com/orang-gaboets/octostate/cmd/octostate/team"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureDeleteTeamBySlugService struct {
	auth.MockTeamsService
	deleteCalled bool
}

func (s *captureDeleteTeamBySlugService) DeleteTeamBySlug(_ context.Context, _, _ string) (*gh.Response, error) {
	s.deleteCalled = true
	return nil, nil
}

func TestDeleteTeamNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestDeleteTeamAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Deleted team o/s") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestDeleteTeamAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTeamPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--yes"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestDeleteTeamBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--yes"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestDeleteTeamWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--yes", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestDeleteTeamRequiresYesUnlessDryRun(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	err := c.Execute()
	if !errors.Is(err, safety.ErrConfirmationRequired) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestDeleteTeamDryRunSkipsDeleteService(t *testing.T) {
	svc := &captureDeleteTeamBySlugService{}
	c := teamcmd.DeleteTeamBySlugCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--dry-run"})
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
	if !strings.Contains(got, "Dry run: would delete team o/s") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}
