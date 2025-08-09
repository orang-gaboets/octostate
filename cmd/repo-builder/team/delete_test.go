package team_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	teamcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/team"
)

// mockTeamDeleteService implements teams.Service for testing.
type mockTeamDeleteService struct{}

func (mockTeamDeleteService) CreateTeam(_ context.Context, _ string, _ github.NewTeam) (*github.Team, *github.Response, error) {
	return &github.Team{}, nil, nil
}

func (mockTeamDeleteService) DeleteTeamBySlug(_ context.Context, _, _ string) (*github.Response, error) {
	return nil, nil
}

func (mockTeamDeleteService) GetTeamBySlug(_ context.Context, _, _ string) (*github.Team, *github.Response, error) {
	return &github.Team{}, nil, nil
}

// TestDeleteTeamNoRequiredFlags tests the DeleteTeam command with no required flags.
func TestDeleteTeamNoRequiredFlags(t *testing.T) {
	c := teamcmd.DeleteTeamBySlugCmd(mockTeamDeleteService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

// TestDeleteTeamAllRequiredFlagsProvided tests the DeleteTeam command with all required flags provided.
func TestDeleteTeamAllRequiredFlagsProvided(t *testing.T) {
	c := teamcmd.DeleteTeamBySlugCmd(mockTeamDeleteService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestDeleteTeamWithInvalidFlags tests the DeleteTeam command with invalid flags.
func TestDeleteTeamWithInvalidFlags(t *testing.T) {
	c := teamcmd.DeleteTeamBySlugCmd(mockTeamDeleteService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}
