package collector

import (
	"context"
	"reflect"
	"sync/atomic"
	"testing"

	gh "github.com/google/go-github/v88/github"
	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	githubteams "github.com/orang-gaboets/octostate/pkg/github/teams"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

type inheritedTeamServiceStub struct {
	*teamServiceStub
	originMembers   map[string][]githubteams.TeamMemberWithOrigin
	listTeamsCalls  atomic.Int64
	originCalls     atomic.Int64
	repositoryCalls atomic.Int64
}

func (s *inheritedTeamServiceStub) ListTeamMembersBySlugWithOrigin(_ context.Context, _, slug string, _ *gh.ListOptions) ([]githubteams.TeamMemberWithOrigin, *gh.Response, error) {
	s.originCalls.Add(1)
	return s.originMembers[slug], &gh.Response{}, nil
}

func TestCollectTeamStateRetainsMembershipOrigin(t *testing.T) {
	const orgName = "acme"
	service := &inheritedTeamServiceStub{}
	service.teamServiceStub = &teamServiceStub{
		listTeamsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			service.listTeamsCalls.Add(1)
			return []*gh.Team{
				{Slug: githubpkg.Ptr("parent"), Name: githubpkg.Ptr("Parent"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}},
				{Slug: githubpkg.Ptr("child"), Name: githubpkg.Ptr("Child"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}},
			}, &gh.Response{}, nil
		},
		listTeamReposBySlugFunc: func(_ context.Context, _, _ string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
			service.repositoryCalls.Add(1)
			return nil, &gh.Response{}, nil
		},
	}
	service.originMembers = map[string][]githubteams.TeamMemberWithOrigin{
		"parent": {
			{Username: "alice", Role: githubteams.TeamMemberRoleMaintainer, Origin: githubpkg.TeamMembershipOriginDirect},
			{Username: "Bob", Role: githubteams.TeamMemberRoleMember, Origin: githubpkg.TeamMembershipOriginInherited},
			{Username: " BOB ", Role: githubteams.TeamMemberRoleMember, Origin: githubpkg.TeamMembershipOriginDirect},
		},
		"child": {{Username: "bob", Role: githubteams.TeamMemberRoleMember, Origin: githubpkg.TeamMembershipOriginDirect}},
	}

	_, members, _, err := collectTeamState(context.Background(), CollectOrganizationOptions{
		OrgName:     orgName,
		TeamService: service,
	}, defaultCollectorConcurrencyLimits)
	if err != nil {
		t.Fatalf("collectTeamState returned error: %v", err)
	}

	want := []state.TeamMember{
		{TeamSlug: "child", Username: "bob", Role: "member", Origin: githubpkg.TeamMembershipOriginDirect},
		{TeamSlug: "parent", Username: "alice", Role: "maintainer", Origin: githubpkg.TeamMembershipOriginDirect},
		{TeamSlug: "parent", Username: "BOB", Role: "member", Origin: githubpkg.TeamMembershipOriginDirect},
	}
	if !reflect.DeepEqual(members, want) {
		t.Fatalf("team members = %#v, want canonical origin-aware memberships %#v", members, want)
	}
	if got := service.listTeamsCalls.Load(); got != 1 {
		t.Errorf("ListTeams calls = %d, want 1", got)
	}
	if got := service.originCalls.Load(); got != 2 {
		t.Errorf("origin-aware member calls = %d, want 2", got)
	}
	if got := service.repositoryCalls.Load(); got != 2 {
		t.Errorf("team repository calls = %d, want 2", got)
	}
	totalCalls := service.listTeamsCalls.Load() + service.originCalls.Load() + service.repositoryCalls.Load()
	if totalCalls != 5 {
		t.Errorf("team service calls = %d, want 5 (2N+1)", totalCalls)
	}
}
