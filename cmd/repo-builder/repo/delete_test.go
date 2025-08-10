package repo_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	reposcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

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
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteRepoAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--app-id", "1", "--installation-id", "2", "--app-key-path", "path", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteRepoPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--app-id", "1", "--org", "o", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestDeleteRepoBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "1", "--installation-id", "2", "--app-key-path", "path", "--org", "o", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestDeleteRepoWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--invalid-flag", "invalid-value"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}
