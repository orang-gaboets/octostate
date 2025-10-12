package repos

import (
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// CreateFromTemplateOptions defines the options for creating a repository from a template.
type CreateFromTemplateOptions struct {
	Service            Service
	Name               string
	Owner              string
	TemplateOwner      string
	TemplateRepo       string
	Description        *string
	Private            *bool
	Topics             []string
	IncludeAllBranches bool
}

// Validate checks if the CreateFromTemplateOptions are valid.
func (opt *CreateFromTemplateOptions) Validate() error {
	if opt.Name == "" || opt.Owner == "" || opt.TemplateOwner == "" || opt.TemplateRepo == "" {
		return github.ErrMissingRequiredField
	}
	if opt.Service == nil {
		return github.ErrNilService
	}
	return nil
}

// DeleteOptions defines the options for deleting a repository.
type DeleteOptions struct {
	Service Service
	Repo    string
	Owner   string
}

// Validate checks if the DeleteOptions are valid.
func (opt *DeleteOptions) Validate() error {
	if opt.Repo == "" || opt.Owner == "" {
		return github.ErrMissingRequiredField
	}
	if opt.Service == nil {
		return github.ErrNilService
	}
	return nil
}

// EditOptions defines the options for editing a repository.
type EditOptions struct {
	Service      Service
	Repo         string
	Owner        string
	Description  *string
	Homepage     *string
	Private      *bool
	IsTemplate   *bool
	Archived     *bool
	AllowForking *bool
}

// Validate checks if the EditOptions are valid.
func (opt *EditOptions) Validate() error {
	if opt.Repo == "" || opt.Owner == "" {
		return github.ErrMissingRequiredField
	}
	if opt.Service == nil {
		return github.ErrNilService
	}
	return nil
}
