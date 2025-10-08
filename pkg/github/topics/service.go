package topics

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// ListAllTopics lists all topics for a given repository.
func ListAllTopics(ctx context.Context, option ListAllTopicsOptions) ([]string, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}
	log.Printf("Listing topics for repository %s/%s", option.Owner, option.Repo)
	topics, _, err := option.Service.ListAllTopics(ctx, option.Owner, option.Repo)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to list topics for repository %s/%s", option.Owner, option.Repo))
	}
	log.Printf("Topics of repository %s/%s: %v", option.Owner, option.Repo, strings.Join(topics, ", "))
	return topics, nil
}

// ReplaceAllTopics replaces all topics for a given repository with the provided topics.
func ReplaceAllTopics(ctx context.Context, option ReplaceAllTopicsOptions) ([]string, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	uniqueTopics := github.Unique(option.Topics)

	log.Printf("Setting topics for repository %s/%s: %v", option.Owner, option.Repo, strings.Join(uniqueTopics, ", "))
	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Owner, option.Repo, uniqueTopics)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to replace topics for repository %s/%s", option.Owner, option.Repo))
	}
	log.Printf("Repository %s/%s topics have been successfully updated to %v", option.Owner, option.Repo, strings.Join(topics, ", "))
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

	log.Printf("Current topics for repository %s/%s: %v", option.Owner, option.Repo, strings.Join(oldTopics, ", "))

	uniqueTopics := github.MergeUnique(oldTopics, option.Topics)

	log.Printf("Adding topics to repository %s/%s: %v", option.Owner, option.Repo, strings.Join(uniqueTopics, ", "))
	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Owner, option.Repo, uniqueTopics)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to add topics to repository %s/%s", option.Owner, option.Repo))
	}
	log.Printf("Topics for repository %s/%s have been successfully updated to %v", option.Owner, option.Repo, strings.Join(topics, ", "))
	return topics, nil
}
