package organization_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	organizationcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/organization"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

func TestListOrgReposCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgReposCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestListOrgReposCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgReposCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOrgReposCmdAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgReposCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOrgReposCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgReposCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestListOrgReposCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgReposCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestListOrgReposCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgReposCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestListOrgReposCmdWithMissingOrg(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgReposCmd(nil)
	c.SetArgs([]string{"--token", "t"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing org flag")
	}
}
