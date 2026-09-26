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
	roleMembers      map[string][]githubteams.TeamMember
	inheritedMembers map[string][]githubteams.TeamMemberWithInheritance
	listTeamsCalls   atomic.Int64
	roleMemberCalls  atomic.Int64
	inheritedCalls   atomic.Int64
	repositoryCalls  atomic.Int64
}

func (s *inheritedTeamServiceStub) ListTeamMembersBySlugWithRoles(_ context.Context, _, slug string, _ *gh.ListOptions) ([]githubteams.TeamMember, *gh.Response, error) {
	s.roleMemberCalls.Add(1)
	return s.roleMembers[slug], &gh.Response{}, nil
}

func (s *inheritedTeamServiceStub) ListTeamMembersBySlugWithInheritance(_ context.Context, _, slug string, _ *gh.ListOptions) ([]githubteams.TeamMemberWithInheritance, *gh.Response, error) {
	s.inheritedCalls.Add(1)
	return s.inheritedMembers[slug], &gh.Response{}, nil
}

func TestCollectTeamStateKeepsDirectMembershipsOnly(t *testing.T) {
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
	service.roleMembers = map[string][]githubteams.TeamMember{
		"parent": {
			{Username: "alice", Role: githubteams.TeamMemberRoleMaintainer},
			{Username: "bob", Role: githubteams.TeamMemberRoleMember},
		},
		"child": {{Username: "bob", Role: githubteams.TeamMemberRoleMember}},
	}
	service.inheritedMembers = map[string][]githubteams.TeamMemberWithInheritance{
		"parent": {
			{Username: "alice", Role: githubteams.TeamMemberRoleMaintainer, Inherited: false},
			{Username: "bob", Role: githubteams.TeamMemberRoleMember, Inherited: true},
		},
		"child": {{Username: "bob", Role: githubteams.TeamMemberRoleMember, Inherited: false}},
	}

	_, members, _, err := collectTeamState(context.Background(), CollectOrganizationOptions{
		OrgName:     orgName,
		TeamService: service,
	}, defaultCollectorConcurrencyLimits)
	if err != nil {
		t.Fatalf("collectTeamState returned error: %v", err)
	}

	want := []state.TeamMember{
		{TeamSlug: "parent", Username: "alice", Role: "maintainer"},
		{TeamSlug: "child", Username: "bob", Role: "member"},
	}
	if !reflect.DeepEqual(members, want) {
		t.Fatalf("team members = %#v, want direct memberships %#v", members, want)
	}
	if got := service.listTeamsCalls.Load(); got != 1 {
		t.Errorf("ListTeams calls = %d, want 1", got)
	}
	if got := service.inheritedCalls.Load(); got != 2 {
		t.Errorf("inheritance-aware member calls = %d, want 2", got)
	}
	if got := service.roleMemberCalls.Load(); got != 0 {
		t.Errorf("role-only member calls = %d, want 0 when inheritance metadata is available", got)
	}
	if got := service.repositoryCalls.Load(); got != 2 {
		t.Errorf("team repository calls = %d, want 2", got)
	}
	totalCalls := service.listTeamsCalls.Load() + service.inheritedCalls.Load() + service.repositoryCalls.Load()
	if totalCalls != 5 {
		t.Errorf("team service calls = %d, want 5 (2N+1)", totalCalls)
	}
}
