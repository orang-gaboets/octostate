package topics

import (
	"context"
	"fmt"

	"github.com/orang-gaboets/octostate/pkg/github"
	ghlogging "github.com/orang-gaboets/octostate/pkg/github/logging"
)

// ListAllTopics lists all topics for a given repository.
func ListAllTopics(ctx context.Context, option ListAllTopicsOptions) ([]string, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}
	ghlogging.Debugf(ctx, "list topics for repository %s/%s", option.Owner, option.Repo)
	topics, _, err := option.Service.ListAllTopics(ctx, option.Owner, option.Repo)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to list topics for repository %s/%s", option.Owner, option.Repo))
	}
	ghlogging.Debugf(ctx, "listed %d topics for repository %s/%s", len(topics), option.Owner, option.Repo)
	return topics, nil
}

// ReplaceAllTopics replaces all topics for a given repository with the provided topics.
func ReplaceAllTopics(ctx context.Context, option ReplaceAllTopicsOptions) ([]string, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	uniqueTopics := github.Unique(option.Topics)
	if uniqueTopics == nil {
		uniqueTopics = []string{}
	}

	ghlogging.Debugf(ctx, "replace topics on repository %s/%s with %d topics", option.Owner, option.Repo, len(uniqueTopics))
	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Owner, option.Repo, uniqueTopics)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to replace topics for repository %s/%s", option.Owner, option.Repo))
	}
	ghlogging.Debugf(ctx, "replaced topics on repository %s/%s; final count=%d", option.Owner, option.Repo, len(topics))
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

	ghlogging.Debugf(ctx, "add topics to repository %s/%s (existing=%d requested=%d merged=%d)", option.Owner, option.Repo, len(oldTopics), len(option.Topics), len(uniqueTopics))
	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Owner, option.Repo, uniqueTopics)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to add topics to repository %s/%s", option.Owner, option.Repo))
	}
	ghlogging.Debugf(ctx, "updated topics on repository %s/%s; final count=%d", option.Owner, option.Repo, len(topics))
	return topics, nil
}
