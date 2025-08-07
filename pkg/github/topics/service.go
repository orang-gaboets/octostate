package topics

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

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
	return topics, nil
}

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

	cleaned := make([]string, 0, len(option.Topics))
	for _, t := range option.Topics {
		if v := strings.TrimSpace(t); v != "" {
			cleaned = append(cleaned, v)
		}
	}

	log.Printf("Setting topics for repository %s/%s: %v", option.Repo.Org, option.Repo.Name, cleaned)
	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Repo.Org, option.Repo.Name, cleaned)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to replace topics for repository %s/%s", option.Repo.Org, option.Repo.Name))
	}
	return topics, nil
}

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

	log.Printf("Current topics for repository %s/%s: %v", option.Repo.Org, option.Repo.Name, oldTopics)

	cleanedSet := make(map[string]struct{})
	for _, t := range oldTopics {
		if v := strings.TrimSpace(t); v != "" {
			cleanedSet[v] = struct{}{}
		}
	}
	for _, t := range option.Topics {
		if v := strings.TrimSpace(t); v != "" {
			cleanedSet[v] = struct{}{}
		}
	}

	cleaned := make([]string, 0, len(cleanedSet))
	for topic := range cleanedSet {
		cleaned = append(cleaned, topic)
	}

	log.Printf("Adding topics to repository %s/%s: %v", option.Repo.Org, option.Repo.Name, cleaned)
	topics, _, err := option.Service.ReplaceAllTopics(ctx, option.Repo.Org, option.Repo.Name, cleaned)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to add topics to repository %s/%s", option.Repo.Org, option.Repo.Name))
	}
	return topics, nil
}
