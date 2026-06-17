package apply

import (
	"errors"
	"fmt"

	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	ghusers "github.com/orang-gaboets/octostate/pkg/github/users"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
)

type preflightFailures struct {
	errs []error
}

func (f *preflightFailures) add(action gitopsplan.Action, err error) {
	if err == nil {
		return
	}
	f.errs = append(f.errs, fmt.Errorf("%s %s %s: %w", action.Operation, action.ResourceType, action.ResourceID, err))
}

func (f *preflightFailures) err() error {
	if len(f.errs) == 0 {
		return nil
	}
	return fmt.Errorf("apply preflight failed for %d action(s): %w", len(f.errs), errors.Join(f.errs...))
}

func (e *executor) markPreflightCreatedTeam(slug string) {
	slugKey := teamSlugKey(slug)
	if slugKey == "" {
		return
	}
	e.preflightCreatedTeams[slugKey] = struct{}{}
}

func (e *executor) hasPreflightCreatedTeam(slug string) bool {
	_, ok := e.preflightCreatedTeams[teamSlugKey(slug)]
	return ok
}

func (e *executor) markPreflightCreatedRepo(owner, name string) {
	key := repositoryResourceID(owner, name)
	if key == "/" {
		return
	}
	e.preflightCreatedRepos[key] = struct{}{}
}

func (e *executor) hasPreflightCreatedRepo(owner, name string) bool {
	_, ok := e.preflightCreatedRepos[repositoryResourceID(owner, name)]
	return ok
}

func (e *executor) preflightGetRepository(owner, name string) (*github.Repository, error) {
	key := repositoryResourceID(owner, name)
	if repository, ok := e.preflightLiveRepos[key]; ok {
		return repository, nil
	}

	repository, err := repos.Get(e.ctx, repos.GetOptions{
		Service: e.repositoryService,
		Owner:   owner,
		Repo:    name,
	})
	if err != nil {
		return nil, err
	}
	e.preflightLiveRepos[key] = repository
	return repository, nil
}

func (e *executor) preflightTemplateRepository(owner, name string) (*github.Repository, error) {
	key := repositoryResourceID(owner, name)
	if repository, ok := e.preflightLiveRepos[key]; ok {
		return repository, nil
	}

	if e.hasPreflightCreatedRepo(owner, name) {
		desired, ok := e.desiredRepositories[key]
		if !ok {
			return nil, fmt.Errorf("desired repository %s not found: %w", key, github.ErrNotFound)
		}
		isTemplate, managed := desired.ManagedIsTemplate()
		return &github.Repository{
			Owner:      owner,
			Name:       name,
			IsTemplate: managed && isTemplate,
		}, nil
	}

	return e.preflightGetRepository(owner, name)
}

func (e *executor) preflightEnsureTeamExists(slug string) (int64, error) {
	if e.hasPreflightCreatedTeam(slug) {
		id, ok := e.teamIDs[teamSlugKey(slug)]
		if !ok || id <= 0 {
			return 0, fmt.Errorf("team %s does not have a resolvable team ID: %w", slug, github.ErrNotFound)
		}
		return id, nil
	}
	if _, ok := e.preflightVerifiedTeams[teamSlugKey(slug)]; ok {
		id, ok := e.teamIDs[teamSlugKey(slug)]
		if !ok || id <= 0 {
			return 0, fmt.Errorf("team %s does not have a resolvable team ID: %w", slug, github.ErrNotFound)
		}
		return id, nil
	}

	team, err := teams.GetTeamBySlug(e.ctx, teams.GetTeamBySlugOptions{
		Service: e.teamService,
		Org:     e.organization,
		Slug:    slug,
	})
	if err != nil {
		return 0, err
	}
	if team == nil || team.ID <= 0 {
		return 0, fmt.Errorf("team %s did not return a valid team ID: %w", slug, github.ErrInvalidFieldValue)
	}
	e.teamIDs[teamSlugKey(slug)] = team.ID
	e.preflightVerifiedTeams[teamSlugKey(slug)] = struct{}{}
	return team.ID, nil
}

func (e *executor) preflightResolveInvitationTeamIDs(teamSlugs []string) ([]int64, error) {
	teamIDs := make([]int64, 0, len(teamSlugs))
	for _, teamSlug := range teamSlugs {
		id, err := e.preflightEnsureTeamExists(teamSlug)
		if err != nil {
			return nil, fmt.Errorf("team slug %q does not have a resolvable team ID: %w", teamSlug, err)
		}
		teamIDs = append(teamIDs, id)
	}
	return teamIDs, nil
}

func (e *executor) preflightResolveInviteUserID(username string) (int64, error) {
	key := inviteUsernameKey(username)
	if userID, ok := e.resolvedInviteUserIDs[key]; ok && userID > 0 {
		return userID, nil
	}

	user, err := ghusers.GetUserByUsername(e.ctx, ghusers.GetUserByUsernameOptions{
		Service:  e.userService,
		Username: username,
	})
	if err != nil {
		return 0, err
	}
	if user == nil || user.ID == nil || *user.ID <= 0 {
		return 0, fmt.Errorf("resolved invite username %q without a valid user ID: %w", username, github.ErrInvalidFieldValue)
	}

	e.resolvedInviteUserIDs[key] = *user.ID
	return *user.ID, nil
}
