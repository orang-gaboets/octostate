package team_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	teamcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/team"
)

// mockTeamCreateService implements teams.Service for testing.
type mockTeamCreateService struct{}

func (mockTeamCreateService) CreateTeam(_ context.Context, _ string, _ github.NewTeam) (*github.Team, *github.Response, error) {
	return &github.Team{}, nil, nil
}

func (mockTeamCreateService) DeleteTeamBySlug(_ context.Context, _, _ string) (*github.Response, error) {
	return nil, nil
}

func (mockTeamCreateService) GetTeamBySlug(_ context.Context, _, _ string) (*github.Team, *github.Response, error) {
	return &github.Team{}, nil, nil
}

// TestCreateTeamNoRequiredFlags tests the CreateTeam command with no required flags.
func TestCreateTeamNoRequiredFlags(t *testing.T) {
	c := teamcmd.CreateTeamCmd(mockTeamCreateService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

// TestCreateTeamAllRequiredFlagsProvided tests the CreateTeam command with all required flags provided.
func TestCreateTeamAllRequiredFlagsProvided(t *testing.T) {
	c := teamcmd.CreateTeamCmd(mockTeamCreateService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestCreateTeamWithInvalidFlags tests the CreateTeam command with invalid flags.
func TestCreateTeamWithInvalidFlags(t *testing.T) {
	c := teamcmd.CreateTeamCmd(mockTeamCreateService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}
