package syncfromlive

import (
	"testing"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
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

func TestBootstrapTeamRepositoryPermissionsGroupsByNormalizedSlug(t *testing.T) {
	t.Parallel()

	got, err := bootstrapTeamRepositoryPermissions(
		"orang-gaboets",
		[]state.Team{{Slug: "platform"}},
		[]state.TeamRepositoryPermission{{TeamSlug: " Platform ", Owner: " orang-gaboets ", Name: " repo-builder ", Permission: " push "}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []config.TeamRepositorySpec{{Name: "repo-builder", Permission: "push"}}
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
