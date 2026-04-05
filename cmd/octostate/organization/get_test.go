package organization_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	organizationcmd "github.com/orang-gaboets/octostate/cmd/octostate/organization"
	"github.com/orang-gaboets/octostate/pkg/github"
)

func TestGetOrgByNameCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByNameCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestGetOrgByNameCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByNameCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetOrgByNameAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByNameCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetOrgByNameCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByNameCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestGetOrgByNameCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByNameCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestGetOrgByNameCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByNameCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestGetOrgByNameCmdWithMissingOrg(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByNameCmd(nil)
	c.SetArgs([]string{"--token", "t"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing org flag")
	}
}

func TestGetOrgByNameCmdWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.GetOrgByNameCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got == "" {
		t.Fatalf("expected stdout output, got empty string")
	}
	if !strings.HasPrefix(got, "{") {
		t.Fatalf("expected JSON object output, got %q", got)
	}
}
