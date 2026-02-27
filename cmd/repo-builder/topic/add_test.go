package topic_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	topicscmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/topic"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

type captureAddTopicsService struct {
	auth.MockRepoService
	listAllTopicsCalled    bool
	replaceAllTopicsCalled bool
}

func (s *captureAddTopicsService) ListAllTopics(_ context.Context, _, _ string) ([]string, *gh.Response, error) {
	s.listAllTopicsCalled = true
	return []string{}, nil, nil
}

func (s *captureAddTopicsService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
	s.replaceAllTopicsCalled = true
	return topics, nil, nil
}

func TestAddTopicsNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestAddTopicsAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
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
	if !strings.Contains(got, "Added topics to repository o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestAddTopicsAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTopicsPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestAddTopicsBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestAddTopicsWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestAddTopicsDryRunSkipsTopicServices(t *testing.T) {
	svc := &captureAddTopicsService{}
	c := topicscmd.AddTopicsCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "a,b", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.listAllTopicsCalled || svc.replaceAllTopicsCalled {
		t.Fatalf("expected topic services not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would add topics to repository o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}
