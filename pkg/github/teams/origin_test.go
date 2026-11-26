package teams

import (
	"context"
	"errors"
	"reflect"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/github"
)

func TestListTeamMembersWithOriginMarksRoleAwareFallbackUnknown(t *testing.T) {
	service := &roleAwareTeamMemberService{pages: map[int][]TeamMember{
		0: {{Username: "alice", Role: TeamMemberRoleMember}},
		2: {{Username: "bob", Role: TeamMemberRoleMaintainer}},
	}}

	got, err := ListTeamMembersBySlugWithOrigin(context.Background(), ListTeamMembersBySlugWithRolesOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if err != nil {
		t.Fatalf("ListTeamMembersBySlugWithOrigin returned error: %v", err)
	}
	want := []TeamMemberWithOrigin{
		{Username: "alice", Role: TeamMemberRoleMember, Origin: github.TeamMembershipOriginUnknown},
		{Username: "bob", Role: TeamMemberRoleMaintainer, Origin: github.TeamMembershipOriginUnknown},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("members = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(service.requested, []int{0, 2}) {
		t.Fatalf("requested pages = %#v, want [0 2]", service.requested)
	}
}

type originListerTestService struct {
	Service
	pages      map[int][]TeamMemberWithOrigin
	pageErrors map[int]error
	requested  []int
}

func (s *originListerTestService) ListTeamMembersBySlugWithOrigin(_ context.Context, _, _ string, opts *gh.ListOptions) ([]TeamMemberWithOrigin, *gh.Response, error) {
	page := opts.Page
	s.requested = append(s.requested, page)
	if err := s.pageErrors[page]; err != nil {
		return nil, nil, err
	}
	nextPage := 0
	if page == 0 && (s.pages[2] != nil || s.pageErrors[2] != nil) {
		nextPage = 2
	}
	return s.pages[page], &gh.Response{NextPage: nextPage}, nil
}

func TestListDirectTeamMembersRejectsUnknownOriginWithoutPartialResults(t *testing.T) {
	service := &originListerTestService{pages: map[int][]TeamMemberWithOrigin{
		0: {
			{Username: "alice", Role: TeamMemberRoleMember, Origin: github.TeamMembershipOriginDirect},
			{Username: "bob", Role: TeamMemberRoleMember, Origin: github.TeamMembershipOriginUnknown},
		},
	}}

	got, err := ListDirectTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("error = %v, want %v", err, github.ErrValidationFailed)
	}
	if got != nil {
		t.Fatalf("members = %#v, want nil for unknown-origin result", got)
	}
}
