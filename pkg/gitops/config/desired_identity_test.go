package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDesiredStateIdentityMinimalGolden(t *testing.T) {
	t.Parallel()

	got, err := DesiredStateIdentity(OrganizationConfig{Organization: " orang-gaboets "})
	if err != nil {
		t.Fatalf("unexpected identity error: %v", err)
	}
	const want = "octostate-config-v1:sha256:8e635f6463c6c721eaa0d02abc667d4dbfcc163b203cdb30f2cbee03b6d38cfc"
	if got != want {
		t.Fatalf("unexpected desired state identity: got %q want %q", got, want)
	}
}

func TestDesiredStateIdentityIgnoresYAMLFormattingCommentsAndKeyOrder(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	firstPath := filepath.Join(configDir, "first.yaml")
	secondPath := filepath.Join(configDir, "second.yaml")
	first := "organization: orang-gaboets\nmembers: []\ninvites: []\nrepositories: []\nteams: []\n"
	second := "# Reordered keys and a comment do not change effective state.\nteams: []\nrepositories: []\ninvites: []\nmembers: []\norganization: orang-gaboets\n"
	if err := os.WriteFile(firstPath, []byte(first), 0o600); err != nil {
		t.Fatalf("write first config: %v", err)
	}
	if err := os.WriteFile(secondPath, []byte(second), 0o600); err != nil {
		t.Fatalf("write second config: %v", err)
	}

	firstConfig, err := LoadFile(firstPath)
	if err != nil {
		t.Fatalf("load first config: %v", err)
	}
	secondConfig, err := LoadFile(secondPath)
	if err != nil {
		t.Fatalf("load second config: %v", err)
	}
	firstIdentity, err := DesiredStateIdentity(firstConfig)
	if err != nil {
		t.Fatalf("identify first config: %v", err)
	}
	secondIdentity, err := DesiredStateIdentity(secondConfig)
	if err != nil {
		t.Fatalf("identify second config: %v", err)
	}
	if firstIdentity != secondIdentity {
		t.Fatalf("formatting changed identity: first=%q second=%q", firstIdentity, secondIdentity)
	}
}

func TestDesiredStateIdentityUsesNormalizedEffectiveValues(t *testing.T) {
	t.Parallel()

	canonical := validOrganizationConfig()
	spaced := validOrganizationConfig()
	spaced.Organization = " orang-gaboets "
	spaced.Members[0].Username = " alice "
	spaced.Repositories[0].Name = " octostate "
	spaced.Teams[0].Slug = " platform "

	canonicalIdentity, err := DesiredStateIdentity(canonical)
	if err != nil {
		t.Fatalf("identify canonical config: %v", err)
	}
	spacedIdentity, err := DesiredStateIdentity(spaced)
	if err != nil {
		t.Fatalf("identify normalized config: %v", err)
	}
	if canonicalIdentity != spacedIdentity {
		t.Fatalf("normalization changed identity: canonical=%q spaced=%q", canonicalIdentity, spacedIdentity)
	}
}

func TestDesiredStateIdentityPreservesListOrder(t *testing.T) {
	t.Parallel()

	first := OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []OrganizationMemberSpec{
			{Username: "alice", Role: "member"},
			{Username: "bob", Role: "member"},
		},
	}
	second := OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []OrganizationMemberSpec{
			{Username: "bob", Role: "member"},
			{Username: "alice", Role: "member"},
		},
	}

	firstIdentity, err := DesiredStateIdentity(first)
	if err != nil {
		t.Fatalf("identify first config: %v", err)
	}
	secondIdentity, err := DesiredStateIdentity(second)
	if err != nil {
		t.Fatalf("identify reordered config: %v", err)
	}
	if firstIdentity == secondIdentity {
		t.Fatalf("list reordering did not change identity: %q", firstIdentity)
	}
}

func TestDesiredStateIdentityPreservesManagedFieldPresence(t *testing.T) {
	t.Parallel()

	unmanaged := validOrganizationConfig()
	unmanaged.Repositories[0].Description = ""
	unmanaged.Repositories[0].description = OptionalString{}
	managedEmpty := validOrganizationConfig()
	managedEmpty.Repositories[0].SetManagedDescription("")

	unmanagedIdentity, err := DesiredStateIdentity(unmanaged)
	if err != nil {
		t.Fatalf("identify unmanaged config: %v", err)
	}
	managedIdentity, err := DesiredStateIdentity(managedEmpty)
	if err != nil {
		t.Fatalf("identify managed config: %v", err)
	}
	if unmanagedIdentity == managedIdentity {
		t.Fatalf("managed field presence did not change identity: %q", unmanagedIdentity)
	}
}

func TestDesiredStateIdentityRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	_, err := DesiredStateIdentity(OrganizationConfig{})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected config validation error, got %v", err)
	}
}

func TestCheckExpectedDesiredStateIdentity(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{Organization: "orang-gaboets"}
	identity, err := DesiredStateIdentity(cfg)
	if err != nil {
		t.Fatalf("compute config identity: %v", err)
	}

	tests := []struct {
		name     string
		expected string
		wantKind ExpectedDesiredStateIdentityErrorKind
	}{
		{name: "missing", wantKind: ExpectedDesiredStateIdentityErrorMissingExpected},
		{name: "blank", expected: " \t", wantKind: ExpectedDesiredStateIdentityErrorMalformed},
		{name: "malformed", expected: "not-an-identity", wantKind: ExpectedDesiredStateIdentityErrorMalformed},
		{name: "unsupported version", expected: "octostate-config-v2:sha256:8e635f6463c6c721eaa0d02abc667d4dbfcc163b203cdb30f2cbee03b6d38cfc", wantKind: ExpectedDesiredStateIdentityErrorUnsupportedVersion},
		{name: "mismatch", expected: "octostate-config-v1:sha256:0000000000000000000000000000000000000000000000000000000000000000", wantKind: ExpectedDesiredStateIdentityErrorMismatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := CheckExpectedDesiredStateIdentity(cfg, tt.expected)
			var identityErr *ExpectedDesiredStateIdentityError
			if !errors.As(err, &identityErr) || identityErr.Kind != tt.wantKind {
				t.Fatalf("expected identity error kind %q, got %#v (err=%v)", tt.wantKind, identityErr, err)
			}
		})
	}

	if err := CheckExpectedDesiredStateIdentity(cfg, identity); err != nil {
		t.Fatalf("expected matching identity to pass: %v", err)
	}
}
