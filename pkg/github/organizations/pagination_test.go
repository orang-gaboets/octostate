package organizations

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
// the runaway guard trips.
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

func (s *stuckPager) ListMembers(context.Context, string, *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
	resp, err := s.next()
	return []*gh.User{{Login: gh.Ptr("alice")}}, resp, err
}

func (s *stuckPager) ListPendingOrgInvitations(context.Context, string, *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
	resp, err := s.next()
	return []*gh.Invitation{{ID: gh.Ptr(int64(1)), Login: gh.Ptr("alice")}}, resp, err
}

func (s *stuckPager) ListOrgInvitationTeams(context.Context, string, string, *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	resp, err := s.next()
	return []*gh.Team{{Slug: gh.Ptr("platform")}}, resp, err
}

func TestListFunctionsRejectNonAdvancingPagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		list func(*stuckPager) (int, error)
	}{
		{name: "ListMembers", list: func(s *stuckPager) (int, error) {
			got, err := ListMembers(context.Background(), ListMembersOptions{Service: s, OrgName: "acme", Role: MemberRoleMember})
			return len(got), err
		}},
		{name: "ListPendingInvitations", list: func(s *stuckPager) (int, error) {
			got, err := ListPendingInvitations(context.Background(), ListPendingInvitationsOptions{Service: s, OrgName: "acme"})
			return len(got), err
		}},
		{name: "ListInvitationTeams", list: func(s *stuckPager) (int, error) {
			got, err := ListInvitationTeams(context.Background(), ListInvitationTeamsOptions{Service: s, OrgName: "acme", InvitationID: 1})
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
