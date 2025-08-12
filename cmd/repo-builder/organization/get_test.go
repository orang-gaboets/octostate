package organization_test

import (
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	organizationcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/organization"
)

func TestGetOrgByNameNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByName(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestGetOrgByNameAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByName(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetOrgByNameAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByName(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetOrgByNamePartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByName(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for partial app credentials")
	}
}

func TestGetOrgByNameBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByName(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for conflicting credentials")
	}
}

func TestGetOrgByNameWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByName(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestGetOrgWithMissingOrg(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByName(nil)
	c.SetArgs([]string{"--token", "t"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing org flag")
	}
}
