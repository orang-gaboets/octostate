package topic_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	topicscmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/topic"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

func TestListAllTopicsNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ListAllTopicsCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestListAllTopicsAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ListAllTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListAllTopicsAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ListAllTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListAllTopicsPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ListAllTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestListAllTopicsBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ListAllTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestListAllTopicsWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ListAllTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestListAllTopicsWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ListAllTopicsCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got == "" {
		t.Fatalf("expected stdout output, got empty string")
	}
	if !strings.HasPrefix(got, "[") {
		t.Fatalf("expected JSON array output, got %q", got)
	}
}
