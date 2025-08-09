package team_test

import (
	"context"
	"testing"

	"github.com/google/go-github/v55/github"
	teamcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/team"
)

// mockTeamGetService implements teams.Service for testing.
type mockTeamGetService struct{}

func (mockTeamGetService) CreateTeam(_ context.Context, _ string, _ github.NewTeam) (*github.Team, *github.Response, error) {
	return &github.Team{}, nil, nil
}

func (mockTeamGetService) DeleteTeamBySlug(_ context.Context, _, _ string) (*github.Response, error) {
	return nil, nil
}

func (mockTeamGetService) GetTeamBySlug(_ context.Context, _, _ string) (*github.Team, *github.Response, error) {
	return &github.Team{}, nil, nil
}

// TestGetTeamBySlugNoRequiredFlags tests the GetTeamBySlug command with no required flags.
func TestGetTeamBySlugNoRequiredFlags(t *testing.T) {
	c := teamcmd.GetTeamBySlugCmd(mockTeamGetService{})
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

// TestGetTeamBySlugAllRequiredFlagsProvided tests the GetTeamBySlug command with all required flags provided.
func TestGetTeamBySlugAllRequiredFlagsProvided(t *testing.T) {
	c := teamcmd.GetTeamBySlugCmd(mockTeamGetService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestGetTeamBySlugWithInvalidFlags tests the GetTeamBySlug command with invalid flags.
func TestGetTeamBySlugWithInvalidFlags(t *testing.T) {
	c := teamcmd.GetTeamBySlugCmd(mockTeamGetService{})
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

// TestGetTeamBySlugWithMissingSlug tests the GetTeamBySlug command with missing slug flag.
func TestGetTeamBySlugWithMissingSlug(t *testing.T) {
	c := teamcmd.GetTeamBySlugCmd(mockTeamGetService{})
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing slug flag")
	}
}
