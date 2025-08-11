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
	if option.Service == nil {
		return nil, github.ErrNilService
	}
	if option.Repo.Org == "" || option.Repo.Name == "" {
		return nil, fmt.Errorf("repository organization and name must be provided: %w", github.ErrMissingRequiredField)
	}

	log.Printf("Listing topics for repository %s/%s", option.Repo.Org, option.Repo.Name)
	topics, _, err := option.Service.ListAllTopics(ctx, option.Repo.Org, option.Repo.Name)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to list topics for repository %s/%s", option.Repo.Org, option.Repo.Name))
	}
	log.Printf("Topics of repository %s/%s: %v", option.Repo.Org, option.Repo.Name, strings.Join(topics, ", "))
	return topics, nil
}

// ReplaceAllTopics replaces all topics for a given repository with the provided topics.
func ReplaceAllTopics(ctx context.Context, option ReplaceAllTopicsOptions) ([]string, error) {
	if option.Service == nil {
		return nil, github.ErrNilService
	}

	if option.Repo.Org == "" || option.Repo.Name == "" {
		return nil, fmt.Errorf("repository organization and name must be provided: %w", github.ErrMissingRequiredField)
	}

	if len(option.Topics) == 0 {
		return nil, fmt.Errorf("no topics to set for repository %s/%s: %w", option.Repo.Org, option.Repo.Name, github.ErrMissingRequiredField)
	}

	uniqueTopics := github.Unique(option.Topics)

	log.Printf("Setting topics for repository %s/%s: %v", option.Repo.Org, option.Repo.Name, strings.Join(uniqueTopics, ", "))
	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Repo.Org, option.Repo.Name, uniqueTopics)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to replace topics for repository %s/%s", option.Repo.Org, option.Repo.Name))
	}
	log.Printf("Repository %s/%s topics have been successfully updated to %v", option.Repo.Org, option.Repo.Name, strings.Join(topics, ", "))
	return topics, nil
}

// AddTopics adds topics to a given repository, merging them with existing topics.
func AddTopics(ctx context.Context, option AddTopicsOptions) ([]string, error) {
	if option.Service == nil {
		return nil, github.ErrNilService
	}

	if option.Repo.Org == "" || option.Repo.Name == "" {
		return nil, fmt.Errorf("repository organization and name must be provided: %w", github.ErrMissingRequiredField)
	}

	if len(option.Topics) == 0 {
		return nil, fmt.Errorf("no topics to add to repository %s/%s: %w", option.Repo.Org, option.Repo.Name, github.ErrMissingRequiredField)
	}

	oldTopics, _, err := option.Service.ListAllTopics(ctx, option.Repo.Org, option.Repo.Name)
	if err != nil {
		return nil, github.WrapError(err, "failed to list existing topics")
	}

	log.Printf("Current topics for repository %s/%s: %v", option.Repo.Org, option.Repo.Name, strings.Join(oldTopics, ", "))

	uniqueTopics := github.MergeUnique(oldTopics, option.Topics)

	log.Printf("Adding topics to repository %s/%s: %v", option.Repo.Org, option.Repo.Name, strings.Join(uniqueTopics, ", "))
	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Repo.Org, option.Repo.Name, uniqueTopics)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to add topics to repository %s/%s", option.Repo.Org, option.Repo.Name))
	}
	log.Printf("Topics for repository %s/%s have been successfully updated to %v", option.Repo.Org, option.Repo.Name, strings.Join(topics, ", "))
	return topics, nil
}
