package organization_test

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	organizationcmd "github.com/orang-gaboets/octostate/cmd/octostate/organization"
)

var errOrganizationCommandDependency = errors.New("organization command dependency failed")

type failingOrganizationService struct {
	auth.MockOrganizationService
}

func (failingOrganizationService) Get(context.Context, string) (*gh.Organization, *gh.Response, error) {
	return nil, nil, errOrganizationCommandDependency
}

func (failingOrganizationService) CreateOrgInvitation(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	return nil, nil, errOrganizationCommandDependency
}

func (failingOrganizationService) ListMembers(context.Context, string, *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
	return nil, nil, errOrganizationCommandDependency
}

func (failingOrganizationService) ListPendingOrgInvitations(context.Context, string, *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
	return nil, nil, errOrganizationCommandDependency
}

type failingOrganizationRepoService struct {
	auth.MockRepoService
}

func (failingOrganizationRepoService) ListByOrg(context.Context, string, *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
	return nil, nil, errOrganizationCommandDependency
}

type failingOrganizationTeamService struct {
	auth.MockTeamsService
}

func (failingOrganizationTeamService) ListTeams(context.Context, string, *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	return nil, nil, errOrganizationCommandDependency
}

type failingOrganizationUserService struct {
	auth.MockUserService
}

func (failingOrganizationUserService) Get(context.Context, string) (*gh.User, *gh.Response, error) {
	return nil, nil, errOrganizationCommandDependency
}

func TestGetOrgByNameCmdPropagatesServiceError(t *testing.T) {
	cmd := organizationcmd.GetOrgByNameCmd(failingOrganizationService{})
	cmd.SetArgs([]string{"--org", "o"})
	if err := cmd.Execute(); !errors.Is(err, errOrganizationCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestInviteCmdPropagatesInvitationServiceError(t *testing.T) {
	cmd := organizationcmd.InviteCmd(failingOrganizationService{}, nil)
	cmd.SetArgs([]string{"--org", "o", "--id", "123"})
	if err := cmd.Execute(); !errors.Is(err, errOrganizationCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestInviteCmdPropagatesUserLookupError(t *testing.T) {
	cmd := organizationcmd.InviteCmd(auth.MockOrganizationService{}, failingOrganizationUserService{})
	cmd.SetArgs([]string{"--org", "o", "--username", "u"})
	if err := cmd.Execute(); !errors.Is(err, errOrganizationCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestListOrgMembersCmdPropagatesServiceError(t *testing.T) {
	cmd := organizationcmd.ListOrgMembersCmd(failingOrganizationService{})
	cmd.SetArgs([]string{"--org", "o"})
	if err := cmd.Execute(); !errors.Is(err, errOrganizationCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestListOrgInvitationsCmdPropagatesServiceErrorFromSharedAudit(t *testing.T) {
	cmd := organizationcmd.ListOrgInvitationsCmd(failingOrganizationService{})
	cmd.SetArgs([]string{"--org", "o"})
	if err := cmd.Execute(); !errors.Is(err, errOrganizationCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestListOrgReposCmdPropagatesServiceError(t *testing.T) {
	cmd := organizationcmd.ListOrgReposCmd(failingOrganizationRepoService{})
	cmd.SetArgs([]string{"--org", "o"})
	if err := cmd.Execute(); !errors.Is(err, errOrganizationCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestListOrgTeamsCmdPropagatesServiceError(t *testing.T) {
	cmd := organizationcmd.ListOrgTeamsCmd(failingOrganizationTeamService{})
	cmd.SetArgs([]string{"--org", "o"})
	if err := cmd.Execute(); !errors.Is(err, errOrganizationCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}
