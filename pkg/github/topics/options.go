package topics

import (
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

type ListAllTopicsOptions struct {
	Repo    github.Repository
	Service Service
}

type ReplaceAllTopicsOptions struct {
	Repo    github.Repository
	Service Service
	Topics  []string
}

type AddTopicsOptions struct {
	Repo    github.Repository
	Service Service
	Topics  []string
}
