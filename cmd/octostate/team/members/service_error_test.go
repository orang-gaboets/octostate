package members_test

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	memberscmd "github.com/orang-gaboets/octostate/cmd/octostate/team/members"
)

var errTeamMembersCommandDependency = errors.New("team members command dependency failed")

type failingTeamMembersService struct {
	auth.MockTeamsService
}

func (failingTeamMembersService) ListTeamMembersBySlug(context.Context, string, string, *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
	return nil, nil, errTeamMembersCommandDependency
}

func (failingTeamMembersService) AddTeamMembershipBySlug(context.Context, string, string, string, *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
	return nil, nil, errTeamMembersCommandDependency
}

func (failingTeamMembersService) RemoveTeamMembershipBySlug(context.Context, string, string, string) (*gh.Response, error) {
	return nil, errTeamMembersCommandDependency
}

func TestListTeamMembersCmdPropagatesServiceError(t *testing.T) {
	cmd := memberscmd.ListCmd(failingTeamMembersService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s"})
	if err := cmd.Execute(); !errors.Is(err, errTeamMembersCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestAddTeamMemberCmdPropagatesServiceError(t *testing.T) {
	cmd := memberscmd.AddCmd(failingTeamMembersService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "u"})
	if err := cmd.Execute(); !errors.Is(err, errTeamMembersCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestRemoveTeamMemberCmdPropagatesServiceError(t *testing.T) {
	cmd := memberscmd.RemoveCmd(failingTeamMembersService{})
	cmd.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "u"})
	if err := cmd.Execute(); !errors.Is(err, errTeamMembersCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}
