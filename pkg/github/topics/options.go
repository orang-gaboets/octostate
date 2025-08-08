package topics

import (
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// ListAllTopicsOptions defines the options for listing all topics of a repository.
type ListAllTopicsOptions struct {
	Repo    github.Repository
	Service Service
}

// ReplaceAllTopicsOptions defines the options for replacing all topics of a repository.
type ReplaceAllTopicsOptions struct {
	Repo    github.Repository
	Service Service
	Topics  []string
}

// AddTopicsOptions defines the options for adding topics to a repository.
type AddTopicsOptions struct {
	Repo    github.Repository
	Service Service
	Topics  []string
}
