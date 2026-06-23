package permissions_test

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	permissionscmd "github.com/orang-gaboets/octostate/cmd/octostate/team/repo/permissions"
)

var errTeamRepoPermissionsCommandDependency = errors.New("team repo permissions command dependency failed")

type failingTeamRepoPermissionsService struct {
	auth.MockTeamsService
}

func (failingTeamRepoPermissionsService) ListTeamReposBySlug(context.Context, string, string, *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
	return nil, nil, errTeamRepoPermissionsCommandDependency
}

func (failingTeamRepoPermissionsService) AddTeamRepoBySlug(context.Context, string, string, string, string, *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
	return nil, errTeamRepoPermissionsCommandDependency
}

func (failingTeamRepoPermissionsService) RemoveTeamRepoBySlug(context.Context, string, string, string, string) (*gh.Response, error) {
	return nil, errTeamRepoPermissionsCommandDependency
}

func TestListTeamRepoPermissionsCmdPropagatesServiceError(t *testing.T) {
	cmd := permissionscmd.ListCmd(failingTeamRepoPermissionsService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s"})
	if err := cmd.Execute(); !errors.Is(err, errTeamRepoPermissionsCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestAddTeamRepoPermissionCmdPropagatesServiceError(t *testing.T) {
	cmd := permissionscmd.AddCmd(failingTeamRepoPermissionsService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "r", "--permission", "push"})
	if err := cmd.Execute(); !errors.Is(err, errTeamRepoPermissionsCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestRemoveTeamRepoPermissionCmdPropagatesServiceError(t *testing.T) {
	cmd := permissionscmd.RemoveCmd(failingTeamRepoPermissionsService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "r"})
	if err := cmd.Execute(); !errors.Is(err, errTeamRepoPermissionsCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}
