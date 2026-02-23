package topics

import (
	"context"
	"fmt"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// ListAllTopics lists all topics for a given repository.
func ListAllTopics(ctx context.Context, option ListAllTopicsOptions) ([]string, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}
	topics, _, err := option.Service.ListAllTopics(ctx, option.Owner, option.Repo)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to list topics for repository %s/%s", option.Owner, option.Repo))
	}
	return topics, nil
}

// ReplaceAllTopics replaces all topics for a given repository with the provided topics.
func ReplaceAllTopics(ctx context.Context, option ReplaceAllTopicsOptions) ([]string, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	uniqueTopics := github.Unique(option.Topics)

	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Owner, option.Repo, uniqueTopics)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to replace topics for repository %s/%s", option.Owner, option.Repo))
	}
	return topics, nil
}

// AddTopics adds topics to a given repository, merging them with existing topics.
func AddTopics(ctx context.Context, option AddTopicsOptions) ([]string, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	oldTopics, _, err := option.Service.ListAllTopics(ctx, option.Owner, option.Repo)
	if err != nil {
		return nil, github.WrapError(err, "failed to list existing topics")
	}

	uniqueTopics := github.MergeUnique(oldTopics, option.Topics)

	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Owner, option.Repo, uniqueTopics)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to add topics to repository %s/%s", option.Owner, option.Repo))
	}
	return topics, nil
}
