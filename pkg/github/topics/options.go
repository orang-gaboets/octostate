package topics

import (
	"fmt"

	"github.com/orang-gaboets/octostate/pkg/github"
)

// ListAllTopicsOptions defines the options for listing all topics of a repository.
type ListAllTopicsOptions struct {
	Service Service
	Repo    string
	Owner   string
}

// Validate checks if the ListAllTopicsOptions are valid.
func (opt ListAllTopicsOptions) Validate() error {
	if opt.Repo == "" || opt.Owner == "" {
		return github.ErrMissingRequiredField
	}
	if opt.Service == nil {
		return github.ErrNilService
	}
	return nil
}

// ReplaceAllTopicsOptions defines the options for replacing all topics of a repository.
type ReplaceAllTopicsOptions struct {
	Service Service
	Repo    string
	Owner   string
	Topics  []string
}

// Validate checks if the ReplaceAllTopicsOptions are valid.
func (opt ReplaceAllTopicsOptions) Validate() error {
	if opt.Repo == "" || opt.Owner == "" {
		return github.ErrMissingRequiredField
	}
	if opt.Service == nil {
		return github.ErrNilService
	}
	return nil
}

// AddTopicsOptions defines the options for adding topics to a repository.
type AddTopicsOptions struct {
	Service Service
	Repo    string
	Owner   string
	Topics  []string
}

// Validate checks if the AddTopicsOptions are valid.
func (opt AddTopicsOptions) Validate() error {
	if opt.Repo == "" || opt.Owner == "" {
		return github.ErrMissingRequiredField
	}
	if opt.Service == nil {
		return github.ErrNilService
	}
	if len(opt.Topics) == 0 {
		return fmt.Errorf("no topics to add to repository %s/%s: %w", opt.Owner, opt.Repo, github.ErrMissingRequiredField)
	}
	return nil
}
