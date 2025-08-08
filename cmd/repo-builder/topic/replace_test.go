package topic_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	topicscmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/topic"
)

// mockReplaceAllTopicsService implements topics.Service for testing.
type mockReplaceAllTopicsService struct{}

func (mockReplaceAllTopicsService) ListAllTopics(_ context.Context, _, _ string) ([]string, *github.Response, error) {
	return []string{"topic1", "topic2"}, nil, nil
}

func (mockReplaceAllTopicsService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func TestReplaceAllTopicsNoRequiredFlags(t *testing.T) {
	c := topicscmd.ReplaceAllTopicsCmd(mockReplaceAllTopicsService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestReplaceAllTopicsAllRequiredFlagsProvided(t *testing.T) {
	c := topicscmd.ReplaceAllTopicsCmd(mockReplaceAllTopicsService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
