package user_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	usercmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/user"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

func TestGetUserByIDCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserByIDCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestGetUserByIDCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserByIDCmd(nil)
	c.SetArgs([]string{"--token", "t", "--id", "123"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetUserByIDCmdAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserByIDCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--id", "123"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetUserByIDCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserByIDCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--id", "123"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestGetUserByIDCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserByIDCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--id", "123"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestGetUserByIDCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserByIDCmd(nil)
	c.SetArgs([]string{"--token", "t", "--id", "123", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestGetUserByIDCmdWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := usercmd.GetUserByIDCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--id", "123"})
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
