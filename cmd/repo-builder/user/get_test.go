package user_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	usercmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/user"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

func TestGetUserCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestGetUserCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserCmd(nil)
	c.SetArgs([]string{"--token", "t", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetUserCmdAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetUserCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestGetUserCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestGetUserCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserCmd(nil)
	c.SetArgs([]string{"--token", "t", "--username", "u", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}
