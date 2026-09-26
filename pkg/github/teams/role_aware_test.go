package teams

import (
	"context"
	"errors"
	"reflect"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type filteredTeamMemberService struct {
	Service
	roles []string
}

func (s *filteredTeamMemberService) ListTeamMembersBySlug(_ context.Context, _, _ string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
	s.roles = append(s.roles, opts.Role)
	switch opts.Role {
	case string(TeamMemberRoleMember):
		return []*gh.User{{Login: github.Ptr("member-user")}}, &gh.Response{}, nil
	case string(TeamMemberRoleMaintainer):
		return []*gh.User{{Login: github.Ptr("maintainer-user")}}, &gh.Response{}, nil
	default:
		return nil, nil, errors.New("unexpected role filter")
	}
}

type roleAwareTeamMemberService struct {
	Service
	pages      map[int][]TeamMember
	pageErrors map[int]error
	requested  []int
}

func TestListTeamMembersWithRolesOptionsValidate(t *testing.T) {
	service := &roleAwareTeamMemberService{}
	for _, test := range []struct {
		name    string
		options ListTeamMembersWithRolesBySlugOptions
		wantErr error
	}{
		{name: "nil service", options: ListTeamMembersWithRolesBySlugOptions{Org: existingTeam.Org, Slug: existingTeam.Slug}, wantErr: github.ErrNilService},
		{name: "missing organization", options: ListTeamMembersWithRolesBySlugOptions{Service: service, Slug: existingTeam.Slug}, wantErr: github.ErrMissingRequiredField},
		{name: "missing team slug", options: ListTeamMembersWithRolesBySlugOptions{Service: service, Org: existingTeam.Org}, wantErr: github.ErrMissingRequiredField},
		{name: "valid", options: ListTeamMembersWithRolesBySlugOptions{Service: service, Org: existingTeam.Org, Slug: existingTeam.Slug}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := test.options.Validate()
			if test.wantErr == nil {
				if err != nil {
					t.Fatalf("Validate returned error: %v", err)
				}
				return
			}
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("Validate error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func (s *roleAwareTeamMemberService) ListTeamMembersBySlugWithRoles(_ context.Context, _, _ string, opts *gh.ListOptions) ([]TeamMember, *gh.Response, error) {
	page := opts.Page
	s.requested = append(s.requested, page)
	if err := s.pageErrors[page]; err != nil {
		return nil, nil, err
	}
	nextPage := 0
	if page == 0 && s.pages[2] != nil {
		nextPage = 2
	}
	return s.pages[page], &gh.Response{NextPage: nextPage}, nil
}

func TestListTeamMembersWithRolesFallsBackToRoleFilters(t *testing.T) {
	service := &filteredTeamMemberService{}
	got, err := ListTeamMembersWithRolesBySlug(context.Background(), ListTeamMembersWithRolesBySlugOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if err != nil {
		t.Fatalf("ListTeamMembersWithRolesBySlug returned error: %v", err)
	}
	want := []TeamMember{
		{Username: "member-user", Role: TeamMemberRoleMember},
		{Username: "maintainer-user", Role: TeamMemberRoleMaintainer},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("members = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(service.roles, []string{"member", "maintainer"}) {
		t.Fatalf("role filters = %#v, want [member maintainer]", service.roles)
	}
}

func TestListTeamMembersWithRolesUsesRoleAwareServiceAndPaginates(t *testing.T) {
	service := &roleAwareTeamMemberService{
		pages: map[int][]TeamMember{
			0: {{Username: "alpha", Role: TeamMemberRoleMember}},
			2: {{Username: "zulu", Role: TeamMemberRoleMaintainer}},
		},
	}
	got, err := ListTeamMembersWithRolesBySlug(context.Background(), ListTeamMembersWithRolesBySlugOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if err != nil {
		t.Fatalf("ListTeamMembersWithRolesBySlug returned error: %v", err)
	}
	want := []TeamMember{
		{Username: "alpha", Role: TeamMemberRoleMember},
		{Username: "zulu", Role: TeamMemberRoleMaintainer},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("members = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(service.requested, []int{0, 2}) {
		t.Fatalf("requested pages = %#v, want [0 2]", service.requested)
	}
}

func TestListTeamMembersWithRolesRejectsMissingOrUnsupportedRole(t *testing.T) {
	for _, test := range []struct {
		name string
		role TeamMemberRole
	}{
		{name: "missing"},
		{name: "unsupported", role: TeamMemberRole("owner")},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &roleAwareTeamMemberService{pages: map[int][]TeamMember{
				0: {{Username: "alice", Role: test.role}},
			}}
			got, err := ListTeamMembersWithRolesBySlug(context.Background(), ListTeamMembersWithRolesBySlugOptions{
				Service: service,
				Org:     existingTeam.Org,
				Slug:    existingTeam.Slug,
			})
			if !errors.Is(err, github.ErrValidationFailed) {
				t.Fatalf("error = %v, want %v", err, github.ErrValidationFailed)
			}
			if got != nil {
				t.Fatalf("members = %#v, want nil on invalid role", got)
			}
		})
	}
}

func TestListTeamMembersWithRolesDoesNotReturnPartialResultsOnPageError(t *testing.T) {
	pageErr := errors.New("second page failed")
	service := &roleAwareTeamMemberService{
		pages: map[int][]TeamMember{
			0: {{Username: "alpha", Role: TeamMemberRoleMember}},
			2: {{Username: "zulu", Role: TeamMemberRoleMaintainer}},
		},
		pageErrors: map[int]error{2: pageErr},
	}
	got, err := ListTeamMembersWithRolesBySlug(context.Background(), ListTeamMembersWithRolesBySlugOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if !errors.Is(err, pageErr) {
		t.Fatalf("error = %v, want %v", err, pageErr)
	}
	if got != nil {
		t.Fatalf("members = %#v, want nil when a later page fails", got)
	}
}
