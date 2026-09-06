package collector

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	gh "github.com/google/go-github/v88/github"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
)

// teamCallCounter records how many GitHub reads team-state collection issues,
// split by endpoint, so the request cost can be characterized as team count
// grows without contacting GitHub.
type teamCallCounter struct {
	listTeams       atomic.Int64
	listMembers     atomic.Int64
	listMaintainers atomic.Int64
	listRepos       atomic.Int64
}

func (c *teamCallCounter) total() int64 {
	return c.listTeams.Load() + c.listMembers.Load() + c.listMaintainers.Load() + c.listRepos.Load()
}

func countingTeamService(t *testing.T, orgName string, teamCount int, counter *teamCallCounter) *teamServiceStub {
	t.Helper()

	return &teamServiceStub{
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
		listTeamMembersBySlugFunc: func(_ context.Context, _, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
			// Counted per role: without this the test would still pass if the
			// collector issued two member reads and never asked for
			// maintainers.
			switch {
			case opts != nil && opts.Role == "maintainer":
				counter.listMaintainers.Add(1)
			case opts != nil && opts.Role == "member":
				counter.listMembers.Add(1)
			default:
				// Returned rather than t.Fatalf: this runs on an orderedtasks
				// worker goroutine, and FailNow must only be called from the
				// test goroutine. collectTeamState propagates it back.
				return nil, nil, fmt.Errorf("unexpected team member role filter: %#v", opts)
			}
			return []*gh.User{{Login: githubpkg.Ptr(slug + "-user")}}, &gh.Response{}, nil
		},
		listTeamReposBySlugFunc: func(_ context.Context, _, slug string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
			counter.listRepos.Add(1)
			return []*gh.Repository{{
				Name:        githubpkg.Ptr(slug + "-repo"),
				Owner:       &gh.User{Login: githubpkg.Ptr(orgName)},
				Permissions: &gh.RepositoryPermissions{Push: githubpkg.Ptr(true)},
			}}, &gh.Response{}, nil
		},
	}
}

// Characterizes the current service-call shape: one ListTeams, then per team a
// member read, a maintainer read, and a repository-permission read.
//
// 3N + 1 is the baseline of this implementation, not a floor imposed by the
// GitHub REST API. The team-members endpoint defaults to role=all and returns
// each user's role, so a single unfiltered read could supply both roles and
// reduce the shape to 2N + 1; the currently vendored go-github does not surface
// that field, so the collector asks per role instead.
//
// These are service calls rather than HTTP requests: a paginated response makes
// one call issue several requests, so the real request total is at least this.
//
// This is the measurement #260 asks for. If a future change alters the call
// count, this test states the new number rather than letting it drift.
func TestCollectTeamStateRequestCountScalesLinearlyWithTeams(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"

	for _, teamCount := range []int{0, 1, 8, 32} {
		t.Run(fmt.Sprintf("teams=%d", teamCount), func(t *testing.T) {
			t.Parallel()

			counter := &teamCallCounter{}
			teamSvc := countingTeamService(t, orgName, teamCount, counter)

			_, _, _, err := collectTeamState(context.Background(), CollectOrganizationOptions{
				OrgName:     orgName,
				TeamService: teamSvc,
			}, defaultCollectorConcurrencyLimits)
			if err != nil {
				t.Fatalf("collectTeamState returned error: %v", err)
			}

			// One read per role today, because the vendored client cannot read
			// the role the endpoint already returns.
			wantMembers := int64(teamCount)
			wantMaintainers := int64(teamCount)
			wantRepos := int64(teamCount)
			wantTotal := int64(teamCount*3 + 1)

			if got := counter.listTeams.Load(); got != 1 {
				t.Errorf("ListTeams calls = %d, want 1", got)
			}
			if got := counter.listMembers.Load(); got != wantMembers {
				t.Errorf("member-role calls = %d, want %d", got, wantMembers)
			}
			if got := counter.listMaintainers.Load(); got != wantMaintainers {
				t.Errorf("maintainer-role calls = %d, want %d", got, wantMaintainers)
			}
			if got := counter.listRepos.Load(); got != wantRepos {
				t.Errorf("ListTeamReposBySlug calls = %d, want %d", got, wantRepos)
			}
			if got := counter.total(); got != wantTotal {
				t.Errorf("total team service calls = %d, want %d (3N+1 today)", got, wantTotal)
			}
		})
	}
}

// Teams without a usable slug are skipped entirely rather than costing three
// reads each, so the request count tracks eligible teams rather than raw list
// length.
func TestCollectTeamStateSkipsIneligibleTeamsWithoutRequests(t *testing.T) {
	t.Parallel()

	const orgName = "orang-gaboets"
	counter := &teamCallCounter{}

	teamSvc := &teamServiceStub{
		listTeamsFunc: func(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
			counter.listTeams.Add(1)
			return []*gh.Team{
				{Slug: githubpkg.Ptr("real"), Name: githubpkg.Ptr("real"), Organization: &gh.Organization{Login: githubpkg.Ptr(orgName)}},
				nil,
				{Slug: githubpkg.Ptr(""), Name: githubpkg.Ptr("no-slug")},
			}, &gh.Response{}, nil
		},
		listTeamMembersBySlugFunc: func(_ context.Context, _, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
			// Counted per role: without this the test would still pass if the
			// collector issued two member reads and never asked for
			// maintainers.
			switch {
			case opts != nil && opts.Role == "maintainer":
				counter.listMaintainers.Add(1)
			case opts != nil && opts.Role == "member":
				counter.listMembers.Add(1)
			default:
				// Returned rather than t.Fatalf: this runs on an orderedtasks
				// worker goroutine, and FailNow must only be called from the
				// test goroutine. collectTeamState propagates it back.
				return nil, nil, fmt.Errorf("unexpected team member role filter: %#v", opts)
			}
			return []*gh.User{{Login: githubpkg.Ptr(slug + "-user")}}, &gh.Response{}, nil
		},
		listTeamReposBySlugFunc: func(_ context.Context, _, _ string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
			counter.listRepos.Add(1)
			return nil, &gh.Response{}, nil
		},
	}

	if _, _, _, err := collectTeamState(context.Background(), CollectOrganizationOptions{
		OrgName:     orgName,
		TeamService: teamSvc,
	}, defaultCollectorConcurrencyLimits); err != nil {
		t.Fatal(err)
	}

	if got := counter.total(); got != 4 {
		t.Fatalf("total service calls = %d, want 4 (1 list + 3 for the single eligible team)", got)
	}
}
