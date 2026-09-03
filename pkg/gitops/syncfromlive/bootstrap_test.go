package syncfromlive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBuildBootstrapConfigRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	_, err := BuildBootstrapConfig(BootstrapOptions{})
	if err == nil {
		t.Fatal("expected error for nil actual state")
	}

	_, err = BuildBootstrapConfig(BootstrapOptions{
		Actual: &state.OrganizationState{},
	})
	if err == nil {
		t.Fatal("expected error for missing organization")
	}
}

func TestBuildBootstrapConfigBuildsCanonicalDesiredState(t *testing.T) {
	t.Parallel()

	actual := canonicalBootstrapActualState()

	got, err := BuildBootstrapConfig(BootstrapOptions{Actual: actual})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertCanonicalBootstrapConfig(t, got)
}

func TestBuildBootstrapConfigPreservesExternalManagedOwnersForValidation(t *testing.T) {
	t.Parallel()

	got, err := BuildBootstrapConfig(BootstrapOptions{Actual: &state.OrganizationState{
		Organization: "org-a",
		Repositories: []state.Repository{{
			Owner:      "org-b",
			Name:       "service",
			Visibility: "private",
		}},
		Teams: []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{{
			TeamSlug:   "platform",
			Owner:      "org-b",
			Name:       "service",
			Permission: "push",
		}},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Repositories[0].Owner != "org-b" || got.Teams[0].Repositories[0].Owner != "org-b" {
		t.Fatalf("expected external owners to be preserved, got %#v", got)
	}

	err = config.ValidateAndError(got)
	assertValidationErrorHasIssue(t, err, "repositories[0].owner", config.ValidationIssueCodeRepositoryOwnerScope)
	assertValidationErrorHasIssue(t, err, "teams[0].repositories[0].owner", config.ValidationIssueCodeRepositoryOwnerScope)
}

func TestBuildBootstrapConfigRejectsUnknownTeamRelationships(t *testing.T) {
	t.Parallel()

	_, err := BuildBootstrapConfig(BootstrapOptions{
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Teams:        []state.Team{},
			TeamMembers: []state.TeamMember{
				{TeamSlug: "platform", Username: "alice", Role: "member"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown team") {
		t.Fatalf("expected unknown team membership error, got %v", err)
	}

	_, err = BuildBootstrapConfig(BootstrapOptions{
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Teams:        []state.Team{},
			TeamRepositoryPermissions: []state.TeamRepositoryPermission{
				{TeamSlug: "platform", Name: "octostate", Permission: "push"},
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown team") {
		t.Fatalf("expected unknown team permission error, got %v", err)
	}
}

func TestBuildBootstrapConfigIncludesDirectOrganizationMembers(t *testing.T) {
	t.Parallel()

	got, err := BuildBootstrapConfig(BootstrapOptions{
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Members: []state.OrganizationMember{
				{Username: "alice", Role: "admin"},
				{Username: "carol", Role: "member"},
			},
			Teams: []state.Team{
				{Slug: "platform", Name: "Platform", Privacy: "closed"},
			},
			TeamMembers: []state.TeamMember{
				{TeamSlug: "platform", Username: "alice", Role: "member"},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected direct members to be included, got %v", err)
	}
	if got.Members == nil || len(got.Members) != 2 {
		t.Fatalf("expected direct members in bootstrap config, got %#v", got.Members)
	}
	if got.Members[0] != (config.OrganizationMemberSpec{Username: "alice", Role: "admin"}) {
		t.Fatalf("unexpected first member %#v", got.Members[0])
	}
	if got.Members[1] != (config.OrganizationMemberSpec{Username: "carol", Role: "member"}) {
		t.Fatalf("unexpected second member %#v", got.Members[1])
	}
}

func loadBootstrapConfig(t *testing.T, encoded []byte) config.OrganizationConfig {
	t.Helper()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "organization.yaml")
	if err := os.WriteFile(configPath, encoded, 0o600); err != nil {
		t.Fatalf("write bootstrap config: %v", err)
	}

	cfg, err := config.LoadDir(configDir)
	if err != nil {
		t.Fatalf("load bootstrap config: %v", err)
	}
	return cfg
}

func canonicalBootstrapActualState() *state.OrganizationState {
	return &state.OrganizationState{
		Organization: " orang-gaboets ",
		Members: []state.OrganizationMember{
			{Username: "bob", Role: "member"},
			{Username: "alice", Role: "admin"},
		},
		PendingInvitations: []state.PendingInvitation{{Username: "octocat", Role: "direct_member"}},
		Repositories: []state.Repository{
			{
				Owner:        "orang-gaboets",
				Name:         "shared-repo",
				Visibility:   "public",
				Description:  "",
				Homepage:     "",
				Topics:       []string{"beta", "alpha"},
				AllowForking: false,
				Archived:     false,
				IsTemplate:   false,
			},
			{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "private",
				Description:  "GitOps CLI",
				Homepage:     "https://example.com/octostate",
				Topics:       []string{"gitops", "go"},
				AllowForking: true,
				Archived:     true,
				IsTemplate:   false,
			},
		},
		Teams: []state.Team{
			{
				Slug:        "platform-infra",
				Name:        "Platform Infra",
				Description: "Infra",
				Privacy:     "closed",
				ParentSlug:  "platform",
			},
			{
				ID:          42,
				Slug:        "platform",
				Name:        "Platform",
				Description: "Platform engineering",
				Privacy:     "closed",
			},
		},
		TeamMembers: []state.TeamMember{
			{TeamSlug: "platform-infra", Username: "bob", Role: "member"},
			{TeamSlug: "platform", Username: "alice", Role: "maintainer"},
		},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{
			{TeamSlug: "platform-infra", Owner: "orang-gaboets", Name: "shared-repo", Permission: "pull"},
			{TeamSlug: "platform", Owner: "orang-gaboets", Name: "octostate", Permission: "admin"},
		},
	}
}

func assertCanonicalBootstrapConfig(t *testing.T, got config.OrganizationConfig) {
	t.Helper()

	assertBootstrapTopLevel(t, got)
	assertBootstrapRepositories(t, got.Repositories)
	assertBootstrapTeams(t, got.Teams)
	assertBootstrapRoundTrip(t, got)
}

func assertBootstrapTopLevel(t *testing.T, got config.OrganizationConfig) {
	t.Helper()

	if got.Organization != "orang-gaboets" {
		t.Fatalf("expected trimmed organization, got %#v", got.Organization)
	}
	if got.Members == nil || len(got.Members) != 2 {
		t.Fatalf("expected top-level members for team-backed users, got %#v", got.Members)
	}
	if got.Members[0] != (config.OrganizationMemberSpec{Username: "alice", Role: "admin"}) {
		t.Fatalf("unexpected first top-level member: %#v", got.Members[0])
	}
	if got.Members[1] != (config.OrganizationMemberSpec{Username: "bob", Role: "member"}) {
		t.Fatalf("unexpected second top-level member: %#v", got.Members[1])
	}
	if got.Invites == nil || len(got.Invites) != 0 {
		t.Fatalf("expected pending invites to be excluded, got %#v", got.Invites)
	}
}

func assertBootstrapRepositories(t *testing.T, repositories []config.RepositorySpec) {
	t.Helper()

	if len(repositories) != 2 {
		t.Fatalf("expected two repositories, got %#v", repositories)
	}

	repoBuilder := repositories[0]
	if repoBuilder.Owner != "" {
		t.Fatalf("expected org-owned repository owner to be omitted, got %#v", repoBuilder.Owner)
	}
	if repoBuilder.Name != "octostate" || repoBuilder.Visibility != "private" {
		t.Fatalf("unexpected octostate bootstrap result: %#v", repoBuilder)
	}
	if repoBuilder.Topics == nil || len(repoBuilder.Topics) != 2 || repoBuilder.Topics[0] != "gitops" || repoBuilder.Topics[1] != "go" {
		t.Fatalf("expected sorted topics, got %#v", repoBuilder.Topics)
	}
	if description, managed := repoBuilder.ManagedDescription(); !managed || description != "GitOps CLI" {
		t.Fatalf("expected managed description, got value=%q managed=%v", description, managed)
	}
	if homepage, managed := repoBuilder.ManagedHomepage(); !managed || homepage != "https://example.com/octostate" {
		t.Fatalf("expected managed homepage, got value=%q managed=%v", homepage, managed)
	}
	if value, managed := repoBuilder.ManagedAllowForking(); !managed || !value {
		t.Fatalf("expected managed private allow_forking=true, got value=%v managed=%v", value, managed)
	}
	if archived, managed := repoBuilder.ManagedArchived(); !managed || !archived {
		t.Fatalf("expected managed archived=true, got value=%v managed=%v", archived, managed)
	}
	if isTemplate, managed := repoBuilder.ManagedIsTemplate(); !managed || isTemplate {
		t.Fatalf("expected managed is_template=false, got value=%v managed=%v", isTemplate, managed)
	}

	sharedRepo := repositories[1]
	if sharedRepo.Owner != "" {
		t.Fatalf("expected same-organization repository owner to be omitted, got %#v", sharedRepo.Owner)
	}
	if _, managed := sharedRepo.ManagedAllowForking(); managed {
		t.Fatalf("expected public allow_forking to stay unmanaged, got %#v", sharedRepo.AllowForkingOption())
	}
	if description, managed := sharedRepo.ManagedDescription(); !managed || description != "" {
		t.Fatalf("expected explicit empty managed description, got value=%q managed=%v", description, managed)
	}
	if homepage, managed := sharedRepo.ManagedHomepage(); !managed || homepage != "" {
		t.Fatalf("expected explicit empty managed homepage, got value=%q managed=%v", homepage, managed)
	}
}

func assertBootstrapTeams(t *testing.T, teams []config.TeamSpec) {
	t.Helper()

	if len(teams) != 2 {
		t.Fatalf("expected two teams, got %#v", teams)
	}
	if teams[0].Slug != "platform" || teams[1].Slug != "platform-infra" {
		t.Fatalf("expected teams sorted by slug, got %#v", teams)
	}
	if teams[0].Members == nil || len(teams[0].Members) != 1 || teams[0].Members[0].Username != "alice" {
		t.Fatalf("unexpected platform members: %#v", teams[0].Members)
	}
	if teams[0].Repositories == nil || len(teams[0].Repositories) != 1 || teams[0].Repositories[0].Owner != "" {
		t.Fatalf("expected org-owned team repository owner to be omitted, got %#v", teams[0].Repositories)
	}
	if teams[1].Repositories == nil || len(teams[1].Repositories) != 1 || teams[1].Repositories[0].Owner != "" {
		t.Fatalf("expected same-organization team repository owner to be omitted, got %#v", teams[1].Repositories)
	}
	if teams[1].ParentSlug != "platform" {
		t.Fatalf("expected parent slug, got %#v", teams[1].ParentSlug)
	}
}

func assertBootstrapRoundTrip(t *testing.T, got config.OrganizationConfig) {
	t.Helper()

	encoded, err := config.EncodeYAML(got)
	if err != nil {
		t.Fatalf("unexpected encode error: %v", err)
	}
	if strings.Contains(string(encoded), "username: octocat") {
		t.Fatalf("expected pending invites to stay excluded from encoded bootstrap config, got:\n%s", string(encoded))
	}
	roundTripped := loadBootstrapConfig(t, encoded)
	if report := config.Validate(roundTripped); !report.Valid {
		t.Fatalf("expected round-tripped bootstrap config to validate, got %#v", report)
	}
}
