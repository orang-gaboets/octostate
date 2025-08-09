package topics

import (
	"context"

	"github.com/google/go-github/v55/github"
)

// Service defines the interface for managing topics in GitHub repositories.
type Service interface {
	// ReplaceAllTopics replaces all topics for a given repository with the provided list of topics.
	ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *github.Response, error)
	// AddTopics adds new topics to an existing repository, preserving existing topics.
	ListAllTopics(ctx context.Context, owner, repo string) ([]string, *github.Response, error)
}
