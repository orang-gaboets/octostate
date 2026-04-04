package organization_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	organizationcmd "github.com/orang-gaboets/octostate/cmd/octostate/organization"
	"github.com/orang-gaboets/octostate/pkg/github"
)

func TestListOrgMembersCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgMembersCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestListOrgMembersCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgMembersCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOrgMembersCmdAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgMembersCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOrgMembersCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgMembersCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestListOrgMembersCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgMembersCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestListOrgMembersCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgMembersCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestListOrgMembersCmdWithMissingOrg(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgMembersCmd(nil)
	c.SetArgs([]string{"--token", "t"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing org flag")
	}
}

func TestListOrgMembersCmdWithInvalidRole(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgMembersCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--role", "invalid"})
	if err := c.Execute(); !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("expected error %v, got %v", github.ErrInvalidFieldValue, err)
	}
}
