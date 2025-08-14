package organization_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	organizationcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/organization"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

func TestInviteCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestInviteCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInviteCmdAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInviteCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestInviteCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestInviteCmdWithUsername(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInviteCmdWithUserID(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--id", "123"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInviteCmdWithBothUsernameAndUserID(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u", "--id", "123"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestInviteCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}
