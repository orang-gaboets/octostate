package topic_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	topicscmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/topic"
)

// mockListAllTopicsService implements topics.Service for testing.
type mockListAllTopicsService struct{}

func (mockListAllTopicsService) ListAllTopics(_ context.Context, _, _ string) ([]string, *github.Response, error) {
	return []string{"topic1", "topic2"}, nil, nil
}

func (mockListAllTopicsService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func TestListAllTopicsNoRequiredFlags(t *testing.T) {
	c := topicscmd.ListAllTopicsCmd(mockListAllTopicsService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestListAllTopicsAllRequiredFlagsProvided(t *testing.T) {
	c := topicscmd.ListAllTopicsCmd(mockListAllTopicsService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
