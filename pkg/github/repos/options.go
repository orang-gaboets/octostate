package repos

import (
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// CreateFromTemplateOptions defines the options for creating a repository from a template.
type CreateFromTemplateOptions struct {
	NewRepo            github.Repository
	TemplateRepo       github.Repository
	IncludeAllBranches bool
	Service            Service
}

// DeleteOptions defines the options for deleting a repository.
type DeleteOptions struct {
	Service    Service
	Repository github.Repository
}

// EditOptions defines the options for editing a repository.
type EditOptions struct {
	Service      Service
	Repository   github.Repository
	Description  *string
	Homepage     *string
	Private      *bool
	IsTemplate   *bool
	Archived     *bool
	AllowForking *bool
}
