package teams

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"

	"github.com/orang-gaboets/octostate/pkg/github"
)

var errRunaway = errors.New("pagination did not stop")

// stuckPager always reports page 2 as next, so the second read repeats a
// page. A correct loop stops there; a loop that trusts NextPage runs until
// the runaway guard trips. It implements the optional role and inheritance
// listers so every team pagination loop is reachable.
type stuckPager struct {
	Service
	calls int
}

func (s *stuckPager) next() (*gh.Response, error) {
	s.calls++
	if s.calls > 10 {
		return nil, errRunaway
	}
	return &gh.Response{NextPage: 2}, nil
}

func (s *stuckPager) ListTeams(context.Context, string, *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	resp, err := s.next()
	return []*gh.Team{{Slug: gh.Ptr("platform")}}, resp, err
}

func (s *stuckPager) ListTeamReposBySlug(context.Context, string, string, *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
	resp, err := s.next()
	return []*gh.Repository{{Name: gh.Ptr("repo"), Owner: &gh.User{Login: gh.Ptr("acme")}, Permissions: &gh.RepositoryPermissions{Pull: gh.Ptr(true)}}}, resp, err
}

func (s *stuckPager) ListTeamMembersBySlug(context.Context, string, string, *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
	resp, err := s.next()
	return []*gh.User{{Login: gh.Ptr("alice")}}, resp, err
}

func (s *stuckPager) ListTeamMembersBySlugWithRoles(context.Context, string, string, *gh.ListOptions) ([]TeamMember, *gh.Response, error) {
	resp, err := s.next()
	return []TeamMember{{Username: "alice", Role: TeamMemberRoleMember}}, resp, err
}

func (s *stuckPager) ListTeamMembersBySlugWithInheritance(context.Context, string, string, *gh.ListOptions) ([]TeamMemberWithInheritance, *gh.Response, error) {
	resp, err := s.next()
	return []TeamMemberWithInheritance{{Username: "alice", Role: TeamMemberRoleMember}}, resp, err
}

func TestListFunctionsRejectNonAdvancingPagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		list func(*stuckPager) (int, error)
	}{
		{name: "ListTeams", list: func(s *stuckPager) (int, error) {
			got, err := ListTeams(context.Background(), ListTeamsOptions{Service: s, Org: "acme"})
			return len(got), err
		}},
		{name: "ListTeamRepoPermissionsBySlug", list: func(s *stuckPager) (int, error) {
			got, err := ListTeamRepoPermissionsBySlug(context.Background(), ListTeamRepoPermissionsBySlugOptions{Service: s, Org: "acme", Slug: "platform"})
			return len(got), err
		}},
		{name: "ListTeamMembersBySlug", list: func(s *stuckPager) (int, error) {
			got, err := ListTeamMembersBySlug(context.Background(), ListTeamMembersBySlugOptions{Service: s, Org: "acme", Slug: "platform", Role: TeamMemberRoleMember})
			return len(got), err
		}},
		{name: "ListTeamMembersBySlugWithRoles", list: func(s *stuckPager) (int, error) {
			got, err := ListTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{Service: s, Org: "acme", Slug: "platform"})
			return len(got), err
		}},
		{name: "ListDirectTeamMembersBySlugWithRoles", list: func(s *stuckPager) (int, error) {
			got, err := ListDirectTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{Service: s, Org: "acme", Slug: "platform"})
			return len(got), err
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc := &stuckPager{}
			n, err := tt.list(svc)
			if !errors.Is(err, github.ErrValidationFailed) {
				t.Fatalf("error = %v, want %v", err, github.ErrValidationFailed)
			}
			if n != 0 || svc.calls != 2 {
				t.Fatalf("got %d results after %d calls, want none after 2", n, svc.calls)
			}
		})
	}
}
