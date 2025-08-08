package topic_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	topicscmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/topic"
)

// mockAddTopicsService implements topics.Service for testing.
type mockAddTopicsService struct{}

func (mockAddTopicsService) ListAllTopics(_ context.Context, _, _ string) ([]string, *github.Response, error) {
	return []string{"topic1", "topic2"}, nil, nil
}

func (mockAddTopicsService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func (mockAddTopicsService) AddTopics(_ context.Context, _, _ string, topics []string) ([]string, *github.Response, error) {
	return topics, nil, nil
}

func TestAddTopicsNoRequiredFlags(t *testing.T) {
	c := topicscmd.AddTopicsCmd(mockAddTopicsService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestAddTopicsAllRequiredFlagsProvided(t *testing.T) {
	c := topicscmd.AddTopicsCmd(mockAddTopicsService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
