package team_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	teamcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/team"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

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
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTeamAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTeamPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestDeleteTeamBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestDeleteTeamWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}
