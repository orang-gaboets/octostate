package repos

import (
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

type RepoCreationOptions struct {
	NewRepo      github.Repository
	TemplateRepo github.Repository
	Service      Service
}
