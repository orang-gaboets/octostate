package team_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	teamcmd "github.com/orang-gaboets/octostate/cmd/octostate/team"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureCreateTeamService struct {
	auth.MockTeamsService
	createCalled bool
}

func (s *captureCreateTeamService) CreateTeam(_ context.Context, _ string, _ gh.NewTeam) (*gh.Team, *gh.Response, error) {
	s.createCalled = true
	return &gh.Team{}, nil, nil
}

func TestCreateTeamNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestCreateTeamAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Created team o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestCreateTeamAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateTeamPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestCreateTeamBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestCreateTeamWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestCreateTeamDryRunSkipsCreateService(t *testing.T) {
	svc := &captureCreateTeamService{}
	c := teamcmd.CreateTeamCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.createCalled {
		t.Fatalf("expected create team service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would create team o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}
