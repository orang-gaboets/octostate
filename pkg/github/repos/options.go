package repos

import (
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

type CreateFromTemplateOptions struct {
	NewRepo            github.Repository
	TemplateRepo       github.Repository
	IncludeAllBranches bool
	Service            Service
}

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
