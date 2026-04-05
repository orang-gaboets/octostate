package team_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	teamcmd "github.com/orang-gaboets/octostate/cmd/octostate/team"
	"github.com/orang-gaboets/octostate/pkg/github"
)

func TestGetTeamBySlugNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.GetTeamBySlugCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestGetTeamBySlugAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.GetTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTeamBySlugAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.GetTeamBySlugCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetTeamBySlugPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.GetTeamBySlugCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestGetTeamBySlugBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.GetTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestGetTeamBySlugWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.GetTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestGetTeamBySlugWithMissingSlug(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.GetTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing slug flag")
	}
}
