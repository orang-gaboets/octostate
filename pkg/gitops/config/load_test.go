package config

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fixture     string
		wantErr     string
		assertState func(t *testing.T, got OrganizationConfig)
	}{
		{
			name:    "valid minimal config",
			fixture: "valid/minimal",
			assertState: func(t *testing.T, got OrganizationConfig) {
				t.Helper()

				if got.Organization != "orang-gaboets" {
					t.Fatalf("expected organization orang-gaboets, got %q", got.Organization)
				}
				if got.Members == nil || len(got.Members) != 0 {
					t.Fatalf("expected empty non-nil members slice, got %#v", got.Members)
				}
				if got.Invites == nil || len(got.Invites) != 0 {
					t.Fatalf("expected empty non-nil invites slice, got %#v", got.Invites)
				}
				if got.Repositories == nil || len(got.Repositories) != 0 {
					t.Fatalf("expected empty non-nil repositories slice, got %#v", got.Repositories)
				}
				if got.Teams == nil || len(got.Teams) != 0 {
					t.Fatalf("expected empty non-nil teams slice, got %#v", got.Teams)
				}
			},
		},
		{
			name:    "valid full config",
			fixture: "valid/full",
			assertState: func(t *testing.T, got OrganizationConfig) {
				t.Helper()

				if got.Organization != "orang-gaboets" {
					t.Fatalf("expected trimmed organization, got %q", got.Organization)
				}

				wantMembers := []OrganizationMemberSpec{{
					Username: "alice",
					Role:     "member",
				}}
				if !reflect.DeepEqual(got.Members, wantMembers) {
					t.Fatalf("unexpected members: got %#v want %#v", got.Members, wantMembers)
				}

				wantInvites := []InviteSpec{{
					Username:  optionalString("octocat"),
					Role:      "direct_member",
					TeamSlugs: []string{"platform"},
				}}
				if !reflect.DeepEqual(got.Invites, wantInvites) {
					t.Fatalf("unexpected invites: got %#v want %#v", got.Invites, wantInvites)
				}

				wantRepos := []RepositorySpec{{
					Owner:        "orang-gaboets",
					Name:         "octostate",
					Template:     TemplateSpec{Owner: "orang-gaboets", Name: "repo-template"},
					Visibility:   "private",
					Description:  "GitHub organization operations CLI",
					Homepage:     "https://github.com/orang-gaboets/octostate",
					Topics:       []string{"go", "gitops"},
					AllowForking: false,
					Archived:     false,
					IsTemplate:   false,
					description:  optionalString("GitHub organization operations CLI"),
					homepage:     optionalString("https://github.com/orang-gaboets/octostate"),
					allowForking: optionalBool(false),
					archived:     optionalBool(false),
					isTemplate:   optionalBool(false),
				}}
				if !reflect.DeepEqual(got.Repositories, wantRepos) {
					t.Fatalf("unexpected repositories: got %#v want %#v", got.Repositories, wantRepos)
				}

				wantTeams := []TeamSpec{{
					Slug:        "platform",
					Name:        "Platform",
					Description: "Platform engineering",
					Privacy:     "closed",
					Members: []TeamMemberSpec{{
						Username: "alice",
						Role:     "maintainer",
					}},
					Repositories: []TeamRepositorySpec{{
						Owner:      "orang-gaboets",
						Name:       "octostate",
						Permission: "push",
					}},
				}}
				if !reflect.DeepEqual(got.Teams, wantTeams) {
					t.Fatalf("unexpected teams: got %#v want %#v", got.Teams, wantTeams)
				}
			},
		},
		{
			name:    "missing organization file",
			fixture: "invalid/missing-organization-file",
			wantErr: "organization.yaml",
		},
		{
			name:    "malformed organization config",
			fixture: "invalid/malformed-organization",
			wantErr: "organization.yaml",
		},
		{
			name:    "unknown field rejected",
			fixture: "invalid/unknown-field",
			wantErr: "field unsupported not found",
		},
		{
			name:    "multiple yaml documents rejected",
			fixture: "invalid/multiple-documents",
			wantErr: "multiple YAML documents are not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			configDir := filepath.Join("testdata", tt.fixture)
			got, err := LoadDir(configDir)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.assertState != nil {
				tt.assertState(t, got)
			}
		})
	}
}

func TestLoadFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		path     string
		wantErr  string
		wantKind LoadErrorKind
	}{
		{
			name: "valid organization file",
			path: filepath.Join("testdata", "valid", "minimal", "organization.yaml"),
		},
		{
			name:     "missing organization file",
			path:     filepath.Join("testdata", "invalid", "missing-organization-file", "organization.yaml"),
			wantErr:  "organization.yaml",
			wantKind: LoadErrorMissingFile,
		},
		{
			name:     "malformed organization file",
			path:     filepath.Join("testdata", "invalid", "malformed-organization", "organization.yaml"),
			wantErr:  "organization.yaml",
			wantKind: LoadErrorDecodeFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := LoadFile(tt.path)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err)
				}
				var loadErr *LoadError
				if !errors.As(err, &loadErr) {
					t.Fatalf("expected LoadError, got %T", err)
				}
				if loadErr.Kind != tt.wantKind {
					t.Fatalf("expected load error kind %q, got %q", tt.wantKind, loadErr.Kind)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Organization != "orang-gaboets" {
				t.Fatalf("unexpected organization: %q", got.Organization)
			}
		})
	}
}

func TestLoadDirRejectsEmptyConfigDir(t *testing.T) {
	t.Parallel()

	_, err := LoadDir("   ")
	if err == nil {
		t.Fatal("expected error for empty config dir")
	}
	if !strings.Contains(err.Error(), "config directory is required") {
		t.Fatalf("unexpected error: %v", err)
	}

	var loadErr *LoadError
	if !errors.As(err, &loadErr) || loadErr.Kind != LoadErrorInvalidDir {
		t.Fatalf("expected invalid-dir load error, got %T %#v", err, loadErr)
	}
}

func TestLoadDirRejectsNonDirectory(t *testing.T) {
	t.Parallel()

	_, err := LoadDir(filepath.Join("testdata", "valid", "minimal", "organization.yaml"))
	if err == nil {
		t.Fatal("expected error for non-directory path")
	}
	if !strings.Contains(err.Error(), "is not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}

	var loadErr *LoadError
	if !errors.As(err, &loadErr) || loadErr.Kind != LoadErrorInvalidDir {
		t.Fatalf("expected invalid-dir load error, got %T %#v", err, loadErr)
	}
}

func TestLoadDirInviteUserIDZeroIsPreserved(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites:
  - user_id: 0
repositories: []
teams: []
`)

	got, err := LoadDir(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Invites) != 1 {
		t.Fatalf("expected one invite, got %#v", got.Invites)
	}
	if !got.Invites[0].UserID.Present {
		t.Fatalf("expected user_id to be marked present, got %#v", got.Invites[0])
	}
	if got.Invites[0].UserID.Null {
		t.Fatalf("expected user_id 0, got explicit null %#v", got.Invites[0].UserID)
	}
	if got.Invites[0].UserID.Value != 0 {
		t.Fatalf("expected user_id 0, got %#v", got.Invites[0].UserID.Value)
	}
}

func TestLoadDirInviteUserIDNullIsPreserved(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites:
  - user_id: null
repositories: []
teams: []
`)

	got, err := LoadDir(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Invites) != 1 {
		t.Fatalf("expected one invite, got %#v", got.Invites)
	}
	if !got.Invites[0].UserID.Present {
		t.Fatalf("expected user_id to be marked present, got %#v", got.Invites[0])
	}
	if !got.Invites[0].UserID.Null {
		t.Fatalf("expected user_id explicit null to be preserved, got %#v", got.Invites[0].UserID)
	}
}

func TestLoadDirInviteUsernameWhitespaceIsPreservedAsDeclared(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites:
  - username: "   "
repositories: []
teams: []
`)

	got, err := LoadDir(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Invites) != 1 {
		t.Fatalf("expected one invite, got %#v", got.Invites)
	}
	if !got.Invites[0].Username.Present {
		t.Fatalf("expected username to be marked present, got %#v", got.Invites[0])
	}
	if got.Invites[0].Username.Null {
		t.Fatalf("expected username whitespace to remain a scalar value, got %#v", got.Invites[0].Username)
	}
	if got.Invites[0].Username.Value != "" {
		t.Fatalf("expected normalized whitespace username to become empty string, got %#v", got.Invites[0].Username.Value)
	}
}

func TestLoadDirInviteEmailNullIsPreserved(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites:
  - email: null
repositories: []
teams: []
`)

	got, err := LoadDir(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Invites) != 1 {
		t.Fatalf("expected one invite, got %#v", got.Invites)
	}
	if !got.Invites[0].Email.Present {
		t.Fatalf("expected email to be marked present, got %#v", got.Invites[0])
	}
	if !got.Invites[0].Email.Null {
		t.Fatalf("expected email explicit null to be preserved, got %#v", got.Invites[0].Email)
	}
}

func TestLoadDirRepositoryOptionalFieldsPresenceIsPreserved(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: private
    description: "   "
    homepage: ""
    allow_forking: false
    archived: false
    is_template: false
teams: []
invites: []
`)

	got, err := LoadDir(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Repositories) != 1 {
		t.Fatalf("expected one repository, got %#v", got.Repositories)
	}

	repo := got.Repositories[0]
	if repo.Description != "" {
		t.Fatalf("expected normalized empty description, got %#v", repo.Description)
	}
	if repo.Homepage != "" {
		t.Fatalf("expected normalized empty homepage, got %#v", repo.Homepage)
	}
	if !repo.DescriptionOption().Present || repo.DescriptionOption().Null || repo.DescriptionOption().Value != "" {
		t.Fatalf("unexpected description option: %#v", repo.DescriptionOption())
	}
	if !repo.HomepageOption().Present || repo.HomepageOption().Null || repo.HomepageOption().Value != "" {
		t.Fatalf("unexpected homepage option: %#v", repo.HomepageOption())
	}
	if !repo.AllowForkingOption().Present || repo.AllowForkingOption().Null || repo.AllowForkingOption().Value {
		t.Fatalf("unexpected allow_forking option: %#v", repo.AllowForkingOption())
	}
	if !repo.ArchivedOption().Present || repo.ArchivedOption().Null || repo.ArchivedOption().Value {
		t.Fatalf("unexpected archived option: %#v", repo.ArchivedOption())
	}
	if !repo.IsTemplateOption().Present || repo.IsTemplateOption().Null || repo.IsTemplateOption().Value {
		t.Fatalf("unexpected is_template option: %#v", repo.IsTemplateOption())
	}
}

func TestLoadDirRepositoryOptionalNullsArePreserved(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: private
    description: null
    homepage: null
    allow_forking: null
    archived: null
    is_template: null
teams: []
invites: []
`)

	got, err := LoadDir(configDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got.Repositories) != 1 {
		t.Fatalf("expected one repository, got %#v", got.Repositories)
	}

	repo := got.Repositories[0]
	if !repo.DescriptionOption().Present || !repo.DescriptionOption().Null {
		t.Fatalf("expected description explicit null to be preserved, got %#v", repo.DescriptionOption())
	}
	if !repo.HomepageOption().Present || !repo.HomepageOption().Null {
		t.Fatalf("expected homepage explicit null to be preserved, got %#v", repo.HomepageOption())
	}
	if !repo.AllowForkingOption().Present || !repo.AllowForkingOption().Null {
		t.Fatalf("expected allow_forking explicit null to be preserved, got %#v", repo.AllowForkingOption())
	}
	if !repo.ArchivedOption().Present || !repo.ArchivedOption().Null {
		t.Fatalf("expected archived explicit null to be preserved, got %#v", repo.ArchivedOption())
	}
	if !repo.IsTemplateOption().Present || !repo.IsTemplateOption().Null {
		t.Fatalf("expected is_template explicit null to be preserved, got %#v", repo.IsTemplateOption())
	}
}

func TestLoadDirRejectsUnknownRepositoryField(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: private
    unsupported: true
teams: []
invites: []
`)

	_, err := LoadDir(configDir)
	if err == nil {
		t.Fatal("expected error for unknown repository field")
	}
	if !strings.Contains(err.Error(), "field unsupported not found in type config.RepositorySpec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadDirRejectsUnknownRepositoryTemplateField(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: private
    template:
      owner: orang-gaboets
      name: repo-template
      unsupported: true
teams: []
invites: []
`)

	_, err := LoadDir(configDir)
	if err == nil {
		t.Fatal("expected error for unknown repository template field")
	}
	if !strings.Contains(err.Error(), "field unsupported not found in type config.TemplateSpec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadDirRejectsUnknownInviteField(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeTestOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites:
  - username: octocat
    unsupported: true
repositories: []
teams: []
`)

	_, err := LoadDir(configDir)
	if err == nil {
		t.Fatal("expected error for unknown invite field")
	}
	if !strings.Contains(err.Error(), "field unsupported not found in type config.InviteSpec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecodeYAMLFromReader(t *testing.T) {
	t.Parallel()

	var cfg OrganizationConfig
	err := decodeYAML(strings.NewReader("organization: orang-gaboets\n"), &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	normalize(&cfg)
	if cfg.Organization != "orang-gaboets" {
		t.Fatalf("expected organization orang-gaboets, got %q", cfg.Organization)
	}
	if cfg.Members == nil || cfg.Invites == nil || cfg.Repositories == nil || cfg.Teams == nil {
		t.Fatalf("expected non-nil top-level slices, got %#v", cfg)
	}
}

func TestDecodeYAMLRejectsMultipleDocuments(t *testing.T) {
	t.Parallel()

	var cfg OrganizationConfig
	err := decodeYAML(strings.NewReader(`
organization: orang-gaboets
---
organization: other-org
`), &cfg)
	if err == nil {
		t.Fatal("expected multi-document error")
	}
	if !strings.Contains(err.Error(), "multiple YAML documents are not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeTestOrganizationYAML(t *testing.T, configDir, contents string) {
	t.Helper()

	path := filepath.Join(configDir, organizationFileName)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
