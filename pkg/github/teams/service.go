package teams

import (
	"context"
	"fmt"
	"log"

	gh "github.com/google/go-github/v55/github"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// CreateTeam creates a new team in the specified organization.
func CreateTeam(ctx context.Context, opts CreateTeamOptions) (*github.Team, error) {
	if opts.Service == nil {
		return nil, github.ErrNilService
	}

	if len(opts.Team.Repos) > 0 {
		return nil, fmt.Errorf("creating teams with repositories is not supported yet")
	}

	var parentTeam *github.Team
	if opts.Team.ParentTeam != nil {
		if opts.Team.ParentTeam.Org != opts.Team.Org {
			return nil, github.WrapError(github.ErrUnauthorized, "parent team must belong to the same organization as the new team")
		}
		var err error
		parentTeam, err = GetTeamBySlug(ctx, GetTeamBySlugOptions{
			Org:     opts.Team.ParentTeam.Org,
			Slug:    opts.Team.ParentTeam.Slug,
			Service: opts.Service,
		})
		if err != nil {
			return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve parent team %s/%s", opts.Team.ParentTeam.Org, opts.Team.ParentTeam.Slug))
		}
	}

	opts.Team.ParentTeam = parentTeam

	newTeam := gh.NewTeam{
		Name:        opts.Team.Name,
		Description: gh.String(opts.Team.Description),
		Privacy:     gh.String(opts.Team.Privacy.String()),
		ParentTeamID: func() *int64 {
			if opts.Team.ParentTeam != nil {
				return gh.Int64(opts.Team.ParentTeam.ID)
			}
			return nil
		}(),
	}

	log.Printf("Creating team %s/%s", opts.Team.Org, opts.Team.Name)
	ghTeam, _, err := opts.Service.CreateTeam(ctx, opts.Team.Org, newTeam)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to create team %s/%s", opts.Team.Org, opts.Team.Name))
	}
	team := github.TeamFromGhTeam(ghTeam)
	log.Printf("Successfully created team: %s", team)
	return team, nil
}

// DeleteTeamBySlug deletes a team by its slug within an organization.
func DeleteTeamBySlug(ctx context.Context, opts DeleteTeamBySlugOptions) error {
	if opts.Service == nil {
		return github.ErrNilService
	}

	log.Printf("Deleting team %s/%s", opts.Org, opts.Slug)

	_, err := opts.Service.DeleteTeamBySlug(ctx, opts.Org, opts.Slug)
	if err != nil {
		return github.WrapError(err, fmt.Sprintf("failed to delete team %s/%s", opts.Org, opts.Slug))
	}

	log.Printf("Successfully deleted team %s/%s", opts.Org, opts.Slug)
	return nil
}

// GetTeamBySlug retrieves a team by its slug within an organization.
func GetTeamBySlug(ctx context.Context, opts GetTeamBySlugOptions) (*github.Team, error) {
	if opts.Service == nil {
		return nil, github.ErrNilService
	}

	log.Printf("Retrieving team %s/%s", opts.Org, opts.Slug)

	ghTeam, _, err := opts.Service.GetTeamBySlug(ctx, opts.Org, opts.Slug)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to get team %s/%s", opts.Org, opts.Slug))
	}

	team := github.TeamFromGhTeam(ghTeam)
	log.Printf("Successfully retrieved team: %s", team)
	return team, nil
}
