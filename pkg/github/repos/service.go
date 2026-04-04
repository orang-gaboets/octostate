package repos

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v55/github"

	"github.com/orang-gaboets/octostate/pkg/github"
	ghlogging "github.com/orang-gaboets/octostate/pkg/github/logging"
	"github.com/orang-gaboets/octostate/pkg/github/topics"
)

// CreateFromTemplate creates a repository from a template and optionally sets topics.
func CreateFromTemplate(ctx context.Context, option CreateFromTemplateOptions) (*gh.Repository, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	req := &gh.TemplateRepoRequest{
		Owner:              &option.Owner,
		Name:               &option.Name,
		Description:        option.Description,
		Private:            option.Private,
		IncludeAllBranches: &option.IncludeAllBranches,
	}

	ghlogging.Debugf(ctx, "create repository %s/%s from template %s/%s", option.Owner, option.Name, option.TemplateOwner, option.TemplateRepo)
	newRepo, _, err := option.Service.CreateFromTemplate(ctx, option.TemplateOwner, option.TemplateRepo, req)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to create repository from template %s/%s", option.TemplateOwner, option.TemplateRepo))
	}
	ghlogging.Debugf(ctx, "created repository %s/%s", option.Owner, option.Name)

	if !option.SkipTopicSync {
		listTemplateTopicsOptions := topics.ListAllTopicsOptions{
			Owner:   option.TemplateOwner,
			Repo:    option.TemplateRepo,
			Service: option.Service,
		}
		templateTopics, err := topics.ListAllTopics(ctx, listTemplateTopicsOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to list template topics for %s/%s", option.TemplateOwner, option.TemplateRepo))
		}
		ghlogging.Debugf(ctx, "loaded %d template topics for %s/%s", len(templateTopics), option.TemplateOwner, option.TemplateRepo)

		uniqueTopics := github.MergeUnique(option.Topics, templateTopics)

		if len(uniqueTopics) > 0 {
			ghlogging.Debugf(ctx, "setting %d merged topics on %s/%s", len(uniqueTopics), option.Owner, option.Name)
			newRepoTopicsOptions := topics.ReplaceAllTopicsOptions{
				Owner:   option.Owner,
				Repo:    option.Name,
				Service: option.Service,
				Topics:  uniqueTopics,
			}
			_, err := topics.ReplaceAllTopics(ctx, newRepoTopicsOptions)
			if err != nil {
				return nil, github.WrapError(err, fmt.Sprintf("failed to set topics for new repository %s/%s", option.Owner, option.Name))
			}
		}
	}
	return newRepo, nil
}

// Delete removes a repository from GitHub.
func Delete(ctx context.Context, option DeleteOptions) error {
	if err := option.Validate(); err != nil {
		return err
	}

	ghlogging.Debugf(ctx, "delete repository %s/%s", option.Owner, option.Repo)
	_, err := option.Service.Delete(ctx, option.Owner, option.Repo)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to delete repository %s/%s", option.Owner, option.Repo))
	}
	ghlogging.Debugf(ctx, "deleted repository %s/%s", option.Owner, option.Repo)
	return nil
}

// Get retrieves a repository from GitHub.
func Get(ctx context.Context, option GetOptions) (*github.Repository, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	ghlogging.Debugf(ctx, "get repository %s/%s", option.Owner, option.Repo)
	ghRepo, _, err := option.Service.Get(ctx, option.Owner, option.Repo)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve repository %s/%s", option.Owner, option.Repo))
	}
	repo := github.RepositoryFromGhRepo(ghRepo)
	ghlogging.Debugf(ctx, "retrieved repository %s/%s", option.Owner, option.Repo)
	return repo, nil
}

// Edit updates the properties of an existing repository.
func Edit(ctx context.Context, option EditOptions) (*gh.Repository, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	repo := &gh.Repository{
		Description:  option.Description,
		Homepage:     option.Homepage,
		Private:      option.Private,
		IsTemplate:   option.IsTemplate,
		Archived:     option.Archived,
		AllowForking: option.AllowForking,
	}

	ghlogging.Debugf(ctx, "edit repository %s/%s", option.Owner, option.Repo)
	updatedRepo, _, err := option.Service.Edit(ctx, option.Owner, option.Repo, repo)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to edit repository %s/%s", option.Owner, option.Repo))
	}
	ghlogging.Debugf(ctx, "edited repository %s/%s", option.Owner, option.Repo)
	return updatedRepo, nil
}

// ListOrgRepos retrieves all repositories within the specified organization.
func ListOrgRepos(ctx context.Context, option ListOrgReposOptions) ([]*github.Repository, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	listOptions := &gh.RepositoryListByOrgOptions{
		Type: string(option.Type),
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}

	var allRepos []*github.Repository
	for {
		ghRepos, resp, err := option.Service.ListByOrg(ctx, option.Org, listOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to list repositories for organization %s", option.Org))
		}

		for _, repo := range github.RepositoriesFromGhRepos(ghRepos) {
			allRepos = append(allRepos, &repo)
		}

		if resp == nil || resp.NextPage == 0 {
			break
		}

		listOptions.Page = resp.NextPage
	}

	ghlogging.Debugf(ctx, "listed %d repositories for organization %s", len(allRepos), option.Org)
	return allRepos, nil
}
