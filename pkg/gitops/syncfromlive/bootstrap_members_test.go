package syncfromlive

import (
	"strings"
	"testing"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func TestBootstrapOrganizationMembersBuildsStableMembersWithRoles(t *testing.T) {
	t.Parallel()

	got, err := bootstrapOrganizationMembers(
		[]state.OrganizationMember{
			{Username: " bob ", Role: "member"},
			{Username: "Alice", Role: "admin"},
		},
		[]state.TeamMember{
			{TeamSlug: "platform", Username: "alice", Role: "maintainer"},
			{TeamSlug: "platform", Username: " bob ", Role: "member"},
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []config.OrganizationMemberSpec{
		{Username: "Alice", Role: "admin"},
		{Username: "bob", Role: "member"},
	}
	if len(got) != len(want) {
		t.Fatalf("unexpected organization members length: got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected organization members: got %#v want %#v", got, want)
		}
	}
}

func TestBootstrapOrganizationMembersRejectsMissingTeamBackedOrgMember(t *testing.T) {
	t.Parallel()

	_, err := bootstrapOrganizationMembers(
		[]state.OrganizationMember{
			{Username: "alice", Role: "admin"},
		},
		[]state.TeamMember{
			{TeamSlug: "platform", Username: "bob", Role: "member"},
		},
	)
	if err == nil || !strings.Contains(err.Error(), "missing from organization members") {
		t.Fatalf("expected missing organization member error, got %v", err)
	}
}

func TestBootstrapOrganizationMembersRejectsUnknownRole(t *testing.T) {
	t.Parallel()

	_, err := bootstrapOrganizationMembers(
		[]state.OrganizationMember{
			{Username: "alice", Role: ""},
		},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "unsupported role") {
		t.Fatalf("expected unsupported role error, got %v", err)
	}
}
