package topics

import (
	"context"

	"github.com/google/go-github/v55/github"
)

type Service interface {
	ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *github.Response, error)
	ListAllTopics(ctx context.Context, owner, repo string) ([]string, *github.Response, error)
}
