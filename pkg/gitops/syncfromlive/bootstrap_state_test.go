package syncfromlive

import (
	"testing"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func TestCloneOrganizationStateNil(t *testing.T) {
	t.Parallel()

	got := cloneOrganizationState(nil)
	if got.Organization != "" {
		t.Fatalf("expected zero-value organization, got %#v", got.Organization)
	}
	if got.Members != nil || got.PendingInvitations != nil || got.Repositories != nil || got.Teams != nil || got.TeamMembers != nil || got.TeamRepositoryPermissions != nil {
		t.Fatalf("expected zero-value state, got %#v", got)
	}
}

func TestCloneOrganizationStateNormalizesAndDeepClones(t *testing.T) {
	t.Parallel()

	actual := &state.OrganizationState{
		Organization: " orang-gaboets ",
		PendingInvitations: []state.PendingInvitation{
			{Username: "octocat", TeamSlugs: []string{"beta", "alpha"}},
		},
		Repositories: []state.Repository{
			{Name: "repo-builder", Topics: []string{"zeta", "alpha"}},
		},
	}

	got := cloneOrganizationState(actual)

	if got.Organization != "orang-gaboets" {
		t.Fatalf("expected trimmed organization, got %#v", got.Organization)
	}
	if got.Members == nil || got.Teams == nil || got.TeamMembers == nil || got.TeamRepositoryPermissions == nil {
		t.Fatalf("expected normalize to initialize nil slices, got %#v", got)
	}
	if len(got.PendingInvitations) != 1 || got.PendingInvitations[0].TeamSlugs[0] != "alpha" || got.PendingInvitations[0].TeamSlugs[1] != "beta" {
		t.Fatalf("expected normalized invitation team slugs, got %#v", got.PendingInvitations)
	}
	if len(got.Repositories) != 1 || got.Repositories[0].Topics[0] != "alpha" || got.Repositories[0].Topics[1] != "zeta" {
		t.Fatalf("expected normalized repository topics, got %#v", got.Repositories)
	}

	actual.PendingInvitations[0].TeamSlugs[0] = "changed"
	actual.Repositories[0].Topics[0] = "changed"
	if got.PendingInvitations[0].TeamSlugs[0] != "alpha" {
		t.Fatalf("expected invitation team slugs to be deep-cloned, got %#v", got.PendingInvitations[0].TeamSlugs)
	}
	if got.Repositories[0].Topics[0] != "alpha" {
		t.Fatalf("expected repository topics to be deep-cloned, got %#v", got.Repositories[0].Topics)
	}
}
