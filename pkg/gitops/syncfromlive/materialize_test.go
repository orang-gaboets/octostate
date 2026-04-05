package syncfromlive

import (
	"strings"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBuildMaterializeConfigRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := BuildMaterializeConfig(MaterializeOptions{})
	if err == nil {
		t.Fatal("expected error for missing desired organization and actual state")
	}

	_, err = BuildMaterializeConfig(MaterializeOptions{
		Desired: config.OrganizationConfig{Organization: "orang-gaboets"},
		Actual:  &state.OrganizationState{Organization: "other-org"},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match desired organization") {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
}

func TestBuildMaterializeConfigFillsOnlyUnmanagedRepositoryFields(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []config.OrganizationMemberSpec{
			{Username: "alice", Role: "member"},
		},
		Invites: []config.InviteSpec{
			{
				Username: config.OptionalString{Present: true, Value: "octocat"},
				Role:     "direct_member",
			},
		},
		Repositories: []config.RepositorySpec{
			func() config.RepositorySpec {
				repo := config.RepositorySpec{
					Name:       "octostate",
					Visibility: "public",
					Topics:     []string{"gitops"},
					Template: config.TemplateSpec{
						Owner: "orang-gaboets",
						Name:  "repo-template",
					},
				}
				repo.SetManagedDescription("Keep me")
				repo.SetManagedArchived(false)
				return repo
			}(),
			{
				Name:       "private-repo",
				Visibility: "private",
			},
			{
				Name:       "config-only",
				Visibility: "public",
			},
		},
		Teams: []config.TeamSpec{
			{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
				Members: []config.TeamMemberSpec{
					{Username: "alice", Role: "member"},
				},
			},
		},
	}

	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{
			{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "private",
				Description:  "Live description",
				Homepage:     "https://example.com/octostate",
				Topics:       []string{"go", "gitops"},
				AllowForking: false,
				Archived:     true,
				IsTemplate:   true,
			},
			{
				Owner:        "orang-gaboets",
				Name:         "private-repo",
				Visibility:   "private",
				Description:  "",
				Homepage:     "",
				AllowForking: true,
				Archived:     false,
				IsTemplate:   false,
			},
			{
				Owner:        "orang-gaboets",
				Name:         "live-only",
				Visibility:   "public",
				Description:  "adopt me first",
				Homepage:     "https://example.com/live-only",
				AllowForking: true,
				Archived:     true,
				IsTemplate:   true,
			},
		},
	}

	got, err := BuildMaterializeConfig(MaterializeOptions{
		Desired: desired,
		Actual:  actual,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Organization != desired.Organization {
		t.Fatalf("unexpected organization %q", got.Organization)
	}
	if len(got.Repositories) != 3 {
		t.Fatalf("unexpected repositories %#v", got.Repositories)
	}
	assertMaterializedTopLevelConfig(t, desired, got)
	assertMaterializedRepoBuilder(t, desired.Repositories[0], got.Repositories[0])
	assertMaterializedPrivateRepo(t, got.Repositories[1])
	assertMaterializedConfigOnlyRepo(t, got.Repositories[2])
	assertNoLiveOnlyRepository(t, got.Repositories)

	if report := config.Validate(got); !report.Valid {
		t.Fatalf("expected materialized config to validate, got %#v", report)
	}
}

func assertMaterializedTopLevelConfig(
	t *testing.T,
	desired config.OrganizationConfig,
	got config.OrganizationConfig,
) {
	t.Helper()

	if len(got.Members) != 1 || got.Members[0] != desired.Members[0] {
		t.Fatalf("expected members to remain unchanged, got %#v", got.Members)
	}
	if len(got.Invites) != 1 || got.Invites[0].Username.Value != "octocat" {
		t.Fatalf("expected invites to remain unchanged, got %#v", got.Invites)
	}
	if len(got.Teams) != 1 || got.Teams[0].Slug != "platform" {
		t.Fatalf("expected teams to remain unchanged, got %#v", got.Teams)
	}
}

func assertMaterializedRepoBuilder(
	t *testing.T,
	desired config.RepositorySpec,
	got config.RepositorySpec,
) {
	t.Helper()

	if description, managed := got.ManagedDescription(); !managed || description != "Keep me" {
		t.Fatalf("expected managed description to stay unchanged, got value=%q managed=%v", description, managed)
	}
	if homepage, managed := got.ManagedHomepage(); !managed || homepage != "https://example.com/octostate" {
		t.Fatalf("expected homepage to materialize from live, got value=%q managed=%v", homepage, managed)
	}
	if allowForking, managed := got.ManagedAllowForking(); !managed || allowForking {
		t.Fatalf("expected allow_forking=false to materialize from live, got value=%v managed=%v", allowForking, managed)
	}
	if archived, managed := got.ManagedArchived(); !managed || archived {
		t.Fatalf("expected managed archived value to stay unchanged, got value=%v managed=%v", archived, managed)
	}
	if isTemplate, managed := got.ManagedIsTemplate(); !managed || !isTemplate {
		t.Fatalf("expected is_template=true to materialize from live, got value=%v managed=%v", isTemplate, managed)
	}
	if got.Template != desired.Template {
		t.Fatalf("expected template to remain unchanged, got %#v", got.Template)
	}
	if got.Visibility != "public" {
		t.Fatalf("expected visibility to remain unchanged, got %q", got.Visibility)
	}
	if len(got.Topics) != 1 || got.Topics[0] != "gitops" {
		t.Fatalf("expected topics to remain unchanged, got %#v", got.Topics)
	}
}

func assertMaterializedPrivateRepo(t *testing.T, got config.RepositorySpec) {
	t.Helper()

	if description, managed := got.ManagedDescription(); !managed || description != "" {
		t.Fatalf("expected empty live description to materialize as managed clear, got value=%q managed=%v", description, managed)
	}
	if homepage, managed := got.ManagedHomepage(); !managed || homepage != "" {
		t.Fatalf("expected empty live homepage to materialize as managed clear, got value=%q managed=%v", homepage, managed)
	}
	if _, managed := got.ManagedAllowForking(); managed {
		t.Fatalf("expected allow_forking to stay unmanaged for private repo, got %#v", got.AllowForkingOption())
	}
	if archived, managed := got.ManagedArchived(); !managed || archived {
		t.Fatalf("expected archived=false to materialize from live, got value=%v managed=%v", archived, managed)
	}
	if isTemplate, managed := got.ManagedIsTemplate(); !managed || isTemplate {
		t.Fatalf("expected is_template=false to materialize from live, got value=%v managed=%v", isTemplate, managed)
	}
}

func assertMaterializedConfigOnlyRepo(t *testing.T, got config.RepositorySpec) {
	t.Helper()

	if _, managed := got.ManagedDescription(); managed {
		t.Fatalf("expected config-only repo description to stay unmanaged, got %#v", got.DescriptionOption())
	}
	if _, managed := got.ManagedHomepage(); managed {
		t.Fatalf("expected config-only repo homepage to stay unmanaged, got %#v", got.HomepageOption())
	}
	if _, managed := got.ManagedAllowForking(); managed {
		t.Fatalf("expected config-only repo allow_forking to stay unmanaged, got %#v", got.AllowForkingOption())
	}
	if _, managed := got.ManagedArchived(); managed {
		t.Fatalf("expected config-only repo archived to stay unmanaged, got %#v", got.ArchivedOption())
	}
	if _, managed := got.ManagedIsTemplate(); managed {
		t.Fatalf("expected config-only repo is_template to stay unmanaged, got %#v", got.IsTemplateOption())
	}
}

func assertNoLiveOnlyRepository(t *testing.T, repositories []config.RepositorySpec) {
	t.Helper()

	for _, repository := range repositories {
		if repository.Name == "live-only" {
			t.Fatalf("did not expect live-only repository to be added: %#v", repository)
		}
	}
}
