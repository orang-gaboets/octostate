package teams

import (
	"context"
	"errors"
	"reflect"
	"strings"
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

type inheritanceAwareTeamMemberService struct {
	Service
	inheritedPages map[int][]TeamMemberWithInheritance
	requested      []int
	roleAwareCalls int
	pageErrors     map[int]error
}

func (s *inheritanceAwareTeamMemberService) ListTeamMembersBySlugWithRoles(_ context.Context, _, _ string, _ *gh.ListOptions) ([]TeamMember, *gh.Response, error) {
	s.roleAwareCalls++
	return []TeamMember{{Username: "fallback", Role: TeamMemberRoleMember}}, &gh.Response{}, nil
}

func (s *inheritanceAwareTeamMemberService) ListTeamMembersBySlugWithInheritance(_ context.Context, _, _ string, opts *gh.ListOptions) ([]TeamMemberWithInheritance, *gh.Response, error) {
	page := opts.Page
	s.requested = append(s.requested, page)
	if err := s.pageErrors[page]; err != nil {
		return nil, nil, err
	}
	nextPage := 0
	if page == 0 && (s.inheritedPages[2] != nil || s.pageErrors[2] != nil) {
		nextPage = 2
	}
	return s.inheritedPages[page], &gh.Response{NextPage: nextPage}, nil
}

func TestListTeamMembersWithRolesOptionsValidate(t *testing.T) {
	service := &roleAwareTeamMemberService{}
	for _, test := range []struct {
		name    string
		options ListTeamMembersBySlugWithRolesOptions
		wantErr error
	}{
		{name: "nil service", options: ListTeamMembersBySlugWithRolesOptions{Org: existingTeam.Org, Slug: existingTeam.Slug}, wantErr: github.ErrNilService},
		{name: "missing organization", options: ListTeamMembersBySlugWithRolesOptions{Service: service, Slug: existingTeam.Slug}, wantErr: github.ErrMissingRequiredField},
		{name: "missing team slug", options: ListTeamMembersBySlugWithRolesOptions{Service: service, Org: existingTeam.Org}, wantErr: github.ErrMissingRequiredField},
		{name: "valid", options: ListTeamMembersBySlugWithRolesOptions{Service: service, Org: existingTeam.Org, Slug: existingTeam.Slug}},
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
	got, err := ListTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if err != nil {
		t.Fatalf("ListTeamMembersBySlugWithRoles returned error: %v", err)
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

func TestListDirectTeamMembersPrefersInheritanceAwareLister(t *testing.T) {
	service := &inheritanceAwareTeamMemberService{
		inheritedPages: map[int][]TeamMemberWithInheritance{
			0: {
				{Username: "alice", Role: TeamMemberRoleMaintainer, Inherited: false},
				{Username: "bob", Role: TeamMemberRoleMember, Inherited: true},
			},
			2: {
				{Username: "bob", Role: TeamMemberRoleMember, Inherited: false},
				{Username: "carol", Role: TeamMemberRoleMember, Inherited: false},
			},
		},
	}
	got, err := ListDirectTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if err != nil {
		t.Fatalf("ListDirectTeamMembersBySlugWithRoles returned error: %v", err)
	}
	want := []TeamMember{
		{Username: "alice", Role: TeamMemberRoleMaintainer},
		{Username: "bob", Role: TeamMemberRoleMember},
		{Username: "carol", Role: TeamMemberRoleMember},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("direct members = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(service.requested, []int{0, 2}) {
		t.Fatalf("inheritance-aware pages = %#v, want [0 2]", service.requested)
	}
	if service.roleAwareCalls != 0 {
		t.Fatalf("role-aware fallback calls = %d, want 0", service.roleAwareCalls)
	}
}

func TestListDirectTeamMembersRejectsInvalidRoleEvenOnInheritedRow(t *testing.T) {
	for _, test := range []struct {
		name string
		role TeamMemberRole
	}{
		{name: "missing"},
		{name: "unsupported", role: TeamMemberRole("owner")},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &inheritanceAwareTeamMemberService{
				inheritedPages: map[int][]TeamMemberWithInheritance{
					0: {{Username: "bob", Role: test.role, Inherited: true}},
				},
			}
			got, err := ListDirectTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
				Service: service,
				Org:     existingTeam.Org,
				Slug:    existingTeam.Slug,
			})
			assertInvalidTeamMemberRoleError(t, err)
			if got != nil {
				t.Fatalf("members = %#v, want nil for invalid inherited role", got)
			}
		})
	}
}

func TestListDirectTeamMembersDoesNotReturnPartialResultsOnPageError(t *testing.T) {
	pageErr := errors.New("second page failed")
	service := &inheritanceAwareTeamMemberService{
		inheritedPages: map[int][]TeamMemberWithInheritance{
			0: {{Username: "alice", Role: TeamMemberRoleMember, Inherited: false}},
		},
		pageErrors: map[int]error{2: pageErr},
	}
	got, err := ListDirectTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if !errors.Is(err, pageErr) {
		t.Fatalf("error = %v, want wrapped %v", err, pageErr)
	}
	if got != nil {
		t.Fatalf("members = %#v, want nil on page error", got)
	}
}

func TestListDirectTeamMembersUsesRoleAwareFallback(t *testing.T) {
	service := &roleAwareTeamMemberService{
		pages: map[int][]TeamMember{
			0: {{Username: "alpha", Role: TeamMemberRoleMember}},
			2: {{Username: "zulu", Role: TeamMemberRoleMaintainer}},
		},
	}
	got, err := ListDirectTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if err != nil {
		t.Fatalf("ListDirectTeamMembersBySlugWithRoles returned error: %v", err)
	}
	want := []TeamMember{
		{Username: "alpha", Role: TeamMemberRoleMember},
		{Username: "zulu", Role: TeamMemberRoleMaintainer},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("members = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(service.requested, []int{0, 2}) {
		t.Fatalf("role-aware pages = %#v, want [0 2]", service.requested)
	}
}

func TestListDirectTeamMembersUsesLegacyRoleFilterFallback(t *testing.T) {
	service := &filteredTeamMemberService{}
	got, err := ListDirectTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if err != nil {
		t.Fatalf("ListDirectTeamMembersBySlugWithRoles returned error: %v", err)
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
	got, err := ListTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
		Service: service,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	})
	if err != nil {
		t.Fatalf("ListTeamMembersBySlugWithRoles returned error: %v", err)
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
			got, err := ListTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
				Service: service,
				Org:     existingTeam.Org,
				Slug:    existingTeam.Slug,
			})
			assertInvalidTeamMemberRoleError(t, err)
			if got != nil {
				t.Fatalf("members = %#v, want nil on invalid role", got)
			}
		})
	}
}

func assertInvalidTeamMemberRoleError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("error = %v, want %v", err, github.ErrValidationFailed)
	}
	for _, want := range []string{
		existingTeam.Org,
		existingTeam.Slug,
		"GitHub did not return a recognized role value",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
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
	got, err := ListTeamMembersBySlugWithRoles(context.Background(), ListTeamMembersBySlugWithRolesOptions{
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
