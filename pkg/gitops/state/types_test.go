package state

import (
	"reflect"
	"testing"
)

func TestOrganizationStateNormalizeNilReceiver(t *testing.T) {
	t.Parallel()

	var actual *OrganizationState
	actual.Normalize()
}

func TestOrganizationStateNormalizeInitializesNilSlices(t *testing.T) {
	t.Parallel()

	actual := &OrganizationState{
		Organization: "orang-gaboets",
	}

	actual.Normalize()

	if actual.Members == nil {
		t.Fatal("expected members to be initialized")
	}
	if actual.PendingInvitations == nil {
		t.Fatal("expected pending invitations to be initialized")
	}
	if actual.Repositories == nil {
		t.Fatal("expected repositories to be initialized")
	}
	if actual.Teams == nil {
		t.Fatal("expected teams to be initialized")
	}
	if actual.TeamMembers == nil {
		t.Fatal("expected team members to be initialized")
	}
	if actual.TeamRepositoryPermissions == nil {
		t.Fatal("expected team repository permissions to be initialized")
	}
}

func TestOrganizationStateNormalizeSortsCollections(t *testing.T) {
	t.Parallel()

	actual := &OrganizationState{
		Organization: "orang-gaboets",
		Members: []OrganizationMember{
			{ID: 2, Username: "zulu"},
			{ID: 1, Username: "Alpha"},
		},
		PendingInvitations: []PendingInvitation{
			{Email: "z@example.com", TeamSlugs: []string{"zeta", "Alpha"}},
			{UserID: 42, Username: "octocat", TeamSlugs: nil},
		},
		Repositories: []Repository{
			{Name: "zeta", Owner: "orang-gaboets", Topics: []string{"gitops", "Go"}},
			{Name: "alpha", Owner: "orang-gaboets", Topics: nil},
		},
		Teams: []Team{
			{ID: 2, Slug: "platform"},
			{ID: 1, Slug: "admins"},
		},
		TeamMembers: []TeamMember{
			{TeamSlug: "platform", Username: "zulu", Role: "member"},
			{TeamSlug: "admins", Username: "alpha", Role: "maintainer"},
		},
		TeamRepositoryPermissions: []TeamRepositoryPermission{
			{TeamSlug: "platform", Owner: "orang-gaboets", Name: "repo-builder", Permission: "push"},
			{TeamSlug: "admins", Owner: "orang-gaboets", Name: "repo-admin", Permission: "admin"},
		},
	}

	actual.Normalize()

	wantMembers := []OrganizationMember{
		{ID: 1, Username: "Alpha"},
		{ID: 2, Username: "zulu"},
	}
	if !reflect.DeepEqual(actual.Members, wantMembers) {
		t.Fatalf("unexpected members: got %#v want %#v", actual.Members, wantMembers)
	}

	wantInvitations := []PendingInvitation{
		{Email: "z@example.com", TeamSlugs: []string{"Alpha", "zeta"}},
		{UserID: 42, Username: "octocat", TeamSlugs: []string{}},
	}
	if !reflect.DeepEqual(actual.PendingInvitations, wantInvitations) {
		t.Fatalf("unexpected pending invitations: got %#v want %#v", actual.PendingInvitations, wantInvitations)
	}

	wantRepositories := []Repository{
		{Name: "alpha", Owner: "orang-gaboets", Topics: []string{}},
		{Name: "zeta", Owner: "orang-gaboets", Topics: []string{"gitops", "Go"}},
	}
	if !reflect.DeepEqual(actual.Repositories, wantRepositories) {
		t.Fatalf("unexpected repositories: got %#v want %#v", actual.Repositories, wantRepositories)
	}

	wantTeams := []Team{
		{ID: 1, Slug: "admins"},
		{ID: 2, Slug: "platform"},
	}
	if !reflect.DeepEqual(actual.Teams, wantTeams) {
		t.Fatalf("unexpected teams: got %#v want %#v", actual.Teams, wantTeams)
	}

	wantTeamMembers := []TeamMember{
		{TeamSlug: "admins", Username: "alpha", Role: "maintainer"},
		{TeamSlug: "platform", Username: "zulu", Role: "member"},
	}
	if !reflect.DeepEqual(actual.TeamMembers, wantTeamMembers) {
		t.Fatalf("unexpected team members: got %#v want %#v", actual.TeamMembers, wantTeamMembers)
	}

	wantPermissions := []TeamRepositoryPermission{
		{TeamSlug: "admins", Owner: "orang-gaboets", Name: "repo-admin", Permission: "admin"},
		{TeamSlug: "platform", Owner: "orang-gaboets", Name: "repo-builder", Permission: "push"},
	}
	if !reflect.DeepEqual(actual.TeamRepositoryPermissions, wantPermissions) {
		t.Fatalf("unexpected team repository permissions: got %#v want %#v", actual.TeamRepositoryPermissions, wantPermissions)
	}
}
