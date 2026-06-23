package topics

import (
	"context"

	gh "github.com/google/go-github/v88/github"
)

// Service defines the interface for managing topics in GitHub repositories.
type Service interface {
	// ReplaceAllTopics replaces all topics for a given repository with the provided list of topics.
	ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *gh.Response, error)
	// ListAllTopics lists all topics for a given repository.
	ListAllTopics(ctx context.Context, owner, repo string) ([]string, *gh.Response, error)
}
