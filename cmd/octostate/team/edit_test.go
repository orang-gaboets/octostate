package team_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	teamcmd "github.com/orang-gaboets/octostate/cmd/octostate/team"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureEditTeamBySlugService struct {
	auth.MockTeamsService
	editCalled bool
}

func (s *captureEditTeamBySlugService) EditTeamBySlug(_ context.Context, _, _ string, _ gh.NewTeam, _ bool) (*gh.Team, *gh.Response, error) {
	s.editCalled = true
	return &gh.Team{}, nil, nil
}

func TestEditTeamNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestEditTeamAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditTeamAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditTeamPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestEditTeamBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestEditTeamWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestEditTeamParentAndClearParentConflict(t *testing.T) {
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--parent", "parent", "--clear-parent"})
	err := c.Execute()
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("expected error %v, got %v", github.ErrValidationFailed, err)
	}
}

func TestEditTeamEmptyParentSlugRejected(t *testing.T) {
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--parent", "   "})
	err := c.Execute()
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestEditTeamDryRunSkipsEditService(t *testing.T) {
	svc := &captureEditTeamBySlugService{}
	c := teamcmd.EditTeamCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--desc", "d", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.editCalled {
		t.Fatalf("expected edit team service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would edit team o/s") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestEditTeamWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Edited team o/s") {
		t.Fatalf("unexpected success output: %q", got)
	}
}
