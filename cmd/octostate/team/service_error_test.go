package team_test

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	teamcmd "github.com/orang-gaboets/octostate/cmd/octostate/team"
)

var errTeamCommandDependency = errors.New("team command dependency failed")

type failingTeamService struct {
	auth.MockTeamsService
}

func (failingTeamService) CreateTeam(context.Context, string, gh.NewTeam) (*gh.Team, *gh.Response, error) {
	return nil, nil, errTeamCommandDependency
}

func (failingTeamService) GetTeamBySlug(context.Context, string, string) (*gh.Team, *gh.Response, error) {
	return nil, nil, errTeamCommandDependency
}

func (failingTeamService) EditTeamBySlug(context.Context, string, string, gh.NewTeam, bool) (*gh.Team, *gh.Response, error) {
	return nil, nil, errTeamCommandDependency
}

func (failingTeamService) DeleteTeamBySlug(context.Context, string, string) (*gh.Response, error) {
	return nil, errTeamCommandDependency
}

func TestCreateTeamCmdPropagatesServiceError(t *testing.T) {
	cmd := teamcmd.CreateTeamCmd(failingTeamService{})
	cmd.SetArgs([]string{"--org", "o", "--name", "n"})
	if err := cmd.Execute(); !errors.Is(err, errTeamCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestGetTeamBySlugCmdPropagatesServiceError(t *testing.T) {
	cmd := teamcmd.GetTeamBySlugCmd(failingTeamService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s"})
	if err := cmd.Execute(); !errors.Is(err, errTeamCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestEditTeamCmdPropagatesServiceError(t *testing.T) {
	cmd := teamcmd.EditTeamCmd(failingTeamService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s", "--desc", "new description"})
	if err := cmd.Execute(); !errors.Is(err, errTeamCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestDeleteTeamBySlugCmdPropagatesServiceError(t *testing.T) {
	cmd := teamcmd.DeleteTeamBySlugCmd(failingTeamService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s", "--yes"})
	if err := cmd.Execute(); !errors.Is(err, errTeamCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}
