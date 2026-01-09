package teams

import (
	"context"
	"fmt"
	"log"

	gh "github.com/google/go-github/v55/github"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// CreateTeam creates a new team in the specified organization.
func CreateTeam(ctx context.Context, option CreateTeamOptions) (*github.Team, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	var parentTeam *github.Team
	if option.ParentTeamSlug != nil {
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

	log.Printf("Creating team %s/%s", option.Org, option.Name)
	ghTeam, _, err := option.Service.CreateTeam(ctx, option.Org, newTeam)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to create team %s/%s", option.Org, option.Name))
	}
	team := github.TeamFromGhTeam(ghTeam)
	log.Printf("Successfully created team: %s", team)
	return team, nil
}

// DeleteTeamBySlug deletes a team by its slug within an organization.
func DeleteTeamBySlug(ctx context.Context, option DeleteTeamBySlugOptions) error {
	if err := option.Validate(); err != nil {
		return err
	}
	log.Printf("Deleting team %s/%s", option.Org, option.Slug)

	_, err := option.Service.DeleteTeamBySlug(ctx, option.Org, option.Slug)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to delete team %s/%s", option.Org, option.Slug))
	}

	log.Printf("Successfully deleted team %s/%s", option.Org, option.Slug)
	return nil
}

// GetTeamBySlug retrieves a team by its slug within an organization.
func GetTeamBySlug(ctx context.Context, option GetTeamBySlugOptions) (*github.Team, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	log.Printf("Retrieving team %s/%s", option.Org, option.Slug)

	ghTeam, _, err := option.Service.GetTeamBySlug(ctx, option.Org, option.Slug)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to get team %s/%s", option.Org, option.Slug))
	}

	team := github.TeamFromGhTeam(ghTeam)
	log.Printf("Successfully retrieved team: %s", team)
	return team, nil
}

// ListTeams retrieves all teams in a GitHub organization.
func ListTeams(ctx context.Context, option ListTeamsOptions) ([]*github.Team, error) {
	if err := option.Validate(); err != nil {
		return nil, err
	}

	log.Printf("Listing teams for organization %s", option.Org)

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

	log.Printf("Retrieved %d teams for organization %s", len(allTeams), option.Org)
	log.Printf("Teams: %+v", allTeams)
	return allTeams, nil
}
