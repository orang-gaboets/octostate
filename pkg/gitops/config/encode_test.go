package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEncodeYAMLMinimal(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: " orang-gaboets ",
	}

	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.Join([]string{
		"organization: orang-gaboets",
		"members: []",
		"invites: []",
		"repositories: []",
		"teams: []",
		"",
	}, "\n")

	if string(got) != want {
		t.Fatalf("unexpected YAML:\n%s\nwant:\n%s", string(got), want)
	}

	roundTripped := loadEncodedConfig(t, got)
	if roundTripped.Organization != "orang-gaboets" {
		t.Fatalf("unexpected round-tripped organization: %#v", roundTripped.Organization)
	}
	if roundTripped.Members == nil || roundTripped.Invites == nil || roundTripped.Repositories == nil || roundTripped.Teams == nil {
		t.Fatalf("expected non-nil top-level slices, got %#v", roundTripped)
	}
}

func TestEncodeYAMLValidRoundTrip(t *testing.T) {
	t.Parallel()

	got, err := EncodeYAML(validOrganizationConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := strings.Join([]string{
		"organization: orang-gaboets",
		"members:",
		"  - username: alice",
		"    role: member",
		"invites:",
		"  - username: octocat",
		"    role: direct_member",
		"    team_slugs:",
		"      - platform",
		"repositories:",
		"  - name: repo-builder",
		"    template:",
		"      owner: orang-gaboets",
		"      name: repo-template",
		"    visibility: private",
		"    description: GitHub organization operations CLI",
		"    homepage: https://github.com/orang-gaboets/repo-builder",
		"    topics:",
		"      - go",
		"      - gitops",
		"    allow_forking: false",
		"    archived: false",
		"    is_template: false",
		"teams:",
		"  - slug: platform",
		"    name: Platform",
		"    description: Platform engineering",
		"    privacy: closed",
		"    members:",
		"      - username: alice",
		"        role: maintainer",
		"    repositories:",
		"      - name: repo-builder",
		"        permission: push",
		"",
	}, "\n")

	if string(got) != want {
		t.Fatalf("unexpected YAML:\n%s\nwant:\n%s", string(got), want)
	}

	roundTripped := loadEncodedConfig(t, got)
	if report := Validate(roundTripped); !report.Valid {
		t.Fatalf("expected round-tripped config to validate, got %#v", report)
	}
}

func TestEncodeYAMLPreservesExplicitNullOptionals(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []InviteSpec{{
			Username: nullOptionalString(),
			Role:     "direct_member",
		}},
		Repositories: []RepositorySpec{{
			Name:         "repo-builder",
			Visibility:   "private",
			description:  nullOptionalString(),
			homepage:     nullOptionalString(),
			allowForking: nullOptionalBool(),
			archived:     nullOptionalBool(),
			isTemplate:   nullOptionalBool(),
		}},
		Teams: []TeamSpec{},
	}

	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := string(got)
	for _, snippet := range []string{
		"username: null",
		"description: null",
		"homepage: null",
		"allow_forking: null",
		"archived: null",
		"is_template: null",
	} {
		if !strings.Contains(text, snippet) {
			t.Fatalf("expected YAML to contain %q, got:\n%s", snippet, text)
		}
	}
}

func TestEncodeYAMLIncludesInviteUserID(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []InviteSpec{{
			UserID: optionalInt64(12345),
			Role:   "direct_member",
		}},
		Repositories: []RepositorySpec{},
		Teams:        []TeamSpec{},
	}

	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := string(got)
	for _, expected := range []string{
		"user_id: 12345",
		"role: direct_member",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected YAML to contain %q, got:\n%s", expected, text)
		}
	}

	roundTripped := loadEncodedConfig(t, got)
	if len(roundTripped.Invites) != 1 {
		t.Fatalf("expected one round-tripped invite, got %#v", roundTripped.Invites)
	}
	if !roundTripped.Invites[0].UserID.Present || roundTripped.Invites[0].UserID.Null || roundTripped.Invites[0].UserID.Value != 12345 {
		t.Fatalf("unexpected round-tripped user_id option: %#v", roundTripped.Invites[0].UserID)
	}
}

func TestEncodeYAMLRoundTripIsStable(t *testing.T) {
	t.Parallel()

	first, err := EncodeYAML(validOrganizationConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	roundTripped := loadEncodedConfig(t, first)
	second, err := EncodeYAML(roundTripped)
	if err != nil {
		t.Fatalf("unexpected error on second encode: %v", err)
	}

	if string(second) != string(first) {
		t.Fatalf("expected stable encode-load-encode round trip:\nfirst:\n%s\nsecond:\n%s", string(first), string(second))
	}
}

func TestEncodeYAMLOmitsOrgOwnersAndEmptyNestedSections(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []InviteSpec{{
			Username: optionalString("octocat"),
			Role:     "direct_member",
		}},
		Repositories: []RepositorySpec{{
			Owner:      "orang-gaboets",
			Name:       "repo-builder",
			Visibility: "private",
			Template: TemplateSpec{
				Owner: "orang-gaboets",
				Name:  "repo-template",
			},
		}},
		Teams: []TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
		}},
	}

	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := string(got)
	for _, unexpected := range []string{
		"team_slugs:",
		"\n    members:",
		"\n    repositories:",
		"include_all_branches:",
	} {
		if strings.Contains(text, unexpected) {
			t.Fatalf("did not expect YAML to contain %q, got:\n%s", unexpected, text)
		}
	}
	if strings.Contains(text, "\n  - owner: orang-gaboets") {
		t.Fatalf("did not expect org-owned repository entries to emit owner, got:\n%s", text)
	}
}

func TestEncodeYAMLIncludesExplicitOptionalsAndExternalOwners(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "orang-gaboets",
		Invites:      []InviteSpec{},
		Repositories: []RepositorySpec{{
			Owner:        "shared-platform",
			Name:         "repo-builder",
			Visibility:   "public",
			Description:  "",
			Homepage:     "",
			description:  optionalString(""),
			homepage:     optionalString(""),
			allowForking: optionalBool(false),
			archived:     optionalBool(false),
			isTemplate:   optionalBool(false),
			Template: TemplateSpec{
				Owner:              "shared-platform",
				Name:               "repo-template",
				IncludeAllBranches: true,
			},
		}},
		Teams: []TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Repositories: []TeamRepositorySpec{{
				Owner:      "shared-platform",
				Name:       "repo-builder",
				Permission: "push",
			}},
		}},
	}

	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := string(got)
	for _, expected := range []string{
		"owner: shared-platform",
		"description: \"\"",
		"homepage: \"\"",
		"allow_forking: false",
		"archived: false",
		"is_template: false",
		"include_all_branches: true",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected YAML to contain %q, got:\n%s", expected, text)
		}
	}
}

func TestEncodeYAMLIncludesTeamParentSlug(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "orang-gaboets",
		Invites:      []InviteSpec{},
		Repositories: []RepositorySpec{},
		Teams: []TeamSpec{
			{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
			},
			{
				Slug:       "platform-infra",
				Name:       "Platform Infra",
				Privacy:    "closed",
				ParentSlug: "platform",
			},
		},
	}

	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := string(got)
	if !strings.Contains(text, "parent_slug: platform") {
		t.Fatalf("expected YAML to contain parent_slug, got:\n%s", text)
	}

	roundTripped := loadEncodedConfig(t, got)
	if len(roundTripped.Teams) != 2 {
		t.Fatalf("expected two round-tripped teams, got %#v", roundTripped.Teams)
	}
	if roundTripped.Teams[1].ParentSlug != "platform" {
		t.Fatalf("expected round-tripped parent slug, got %#v", roundTripped.Teams[1].ParentSlug)
	}
}

func TestEncodeYAMLProgrammaticRepositoryFallbacks(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "orang-gaboets",
		Invites:      []InviteSpec{},
		Repositories: []RepositorySpec{{
			Name:         "repo-builder",
			Visibility:   "public",
			Description:  "GitOps CLI",
			Homepage:     "https://example.com/repo-builder",
			AllowForking: true,
			Archived:     true,
			IsTemplate:   true,
			Topics:       []string{"go"},
		}},
		Teams: []TeamSpec{},
	}

	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	text := string(got)
	for _, expected := range []string{
		"description: GitOps CLI",
		"homepage: https://example.com/repo-builder",
		"allow_forking: true",
		"archived: true",
		"is_template: true",
	} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected YAML to contain %q, got:\n%s", expected, text)
		}
	}

	roundTripped := loadEncodedConfig(t, got)
	repo := roundTripped.Repositories[0]
	if value, managed := repo.ManagedDescription(); !managed || value != "GitOps CLI" {
		t.Fatalf("expected round-tripped fallback description, got value=%q managed=%v", value, managed)
	}
	if value, managed := repo.ManagedHomepage(); !managed || value != "https://example.com/repo-builder" {
		t.Fatalf("expected round-tripped fallback homepage, got value=%q managed=%v", value, managed)
	}
	if value, managed := repo.ManagedAllowForking(); !managed || !value {
		t.Fatalf("expected round-tripped fallback allow_forking, got value=%v managed=%v", value, managed)
	}
	if value, managed := repo.ManagedArchived(); !managed || !value {
		t.Fatalf("expected round-tripped fallback archived, got value=%v managed=%v", value, managed)
	}
	if value, managed := repo.ManagedIsTemplate(); !managed || !value {
		t.Fatalf("expected round-tripped fallback is_template, got value=%v managed=%v", value, managed)
	}
}

func TestAppendOptionalInt64FieldOmitsUndeclaredValue(t *testing.T) {
	t.Parallel()

	node := mapNode()
	appendOptionalInt64Field(node, "user_id", OptionalInt64{})

	if len(node.Content) != 0 {
		t.Fatalf("expected no map content for undeclared optional int64, got %#v", node.Content)
	}
}

func TestSequenceNodeNilReturnsEmptySequence(t *testing.T) {
	t.Parallel()

	node := sequenceNode(nil)
	if node == nil {
		t.Fatal("expected non-nil sequence node")
		return
	}
	if node.Kind != 2 { // yaml.SequenceNode
		t.Fatalf("expected sequence node kind, got %#v", node.Kind)
	}
	if node.Content == nil {
		t.Fatal("expected non-nil empty sequence content")
	}
	if len(node.Content) != 0 {
		t.Fatalf("expected empty sequence content, got %#v", node.Content)
	}
}

func loadEncodedConfig(t *testing.T, encoded []byte) OrganizationConfig {
	t.Helper()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, organizationFileName)
	if err := os.WriteFile(configPath, encoded, 0o600); err != nil {
		t.Fatalf("write encoded config: %v", err)
	}

	cfg, err := LoadDir(configDir)
	if err != nil {
		t.Fatalf("load encoded config: %v", err)
	}
	return cfg
}
