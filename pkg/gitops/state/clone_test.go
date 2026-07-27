package state

import (
	"reflect"
	"testing"
)

func TestCloneNilStateReturnsEmptyState(t *testing.T) {
	t.Parallel()

	var s *OrganizationState
	if got := s.Clone(); !reflect.DeepEqual(got, OrganizationState{}) {
		t.Fatalf("expected empty state, got %#v", got)
	}
}

func TestCloneDeepCopiesState(t *testing.T) {
	t.Parallel()

	original := OrganizationState{
		Organization: "orang-gaboets",
		Members: []OrganizationMember{
			{ID: 1, Username: "alice", Role: "admin"},
		},
		PendingInvitations: []PendingInvitation{
			{ID: 10, Username: "octocat", Role: "direct_member", TeamSlugs: []string{"platform"}},
		},
		Repositories: []Repository{
			{Owner: "orang-gaboets", Name: "octostate", Visibility: "private", Topics: []string{"go"}},
		},
		Teams: []Team{
			{ID: 2, Slug: "platform", Name: "Platform", Privacy: "closed"},
		},
		TeamMembers: []TeamMember{
			{TeamSlug: "platform", Username: "alice", Role: "maintainer"},
		},
		TeamRepositoryPermissions: []TeamRepositoryPermission{
			{TeamSlug: "platform", Owner: "orang-gaboets", Name: "octostate", Permission: "push"},
		},
	}

	cloned := original.Clone()
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("expected clone to equal original:\n got %#v\nwant %#v", cloned, original)
	}

	cloned.Members[0].Username = "changed"
	cloned.PendingInvitations[0].TeamSlugs[0] = "changed"
	cloned.Repositories[0].Topics[0] = "changed"
	cloned.Teams[0].Slug = "changed"
	cloned.TeamMembers[0].Username = "changed"
	cloned.TeamRepositoryPermissions[0].Permission = "admin"

	if original.Members[0].Username != "alice" {
		t.Fatalf("clone shares members with original: %#v", original.Members)
	}
	if original.PendingInvitations[0].TeamSlugs[0] != "platform" {
		t.Fatalf("clone shares invitation team slugs with original: %#v", original.PendingInvitations)
	}
	if original.Repositories[0].Topics[0] != "go" {
		t.Fatalf("clone shares repository topics with original: %#v", original.Repositories)
	}
	if original.Teams[0].Slug != "platform" {
		t.Fatalf("clone shares teams with original: %#v", original.Teams)
	}
	if original.TeamMembers[0].Username != "alice" {
		t.Fatalf("clone shares team members with original: %#v", original.TeamMembers)
	}
	if original.TeamRepositoryPermissions[0].Permission != "push" {
		t.Fatalf("clone shares team repository permissions with original: %#v", original.TeamRepositoryPermissions)
	}
}
