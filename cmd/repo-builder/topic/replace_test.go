package topic_test

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	topicscmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/topic"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

func TestReplaceAllTopicsNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestReplaceAllTopicsAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplaceAllTopicsAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplaceAllTopicsPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestReplaceAllTopicsBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestReplaceAllTopicsWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}
