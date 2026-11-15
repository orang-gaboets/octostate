package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestEncodeYAMLRejectsOutputAboveSupportedLimit(t *testing.T) {
	t.Parallel()

	_, err := EncodeYAML(OrganizationConfig{
		Organization: strings.Repeat("x", (16<<20)+1),
	})
	if err == nil {
		t.Fatal("EncodeYAML accepted output larger than 16 MiB")
	}
	if !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("EncodeYAML error does not identify the supported size limit: %v", err)
	}
}

func TestEncodeYAMLEnforcesOutputLimitAtByteBoundary(t *testing.T) {
	baseline, err := EncodeYAML(OrganizationConfig{Organization: "x"})
	if err != nil {
		t.Fatalf("encode baseline: %v", err)
	}
	fixedBytes := int64(len(baseline) - len("x"))

	for _, test := range []struct {
		name     string
		wantSize int64
		wantErr  bool
	}{
		{name: "exact limit accepted", wantSize: maxDesiredStateYAMLSize},
		{name: "one byte over rejected", wantSize: maxDesiredStateYAMLSize + 1, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			organization := strings.Repeat("x", int(test.wantSize-fixedBytes))
			got, err := EncodeYAML(OrganizationConfig{Organization: organization})
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "16 MiB") {
					t.Fatalf("EncodeYAML error = %v, want 16 MiB limit", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("EncodeYAML rejected output at the limit: %v", err)
			}
			if int64(len(got)) != test.wantSize {
				t.Fatalf("encoded size = %d, want %d", len(got), test.wantSize)
			}
		})
	}
}

func TestEncodeYAMLReportsSupportedSizeWhenDocumentOverflowsAfterScalarEncoding(t *testing.T) {
	cfg := OrganizationConfig{Organization: strings.Repeat("x", int(maxDesiredStateYAMLSize)-40)}
	_, err := EncodeYAML(cfg)
	if err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("EncodeYAML error = %v, want supported 16 MiB size limit", err)
	}
}

func TestEncodeYAMLRejectsOversizedScalarWithoutLargeSerializationAllocation(t *testing.T) {
	oversized := strings.Repeat("x", int(maxDesiredStateYAMLSize)+1)
	cfg := OrganizationConfig{Organization: oversized}

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := EncodeYAML(cfg)
	runtime.ReadMemStats(&after)

	if err == nil || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("EncodeYAML error = %v, want 16 MiB output limit", err)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 8<<20 {
		t.Fatalf("EncodeYAML allocated %d bytes for a scalar rejected by the output cap, want under 8 MiB", allocated)
	}
}

func TestEncodeYAMLMatchesLegacyBytesForScalarEdgeValues(t *testing.T) {
	cfg := OrganizationConfig{
		Organization: "  123: value  ",
		Members: []OrganizationMemberSpec{{
			Username: " true ",
			Role:     "null",
		}},
		Invites: []InviteSpec{{
			Username:  optionalString("- leading"),
			Email:     optionalString("  mail@example.test  "),
			UserID:    optionalInt64(7),
			Role:      " direct_member ",
			TeamSlugs: []string{"?", "[flow]", "<&>", "line one\nline two"},
		}},
		Repositories: []RepositorySpec{{
			Owner:      "other-owner",
			Name:       " null ",
			Visibility: " private ",
			Template: TemplateSpec{
				Owner:              " template-owner ",
				Name:               " yes ",
				IncludeAllBranches: true,
			},
			Description: "first line\nsecond line\n",
			Homepage:    "https://example.test/<>&",
			Topics:      []string{"true", "no", "12", `"quoted"`, "\tleading-tab"},
		}},
		Teams: []TeamSpec{{
			Slug:        "platform",
			Name:        "Platform",
			Description: "description\nwith newline",
			Privacy:     "closed",
			ParentSlug:  "1-parent",
			Members:     []TeamMemberSpec{{Username: "alice", Role: "maintainer"}},
			Repositories: []TeamRepositorySpec{{
				Owner:      "external-owner",
				Name:       "repository",
				Permission: "push",
			}},
		}},
	}
	cfg.Repositories[0].SetManagedDescription(cfg.Repositories[0].Description)
	cfg.Repositories[0].SetManagedHomepage(cfg.Repositories[0].Homepage)

	want, err := legacyEncodeYAMLForTest(cfg)
	if err != nil {
		t.Fatalf("legacy encoder returned an error: %v", err)
	}
	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("EncodeYAML returned an error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeYAML changed canonical bytes:\ngot:\n%s\nwant:\n%s", got, want)
	}

	var roundTrip OrganizationConfig
	decoder := yaml.NewDecoder(bytes.NewReader(got))
	decoder.KnownFields(true)
	if err := decoder.Decode(&roundTrip); err != nil {
		t.Fatalf("decode encoded edge-value config: %v", err)
	}
	normalized := NormalizeDesiredState(cfg)
	if roundTrip.Repositories[0].Description != normalized.Repositories[0].Description || roundTrip.Repositories[0].Homepage != normalized.Repositories[0].Homepage {
		t.Fatalf("round trip changed managed repository strings: %#v", roundTrip.Repositories[0])
	}

	legacyDocument := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{encodeOrganizationConfig(normalized)}}
	nodeCount := countYAMLNodesForTest(legacyDocument)
	if err := preflightDesiredYAML(normalized, nodeCount, maxDesiredStateYAMLSize); err != nil {
		t.Fatalf("preflight rejected exactly %d YAML nodes: %v", nodeCount, err)
	}
	if err := preflightDesiredYAML(normalized, nodeCount-1, maxDesiredStateYAMLSize); err == nil || !strings.Contains(err.Error(), "YAML node limit") {
		t.Fatalf("preflight error at %d nodes = %v, want node limit", nodeCount-1, err)
	}
}

func TestEncodeYAMLMatchesLegacyBytesAcrossBoundedFragments(t *testing.T) {
	cfg := OrganizationConfig{Organization: "acme", Members: make([]OrganizationMemberSpec, 500)}
	for i := range cfg.Members {
		cfg.Members[i] = OrganizationMemberSpec{Username: fmt.Sprintf("user-%04d", i), Role: "member"}
	}
	cfg.Invites = []InviteSpec{{Role: "direct_member", TeamSlugs: make([]string, 2_300)}}
	for i := range cfg.Invites[0].TeamSlugs {
		cfg.Invites[0].TeamSlugs[i] = fmt.Sprintf("team-%04d", i)
	}
	cfg.Repositories = make([]RepositorySpec, 450)
	for i := range cfg.Repositories {
		cfg.Repositories[i] = RepositorySpec{Name: fmt.Sprintf("repo-%04d", i), Visibility: "private", Description: "generated"}
	}
	cfg.Repositories[0].Topics = make([]string, 2_200)
	for i := range cfg.Repositories[0].Topics {
		cfg.Repositories[0].Topics[i] = fmt.Sprintf("topic-%04d", i)
	}
	cfg.Teams = []TeamSpec{{
		Slug: "platform", Name: "Platform", Privacy: "closed",
		Members: make([]TeamMemberSpec, 500), Repositories: make([]TeamRepositorySpec, 500),
	}}
	for i := range cfg.Teams[0].Members {
		cfg.Teams[0].Members[i] = TeamMemberSpec{Username: fmt.Sprintf("user-%04d", i), Role: "member"}
	}
	for i := range cfg.Teams[0].Repositories {
		cfg.Teams[0].Repositories[i] = TeamRepositorySpec{Name: fmt.Sprintf("repo-%04d", i), Permission: "push"}
	}

	want, err := legacyEncodeYAMLForTest(cfg)
	if err != nil {
		t.Fatalf("legacy encoder returned an error: %v", err)
	}
	got, err := EncodeYAML(cfg)
	if err != nil {
		t.Fatalf("EncodeYAML returned an error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("chunked encoder changed canonical YAML bytes (got %d, want %d bytes)", len(got), len(want))
	}
}

func legacyEncodeYAMLForTest(cfg OrganizationConfig) ([]byte, error) {
	legacyDocument := &yaml.Node{
		Kind: yaml.DocumentNode,
		Content: []*yaml.Node{
			encodeOrganizationConfig(NormalizeDesiredState(cfg)),
		},
	}
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(legacyDocument); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func countYAMLNodesForTest(node *yaml.Node) int64 {
	count := int64(1)
	for _, child := range node.Content {
		count += countYAMLNodesForTest(child)
	}
	return count
}

func TestEncodeYAMLRejectsOutputAboveStructuralBudgets(t *testing.T) {
	t.Parallel()

	if _, err := encodeYAMLWithLimits(OrganizationConfig{}, 0, maxDesiredStateYAMLSourceUnits); err == nil || !strings.Contains(err.Error(), "YAML node limit") {
		t.Fatalf("encodeYAMLWithLimits node-budget error = %v, want YAML node limit", err)
	}
	if _, err := encodeYAMLWithLimits(OrganizationConfig{}, maxDesiredStateYAMLNodes, 1); err == nil || !strings.Contains(err.Error(), "source complexity limit") {
		t.Fatalf("encodeYAMLWithLimits source-budget error = %v, want source complexity limit", err)
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
		"  - name: octostate",
		"    template:",
		"      owner: orang-gaboets",
		"      name: repo-template",
		"    visibility: private",
		"    description: GitHub organization operations CLI",
		"    homepage: https://github.com/orang-gaboets/octostate",
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
		"      - name: octostate",
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
			Name:         "octostate",
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
			Name:       "octostate",
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
			Name:         "octostate",
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
				Name:       "octostate",
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

func TestEncodeYAMLPreservesExplicitMismatchedManagedOwners(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "orang-gaboets",
		Invites:      []InviteSpec{},
		Repositories: []RepositorySpec{{
			Owner:      "shared-platform",
			Name:       "octostate",
			Visibility: "private",
		}},
		Teams: []TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Repositories: []TeamRepositorySpec{{
				Owner:      "other-org",
				Name:       "octostate-infra",
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
		"- owner: shared-platform",
		"- owner: other-org",
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
			Name:         "octostate",
			Visibility:   "public",
			Description:  "GitOps CLI",
			Homepage:     "https://example.com/octostate",
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
		"homepage: https://example.com/octostate",
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
	if value, managed := repo.ManagedHomepage(); !managed || value != "https://example.com/octostate" {
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
