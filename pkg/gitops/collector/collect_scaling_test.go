package collector

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"

	gh "github.com/google/go-github/v88/github"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	githubteams "github.com/orang-gaboets/octostate/pkg/github/teams"
)

// teamCallCounter records how many GitHub reads team-state collection issues,
// split by endpoint, so the request cost can be characterized as team count
// grows without contacting GitHub.
type teamCallCounter struct {
	listTeams   atomic.Int64
	listMembers atomic.Int64
	listRepos   atomic.Int64
}

func (c *teamCallCounter) total() int64 {
	return c.listTeams.Load() + c.listMembers.Load() + c.listRepos.Load()
}

type roleAwareTeamServiceStub struct {
	*teamServiceStub
	listTeamMembersWithRolesFunc func(context.Context, string, string, *gh.ListOptions) ([]githubteams.TeamMember, *gh.Response, error)
}

func (s *roleAwareTeamServiceStub) ListTeamMembersBySlugWithRoles(ctx context.Context, org, slug string, opts *gh.ListOptions) ([]githubteams.TeamMember, *gh.Response, error) {
	return s.listTeamMembersWithRolesFunc(ctx, org, slug, opts)
}

func countingTeamService(orgName string, teamCount int, counter *teamCallCounter) *roleAwareTeamServiceStub {
	return &roleAwareTeamServiceStub{
		teamServiceStub: &teamServiceStub{
			listTeamsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
				counter.listTeams.Add(1)
				teams := make([]*gh.Team, 0, teamCount)
				for i := 1; i <= teamCount; i++ {
					slug := fmt.Sprintf("team-%03d", i)
					teams = append(teams, &gh.Team{
						Slug:         githubpkg.Ptr(slug),
						Name:         githubpkg.Ptr(slug),
						Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)},
					})
				}
				return teams, &gh.Response{}, nil
			},
			listTeamReposBySlugFunc: func(_ context.Context, _, slug string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
				counter.listRepos.Add(1)
				return []*gh.Repository{{
					Name:        githubpkg.Ptr(slug + "-repo"),
					Owner:       &gh.User{Login: githubpkg.Ptr(orgName)},
					Permissions: &gh.RepositoryPermissions{Push: githubpkg.Ptr(true)},
				}}, &gh.Response{}, nil
			},
		},
		listTeamMembersWithRolesFunc: func(_ context.Context, _, slug string, _ *gh.ListOptions) ([]githubteams.TeamMember, *gh.Response, error) {
			counter.listMembers.Add(1)
			return []githubteams.TeamMember{
				{Username: slug + "-member", Role: githubteams.TeamMemberRoleMember},
				{Username: slug + "-maintainer", Role: githubteams.TeamMemberRoleMaintainer},
			}, &gh.Response{}, nil
		},
	}
}

// Characterizes the optimized service-call shape: one ListTeams, then per team
// a role-aware member read and a repository-permission read.
//
// The measured baseline from PR #275 was 3N + 1. GitHub's role=all response now
// supplies both member and maintainer roles in one service call, reducing the
// collector to 2N + 1 without dropping normalized team state.
//
// These are service calls rather than HTTP requests: a paginated response makes
// one call issue several requests, so the real request total is at least this.
//
// This is the measurement #260 asks for. If a future change alters the call
// count, this test states the new number rather than letting it drift.
func TestCollectTeamStateRoleAwareRequestCountScalesLinearlyWithTeams(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"

	for _, teamCount := range []int{0, 1, 8, 32} {
		t.Run(fmt.Sprintf("teams=%d", teamCount), func(t *testing.T) {
			t.Parallel()

			counter := &teamCallCounter{}
			teamSvc := countingTeamService(orgName, teamCount, counter)

			collectedTeams, members, repoPermissions, err := collectTeamState(context.Background(), CollectOrganizationOptions{
				OrgName:     orgName,
				TeamService: teamSvc,
			}, defaultCollectorConcurrencyLimits)
			if err != nil {
				t.Fatalf("collectTeamState returned error: %v", err)
			}

			wantMembers := int64(teamCount)
			wantRepos := int64(teamCount)
			wantTotal := int64(teamCount*2 + 1)

			if got := counter.listTeams.Load(); got != 1 {
				t.Errorf("ListTeams calls = %d, want 1", got)
			}
			if got := counter.listMembers.Load(); got != wantMembers {
				t.Errorf("member-role calls = %d, want %d", got, wantMembers)
			}
			if got := counter.listRepos.Load(); got != wantRepos {
				t.Errorf("ListTeamReposBySlug calls = %d, want %d", got, wantRepos)
			}
			if got := counter.total(); got != wantTotal {
				t.Errorf("total team service calls = %d, want %d (2N+1)", got, wantTotal)
			}
			if got, want := len(members), teamCount*2; got != want {
				t.Errorf("normalized team members = %d, want %d", got, want)
			}
			if got, want := len(repoPermissions), teamCount; got != want {
				t.Errorf("normalized team repository permissions = %d, want %d", got, want)
			}
			if got, want := len(collectedTeams), teamCount; got != want {
				t.Errorf("normalized teams = %d, want %d", got, want)
			}
			memberRoles := map[string]int{"member": 0, "maintainer": 0}
			for _, member := range members {
				switch {
				case strings.HasSuffix(member.Username, "-member") && member.Role == "member":
					memberRoles["member"]++
				case strings.HasSuffix(member.Username, "-maintainer") && member.Role == "maintainer":
					memberRoles["maintainer"]++
				default:
					t.Errorf("normalized member = %#v, want role to match username", member)
				}
			}
			for role, got := range memberRoles {
				if got != teamCount {
					t.Errorf("normalized %s members = %d, want %d", role, got, teamCount)
				}
			}
		})
	}
}

// Teams without a usable slug are skipped entirely rather than costing two
// detail reads each. Counts therefore track eligible teams rather than raw
// list length.
func TestCollectTeamStateSkipsIneligibleTeamsWithoutRequests(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	counter := &teamCallCounter{}

	teamSvc := &roleAwareTeamServiceStub{
		teamServiceStub: &teamServiceStub{
			listTeamsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
				counter.listTeams.Add(1)
				return []*gh.Team{
					{Slug: githubpkg.Ptr("real"), Name: githubpkg.Ptr("real"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}},
					nil,
					{Slug: githubpkg.Ptr(""), Name: githubpkg.Ptr("no-slug")},
				}, &gh.Response{}, nil
			},
			listTeamReposBySlugFunc: func(_ context.Context, _, _ string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
				counter.listRepos.Add(1)
				return nil, &gh.Response{}, nil
			},
		},
		listTeamMembersWithRolesFunc: func(_ context.Context, _, slug string, _ *gh.ListOptions) ([]githubteams.TeamMember, *gh.Response, error) {
			counter.listMembers.Add(1)
			return []githubteams.TeamMember{{Username: slug + "-user", Role: githubteams.TeamMemberRoleMember}}, &gh.Response{}, nil
		},
	}

	if _, _, _, err := collectTeamState(context.Background(), CollectOrganizationOptions{
		OrgName:     orgName,
		TeamService: teamSvc,
	}, defaultCollectorConcurrencyLimits); err != nil {
		t.Fatal(err)
	}

	if got := counter.total(); got != 3 {
		t.Fatalf("total service calls = %d, want 3 (1 list + 2 for the single eligible team)", got)
	}
}
