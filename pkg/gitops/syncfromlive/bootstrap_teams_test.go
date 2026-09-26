package syncfromlive

import (
	"reflect"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBootstrapTeamMembersGroupsByNormalizedSlug(t *testing.T) {
	t.Parallel()

	got, err := bootstrapTeamMembers(
		[]state.Team{{Slug: "platform"}},
		[]state.TeamMember{{TeamSlug: " Platform ", Username: " alice ", Role: " maintainer "}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []config.TeamMemberSpec{{Username: "alice", Role: "maintainer"}}
	if len(got["platform"]) != 1 || got["platform"][0] != want[0] {
		t.Fatalf("unexpected grouped team members: %#v", got)
	}
}

func TestBootstrapExcludesInheritedOnlyParentMembership(t *testing.T) {
	t.Parallel()

	got, err := BuildBootstrapConfig(BootstrapOptions{
		Actual: &state.OrganizationState{
			Organization: "acme",
			Members:      []state.OrganizationMember{{Username: "bob", Role: "member"}},
			Teams: []state.Team{
				{Slug: "parent", Name: "Parent", Privacy: "closed"},
				{Slug: "child", Name: "Child", Privacy: "closed", ParentSlug: "parent"},
			},
			// The collector omits bob's inherited-only parent membership and keeps
			// the direct membership on the child team.
			TeamMembers: []state.TeamMember{{TeamSlug: "child", Username: "bob", Role: "member"}},
		},
	})
	if err != nil {
		t.Fatalf("BuildBootstrapConfig returned error: %v", err)
	}

	teamsBySlug := make(map[string]config.TeamSpec, len(got.Teams))
	for _, team := range got.Teams {
		teamsBySlug[team.Slug] = team
	}
	if members := teamsBySlug["parent"].Members; len(members) != 0 {
		t.Fatalf("parent team members = %#v, want no inherited-only membership", members)
	}
	wantChildMembers := []config.TeamMemberSpec{{Username: "bob", Role: "member"}}
	if members := teamsBySlug["child"].Members; !reflect.DeepEqual(members, wantChildMembers) {
		t.Fatalf("child team members = %#v, want %#v", members, wantChildMembers)
	}
}

func TestBootstrapTeamRepositoryPermissionsGroupsByNormalizedSlug(t *testing.T) {
	t.Parallel()

	got, err := bootstrapTeamRepositoryPermissions(
		"orang-gaboets",
		[]state.Team{{Slug: "platform"}},
		[]state.TeamRepositoryPermission{{TeamSlug: " Platform ", Owner: " orang-gaboets ", Name: " octostate ", Permission: " push "}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []config.TeamRepositorySpec{{Name: "octostate", Permission: "push"}}
	if len(got["platform"]) != 1 || got["platform"][0] != want[0] {
		t.Fatalf("unexpected grouped team repository permissions: %#v", got)
	}
}

func TestBootstrapTeamsInitializesEmptyNestedSlices(t *testing.T) {
	t.Parallel()

	got := bootstrapTeams(
		[]state.Team{{Slug: " platform ", Name: " Platform ", Description: " Infra ", Privacy: " closed ", ParentSlug: " parent "}},
		map[string][]config.TeamMemberSpec{},
		map[string][]config.TeamRepositorySpec{},
	)
	if len(got) != 1 {
		t.Fatalf("expected one team, got %#v", got)
	}

	team := got[0]
	if team.Slug != "platform" || team.Name != "Platform" || team.Description != "Infra" || team.Privacy != "closed" || team.ParentSlug != "parent" {
		t.Fatalf("unexpected team bootstrap result: %#v", team)
	}
	if team.Members == nil || len(team.Members) != 0 {
		t.Fatalf("expected empty non-nil members slice, got %#v", team.Members)
	}
	if team.Repositories == nil || len(team.Repositories) != 0 {
		t.Fatalf("expected empty non-nil repositories slice, got %#v", team.Repositories)
	}
}
