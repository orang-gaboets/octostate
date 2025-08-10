package repo_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	reposcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

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
	c.SetArgs([]string{
		"--token", "t",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
