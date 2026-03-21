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
	if roundTripped.Invites == nil || roundTripped.Repositories == nil || roundTripped.Teams == nil {
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
