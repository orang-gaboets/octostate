package teams

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v55/github"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	ghlogging "github.com/orang-gaboets/repo-builder/pkg/github/logging"
)

// CreateTeam creates a new team in the specified organization.
func CreateTeam(ctx context.Context, option CreateTeamOptions) (*github.Team, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	var parentTeam *github.Team
	if option.ParentTeamSlug != nil {
		ghlogging.Debugf(ctx, "resolve parent team %s/%s", option.Org, *option.ParentTeamSlug)
		var err error
		parentTeam, err = GetTeamBySlug(ctx, GetTeamBySlugOptions{
			Org:     option.Org,
			Slug:    *option.ParentTeamSlug,
			Service: option.Service,
		})
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve parent team %s/%s", option.Org, *option.ParentTeamSlug))
		}
	}

	newTeam := gh.NewTeam{
		Name:        option.Name,
		Description: option.Description,
		Privacy:     (*string)(option.Privacy),
		ParentTeamID: func() *int64 {
			if option.ParentTeamSlug != nil {
				return &parentTeam.ID
			}
			return nil
		}(),
	}

	ghlogging.Debugf(ctx, "create team %s/%s", option.Org, option.Name)
	ghTeam, _, err := option.Service.CreateTeam(ctx, option.Org, newTeam)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to create team %s/%s", option.Org, option.Name))
	}
	team := github.TeamFromGhTeam(ghTeam)
	ghlogging.Debugf(ctx, "created team %s/%s", option.Org, option.Name)
	return team, nil
}

// EditTeamBySlug updates a team by slug within an organization.
func EditTeamBySlug(ctx context.Context, option EditTeamBySlugOptions) (*github.Team, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	var teamName string
	if option.Name != nil {
		teamName = *option.Name
	} else {
		ghlogging.Debugf(ctx, "resolve current team before editing %s/%s", option.Org, option.Slug)
		currentTeam, err := GetTeamBySlug(ctx, GetTeamBySlugOptions{
			Org:     option.Org,
			Slug:    option.Slug,
			Service: option.Service,
		})
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve current team %s/%s before edit", option.Org, option.Slug))
		}
		if currentTeam == nil || currentTeam.Name == "" {
			return nil, fmt.Errorf("failed to resolve current team name for %s/%s: %w", option.Org, option.Slug, github.ErrNotFound)
		}
		teamName = currentTeam.Name
	}

	var parentTeamID *int64
	if option.ParentTeamSlug != nil {
		ghlogging.Debugf(ctx, "resolve parent team %s/%s for edit of %s/%s", option.Org, *option.ParentTeamSlug, option.Org, option.Slug)
		parentTeam, err := GetTeamBySlug(ctx, GetTeamBySlugOptions{
			Org:     option.Org,
			Slug:    *option.ParentTeamSlug,
			Service: option.Service,
		})
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve parent team %s/%s", option.Org, *option.ParentTeamSlug))
		}
		parentTeamID = &parentTeam.ID
	}

	editTeam := gh.NewTeam{
		Name:         teamName,
		Description:  option.Description,
		Privacy:      (*string)(option.Privacy),
		ParentTeamID: parentTeamID,
	}

	ghlogging.Debugf(ctx, "edit team %s/%s", option.Org, option.Slug)
	ghTeam, _, err := option.Service.EditTeamBySlug(ctx, option.Org, option.Slug, editTeam, option.RemoveParent)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to edit team %s/%s", option.Org, option.Slug))
	}
	team := github.TeamFromGhTeam(ghTeam)
	ghlogging.Debugf(ctx, "edited team %s/%s", option.Org, option.Slug)
	return team, nil
}

// DeleteTeamBySlug deletes a team by its slug within an organization.
func DeleteTeamBySlug(ctx context.Context, option DeleteTeamBySlugOptions) error {
	if err := option.Validate(); err != nil {
		return err
	}
	ghlogging.Debugf(ctx, "delete team %s/%s", option.Org, option.Slug)
	_, err := option.Service.DeleteTeamBySlug(ctx, option.Org, option.Slug)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to delete team %s/%s", option.Org, option.Slug))
	}
	ghlogging.Debugf(ctx, "deleted team %s/%s", option.Org, option.Slug)
	return nil
}

// GetTeamBySlug retrieves a team by its slug within an organization.
func GetTeamBySlug(ctx context.Context, option GetTeamBySlugOptions) (*github.Team, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	ghlogging.Debugf(ctx, "get team %s/%s", option.Org, option.Slug)
	ghTeam, _, err := option.Service.GetTeamBySlug(ctx, option.Org, option.Slug)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to get team %s/%s", option.Org, option.Slug))
	}

	team := github.TeamFromGhTeam(ghTeam)
	ghlogging.Debugf(ctx, "retrieved team %s/%s", option.Org, option.Slug)
	return team, nil
}

// ListTeamMembersBySlug retrieves all members for a GitHub team by its slug.
func ListTeamMembersBySlug(ctx context.Context, option ListTeamMembersBySlugOptions) ([]*github.User, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	listOptions := &gh.TeamListTeamMembersOptions{
		Role: string(option.Role),
		ListOptions: gh.ListOptions{
			PerPage: 100,
		},
	}

	var allMembers []*github.User
	for {
		ghMembers, resp, err := option.Service.ListTeamMembersBySlug(ctx, option.Org, option.Slug, listOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to list members for team %s/%s", option.Org, option.Slug))
		}

		allMembers = append(allMembers, github.UsersFromGhUsers(ghMembers)...)

		if resp == nil || resp.NextPage == 0 {
			break
		}

		listOptions.Page = resp.NextPage
	}

	ghlogging.Debugf(ctx, "listed %d members for team %s/%s", len(allMembers), option.Org, option.Slug)
	return allMembers, nil
}

// ListTeams retrieves all teams in a GitHub organization.
func ListTeams(ctx context.Context, option ListTeamsOptions) ([]*github.Team, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	listOptions := &gh.ListOptions{
		PerPage: 100,
	}

	var allTeams []*github.Team
	for {
		ghTeams, resp, err := option.Service.ListTeams(ctx, option.Org, listOptions)
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to list teams for organization %s", option.Org))
		}

		allTeams = append(allTeams, github.TeamsFromGhTeams(ghTeams)...)

		if resp == nil || resp.NextPage == 0 {
			break
		}

		listOptions.Page = resp.NextPage
	}

	ghlogging.Debugf(ctx, "listed %d teams for organization %s", len(allTeams), option.Org)
	return allTeams, nil
}
