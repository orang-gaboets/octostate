package config

import (
	"errors"
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

				wantInvites := []InviteSpec{{
					Username:  "octocat",
					Role:      "direct_member",
					TeamSlugs: []string{"platform"},
				}}
				if !reflect.DeepEqual(got.Invites, wantInvites) {
					t.Fatalf("unexpected invites: got %#v want %#v", got.Invites, wantInvites)
				}

				wantRepos := []RepositorySpec{{
					Owner:       "orang-gaboets",
					Name:        "repo-builder",
					Template:    TemplateSpec{Owner: "orang-gaboets", Name: "repo-template"},
					Visibility:  "private",
					Description: "GitHub organization operations CLI",
					Homepage:    "https://github.com/orang-gaboets/repo-builder",
					Topics:      []string{"go", "gitops"},
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
						Name:       "repo-builder",
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
