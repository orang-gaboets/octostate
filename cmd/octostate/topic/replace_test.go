package topic_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	topicscmd "github.com/orang-gaboets/octostate/cmd/octostate/topic"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureReplaceAllTopicsService struct {
	auth.MockRepoService
	replaceAllTopicsCalled bool
}

func (s *captureReplaceAllTopicsService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
	s.replaceAllTopicsCalled = true
	return topics, nil, nil
}

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
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Replaced topics for repository o/n") {
		t.Fatalf("unexpected success output: %q", got)
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

func TestReplaceAllTopicsDryRunSkipsTopicService(t *testing.T) {
	svc := &captureReplaceAllTopicsService{}
	c := topicscmd.ReplaceAllTopicsCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "a,b", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.replaceAllTopicsCalled {
		t.Fatalf("expected replace topics service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would replace topics for repository o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}
