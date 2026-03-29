package syncfromlive

import (
	"strings"
	"testing"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func TestBuildAdoptConfigRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := BuildAdoptConfig(AdoptOptions{})
	if err == nil {
		t.Fatal("expected error for missing desired organization and actual state")
	}

	_, err = BuildAdoptConfig(AdoptOptions{
		Desired: config.OrganizationConfig{Organization: "orang-gaboets"},
		Actual:  &state.OrganizationState{Organization: "other-org"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match desired organization") {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
}

func TestBuildAdoptConfigMergesLiveStateWithoutDeletingConfigDeclarations(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []config.OrganizationMemberSpec{
			{Username: "carol", Role: "member"},
			{Username: "alice", Role: "member"},
		},
		Invites: []config.InviteSpec{{
			Username: config.OptionalString{Present: true, Value: "octocat"},
			Role:     "direct_member",
			TeamSlugs: []string{
				"platform",
			},
		}},
		Repositories: []config.RepositorySpec{
			func() config.RepositorySpec {
				repo := config.RepositorySpec{
					Name:       "repo-builder",
					Visibility: "public",
					Template: config.TemplateSpec{
						Owner: "orang-gaboets",
						Name:  "repo-template",
					},
					Topics: []string{"legacy"},
				}
				repo.SetManagedDescription("Old description")
				repo.SetManagedHomepage("https://old.example.com")
				return repo
			}(),
			{
				Name:       "legacy-only",
				Visibility: "private",
				Topics:     []string{"legacy"},
			},
		},
		Teams: []config.TeamSpec{
			{
				Slug:        "platform",
				Name:        "Platform",
				Description: "Old platform team",
				Privacy:     "closed",
				Members: []config.TeamMemberSpec{
					{Username: "carol", Role: "member"},
					{Username: "alice", Role: "member"},
				},
				Repositories: []config.TeamRepositorySpec{
					{Name: "legacy-only", Permission: "push"},
					{Name: "repo-builder", Permission: "pull"},
				},
			},
			{
				Slug:        "legacy-team",
				Name:        "Legacy Team",
				Description: "Keep me",
				Privacy:     "closed",
			},
		},
	}

	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		Members: []state.OrganizationMember{
			{Username: "erin", Role: "member"},
			{Username: "alice", Role: "admin"},
			{Username: "bob", Role: "member"},
		},
		Repositories: []state.Repository{
			{
				Owner:        "orang-gaboets",
				Name:         "repo-builder",
				Visibility:   "private",
				Description:  "GitOps CLI",
				Homepage:     "https://example.com/repo-builder",
				Topics:       []string{"gitops", "go"},
				AllowForking: false,
				Archived:     false,
				IsTemplate:   false,
			},
			{
				Owner:        "orang-gaboets",
				Name:         "live-only",
				Visibility:   "public",
				Description:  "Live only repo",
				Homepage:     "https://example.com/live-only",
				Topics:       []string{"adopted"},
				AllowForking: true,
				Archived:     true,
				IsTemplate:   true,
			},
		},
		Teams: []state.Team{
			{
				Slug:        "platform",
				Name:        "Platform",
				Description: "Platform engineering",
				Privacy:     "closed",
			},
			{
				Slug:        "operations",
				Name:        "Operations",
				Description: "Operations",
				Privacy:     "closed",
			},
		},
		TeamMembers: []state.TeamMember{
			{TeamSlug: "platform", Username: "alice", Role: "maintainer"},
			{TeamSlug: "platform", Username: "bob", Role: "member"},
			{TeamSlug: "operations", Username: "bob", Role: "member"},
		},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{
			{TeamSlug: "platform", Name: "repo-builder", Permission: "admin"},
			{TeamSlug: "operations", Name: "live-only", Permission: "push"},
		},
	}

	got, err := BuildAdoptConfig(AdoptOptions{
		Desired: desired,
		Actual:  actual,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Organization != "orang-gaboets" {
		t.Fatalf("unexpected organization %q", got.Organization)
	}
	if len(got.Invites) != 1 || got.Invites[0].Role != "direct_member" {
		t.Fatalf("expected invites to stay unchanged, got %#v", got.Invites)
	}
	assertAdoptedMembers(t, got.Members)
	assertAdoptedRepositories(t, got.Repositories)
	assertAdoptedTeams(t, got.Teams)

	if report := config.Validate(got); !report.Valid {
		t.Fatalf("expected adopted config to validate, got %#v", report)
	}
}

func TestBuildAdoptConfigDropsInvitesSatisfiedByLiveMembers(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []config.OrganizationMemberSpec{},
		Invites: []config.InviteSpec{
			{
				Username: config.OptionalString{Present: true, Value: "alice"},
				Role:     "direct_member",
			},
			{
				UserID: config.OptionalInt64{Present: true, Value: 42},
				Role:   "direct_member",
			},
			{
				Email: config.OptionalString{Present: true, Value: "carol@example.com"},
				Role:  "direct_member",
			},
			{
				Username: config.OptionalString{Present: true, Value: "octocat"},
				Role:     "direct_member",
			},
		},
		Repositories: []config.RepositorySpec{},
		Teams:        []config.TeamSpec{},
	}

	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		Members: []state.OrganizationMember{
			{ID: 1, Username: "alice", Role: "member"},
			{ID: 42, Username: "bob", Role: "member"},
			{ID: 99, Username: "carol", Email: "carol@example.com", Role: "admin"},
		},
	}

	got, err := BuildAdoptConfig(AdoptOptions{
		Desired: desired,
		Actual:  actual,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got.Invites) != 1 {
		t.Fatalf("expected only unmatched invites to remain, got %#v", got.Invites)
	}
	if got.Invites[0].Username.Value != "octocat" {
		t.Fatalf("expected unmatched invite to be preserved, got %#v", got.Invites[0])
	}
	if len(got.Members) != 3 {
		t.Fatalf("expected live org members to be adopted, got %#v", got.Members)
	}
	if report := config.Validate(got); !report.Valid {
		t.Fatalf("expected adopted config to validate after removing satisfied invites, got %#v", report)
	}
}

func assertAdoptedMembers(t *testing.T, members []config.OrganizationMemberSpec) {
	t.Helper()

	if len(members) != 4 {
		t.Fatalf("unexpected members %#v", members)
	}
	if members[0] != (config.OrganizationMemberSpec{Username: "carol", Role: "member"}) {
		t.Fatalf("unexpected first member %#v", members[0])
	}
	if members[1] != (config.OrganizationMemberSpec{Username: "alice", Role: "admin"}) {
		t.Fatalf("expected alice role to update from live, got %#v", members[1])
	}
	if members[2] != (config.OrganizationMemberSpec{Username: "bob", Role: "member"}) {
		t.Fatalf("expected bob to be appended from live, got %#v", members[2])
	}
	if members[3] != (config.OrganizationMemberSpec{Username: "erin", Role: "member"}) {
		t.Fatalf("expected direct member erin to be appended from live, got %#v", members[3])
	}
}

func assertAdoptedRepositories(t *testing.T, repositories []config.RepositorySpec) {
	t.Helper()

	if len(repositories) != 3 {
		t.Fatalf("unexpected repositories %#v", repositories)
	}
	repoBuilder := repositories[0]
	if repoBuilder.Template != (config.TemplateSpec{Owner: "orang-gaboets", Name: "repo-template"}) {
		t.Fatalf("expected template to be preserved, got %#v", repoBuilder.Template)
	}
	if repoBuilder.Visibility != "private" {
		t.Fatalf("expected visibility to update from live, got %#v", repoBuilder.Visibility)
	}
	if description, managed := repoBuilder.ManagedDescription(); !managed || description != "GitOps CLI" {
		t.Fatalf("expected managed description from live, got value=%q managed=%v", description, managed)
	}
	if homepage, managed := repoBuilder.ManagedHomepage(); !managed || homepage != "https://example.com/repo-builder" {
		t.Fatalf("expected managed homepage from live, got value=%q managed=%v", homepage, managed)
	}

	liveOnly := repositories[2]
	if liveOnly.Name != "live-only" || liveOnly.Visibility != "public" {
		t.Fatalf("unexpected live-only repository %#v", liveOnly)
	}
	if _, managed := liveOnly.ManagedDescription(); managed {
		t.Fatalf("expected adopted live-only description to stay unmanaged, got %#v", liveOnly.DescriptionOption())
	}
	if _, managed := liveOnly.ManagedHomepage(); managed {
		t.Fatalf("expected adopted live-only homepage to stay unmanaged, got %#v", liveOnly.HomepageOption())
	}
	if _, managed := liveOnly.ManagedAllowForking(); managed {
		t.Fatalf("expected adopted live-only allow_forking to stay unmanaged, got %#v", liveOnly.AllowForkingOption())
	}
	if _, managed := liveOnly.ManagedArchived(); managed {
		t.Fatalf("expected adopted live-only archived to stay unmanaged, got %#v", liveOnly.ArchivedOption())
	}
	if _, managed := liveOnly.ManagedIsTemplate(); managed {
		t.Fatalf("expected adopted live-only is_template to stay unmanaged, got %#v", liveOnly.IsTemplateOption())
	}
}

func assertAdoptedTeams(t *testing.T, teams []config.TeamSpec) {
	t.Helper()

	if len(teams) != 3 {
		t.Fatalf("unexpected teams %#v", teams)
	}
	if teams[0].Slug != "platform" || teams[1].Slug != "legacy-team" || teams[2].Slug != "operations" {
		t.Fatalf("expected existing order plus appended live team, got %#v", teams)
	}
	platform := teams[0]
	if platform.Description != "Platform engineering" {
		t.Fatalf("expected platform description to update from live, got %#v", platform.Description)
	}
	if len(platform.Members) != 3 {
		t.Fatalf("unexpected platform members %#v", platform.Members)
	}
	if platform.Members[0] != (config.TeamMemberSpec{Username: "carol", Role: "member"}) {
		t.Fatalf("expected config-only team member to stay first, got %#v", platform.Members[0])
	}
	if platform.Members[1] != (config.TeamMemberSpec{Username: "alice", Role: "maintainer"}) {
		t.Fatalf("expected alice team role to update from live, got %#v", platform.Members[1])
	}
	if platform.Members[2] != (config.TeamMemberSpec{Username: "bob", Role: "member"}) {
		t.Fatalf("expected bob team member to be appended from live, got %#v", platform.Members[2])
	}
	if len(platform.Repositories) != 2 {
		t.Fatalf("unexpected platform repositories %#v", platform.Repositories)
	}
	if platform.Repositories[0] != (config.TeamRepositorySpec{Name: "legacy-only", Permission: "push"}) {
		t.Fatalf("expected config-only team repository to stay first, got %#v", platform.Repositories[0])
	}
	if platform.Repositories[1] != (config.TeamRepositorySpec{Name: "repo-builder", Permission: "admin"}) {
		t.Fatalf("expected team repository permission to update from live, got %#v", platform.Repositories[1])
	}
	if len(teams[2].Members) != 1 || teams[2].Members[0] != (config.TeamMemberSpec{Username: "bob", Role: "member"}) {
		t.Fatalf("unexpected adopted ops members %#v", teams[2].Members)
	}
	if len(teams[2].Repositories) != 1 || teams[2].Repositories[0] != (config.TeamRepositorySpec{Name: "live-only", Permission: "push"}) {
		t.Fatalf("unexpected adopted ops repositories %#v", teams[2].Repositories)
	}
}
