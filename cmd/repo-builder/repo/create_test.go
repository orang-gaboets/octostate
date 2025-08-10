package repo_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	reposcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/repo"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

func TestCreateRepoFromTemplateNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestCreateRepoFromTemplateAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
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
