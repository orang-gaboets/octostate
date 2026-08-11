package permissions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	permissionscmd "github.com/orang-gaboets/octostate/cmd/octostate/team/repo/permissions"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type captureAddTeamRepoBySlugService struct {
	auth.MockTeamsService
	addCalled  bool
	org        string
	slug       string
	repoOwner  string
	repoName   string
	permission string
}

func (s *captureAddTeamRepoBySlugService) AddTeamRepoBySlug(_ context.Context, org, slug, owner, repo string, opts *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
	s.addCalled = true
	s.org = org
	s.slug = slug
	s.repoOwner = owner
	s.repoName = repo
	if opts != nil {
		s.permission = opts.Permission
	}
	return &gh.Response{}, nil
}

func TestAddTeamRepoPermissionNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestAddTeamRepoPermissionAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTeamRepoPermissionAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTeamRepoPermissionPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestAddTeamRepoPermissionBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestAddTeamRepoPermissionWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestAddTeamRepoPermissionWithInvalidPermission(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r", "--permission", "invalid"})
	if err := c.Execute(); !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("expected error %v, got %v", github.ErrInvalidFieldValue, err)
	}
}

func TestAddTeamRepoPermissionWithWhitespaceRepoRejected(t *testing.T) {
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "   "})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestAddTeamRepoPermissionDryRunSkipsAddService(t *testing.T) {
	svc := &captureAddTeamRepoBySlugService{}
	c := permissionscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "r", "--permission", "push", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.addCalled {
		t.Fatalf("expected add team repo permission service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would grant team o/s permission push on repository o/r") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestAddTeamRepoPermissionWritesSuccessToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r", "--permission", "maintain"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Granted team o/s permission maintain on repository o/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestAddTeamRepoPermissionUsesProvidedServiceAndDefaultsRepoOrg(t *testing.T) {
	svc := &captureAddTeamRepoBySlugService{}
	c := permissionscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--repo", " r ", "--permission", "triage"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.addCalled {
		t.Fatalf("expected add team repo permission service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.repoOwner != "o" || svc.repoName != "r" {
		t.Fatalf("expected default repo target o/r, got %q/%q", svc.repoOwner, svc.repoName)
	}
	if svc.permission != "triage" {
		t.Fatalf("expected permission %q, got %q", "triage", svc.permission)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Granted team o/s permission triage on repository o/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestAddTeamRepoPermissionRepoOrgCompatibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantRepoOwner string
	}{
		{
			name:          "omitted repo org defaults to org",
			args:          []string{"--org", " o ", "--slug", " s ", "--repo", " r ", "--permission", "admin"},
			wantRepoOwner: "o",
		},
		{
			name:          "explicit same org is accepted",
			args:          []string{"--org", " o ", "--slug", " s ", "--repo-org", "o", "--repo", " r ", "--permission", "admin"},
			wantRepoOwner: "o",
		},
		{
			name:          "trimmed and case equivalent repo org is accepted",
			args:          []string{"--org", " o ", "--slug", " s ", "--repo-org", " O ", "--repo", " r ", "--permission", "admin"},
			wantRepoOwner: "O",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &captureAddTeamRepoBySlugService{}
			c := permissionscmd.AddCmd(svc)
			var out bytes.Buffer
			c.SetOut(&out)
			c.SetArgs(tt.args)
			if err := c.Execute(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !svc.addCalled {
				t.Fatalf("expected add team repo permission service to be called")
			}
			if svc.repoOwner != tt.wantRepoOwner || svc.repoName != "r" {
				t.Fatalf("expected repo target %s/r, got %q/%q", tt.wantRepoOwner, svc.repoOwner, svc.repoName)
			}
			if svc.permission != "admin" {
				t.Fatalf("expected permission %q, got %q", "admin", svc.permission)
			}
			got := strings.TrimSpace(out.String())
			if !strings.Contains(got, `"status": "success"`) {
				t.Fatalf("expected success status output, got: %q", got)
			}
		})
	}
}

func TestAddTeamRepoPermissionRejectsCrossOrgRepoOwnerBeforeAuth(t *testing.T) {
	c := permissionscmd.AddCmd(nil)
	c.SilenceUsage = true
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo-org", "other-org", "--repo", "api", "--permission", "push"})

	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), `repository owner "other-org" must match organization "o"`) {
		t.Fatalf("expected owner mismatch error, got %v", err)
	}
	if errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("cross-org validation should happen before auth, got %v", err)
	}
}

func TestAddTeamRepoPermissionRejectsCrossOrgRepoOwnerBeforeSideEffects(t *testing.T) {
	t.Parallel()

	wantError := `repository owner "other-org" must match organization "o"`
	configPath := writePermissionsConfig(t, permissionsBaseConfig)

	tests := []struct {
		name       string
		args       []string
		configPath string
	}{
		{
			name: "live path before GitHub call",
			args: []string{"--org", "o", "--slug", "platform", "--repo-org", "other-org", "--repo", "api", "--permission", "push"},
		},
		{
			name: "dry run before output",
			args: []string{"--org", "o", "--slug", "platform", "--repo-org", "other-org", "--repo", "api", "--permission", "push", "--dry-run"},
		},
		{
			name:       "proposal path before config mutation",
			args:       []string{"--org", "o", "--slug", "platform", "--repo-org", "other-org", "--repo", "api", "--permission", "push", "--to-config", configPath},
			configPath: configPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &captureAddTeamRepoBySlugService{}
			c := permissionscmd.AddCmd(svc)
			c.SilenceUsage = true
			var out bytes.Buffer
			var errBuf bytes.Buffer
			c.SetOut(&out)
			c.SetErr(&errBuf)
			c.SetArgs(tt.args)
			err := c.Execute()
			if err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatalf("expected owner mismatch error containing %q, got %v", wantError, err)
			}
			if out.Len() != 0 {
				t.Fatalf("expected no stdout output, got %q", out.String())
			}
			if errors.Is(err, github.ErrNoValidCredentials) {
				t.Fatalf("cross-org validation should happen before auth, got %v", err)
			}
			if svc.addCalled {
				t.Fatal("cross-org validation should happen before GitHub calls")
			}
			if tt.configPath != "" {
				if got := readPermissionsConfig(t, tt.configPath); got != permissionsBaseConfig {
					t.Fatalf("config changed after cross-org rejection:\n%s", got)
				}
			}
		})
	}
}

type configOperationData struct {
	Organization string `json:"organization"`
	Slug         string `json:"slug"`
	RepoOwner    string `json:"repo_owner"`
	RepoName     string `json:"repo_name"`
	Permission   string `json:"permission"`
	ConfigPath   string `json:"config_path"`
	Changed      bool   `json:"changed"`
}

type configOperationResult struct {
	Status  string              `json:"status"`
	Message string              `json:"message"`
	Data    configOperationData `json:"data"`
}

func decodeConfigOperationOutput(t *testing.T, output string) configOperationResult {
	t.Helper()
	var result configOperationResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode config operation output: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("expected success status, got %q", result.Status)
	}
	return result
}

func writePermissionsConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readPermissionsConfig(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

const permissionsBaseConfig = `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
`

func TestAddTeamRepoPermissionToConfigAppendsNewEntry(t *testing.T) {
	configPath := writePermissionsConfig(t, permissionsBaseConfig)

	c := permissionscmd.AddCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " o ", "--slug", " PLATFORM ", "--repo", " api ", "--permission", "push", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed repository permission for team o/PLATFORM in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	data := result.Data
	if data.Organization != "o" || data.Slug != "PLATFORM" || data.RepoOwner != "o" || data.RepoName != "api" {
		t.Fatalf("unexpected identity data: %#v", data)
	}
	if data.Permission != "push" || data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected operation data: %#v", data)
	}

	want := `organization: o
members: []
invites: []
repositories: []
teams:
  - slug: platform
    name: Platform
    privacy: closed
    repositories:
      - name: api
        permission: push
`
	if got := readPermissionsConfig(t, configPath); got != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddTeamRepoPermissionToConfigUpdatesExistingEntry(t *testing.T) {
	configPath := writePermissionsConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
    repositories:
      - name: api
        permission: pull
      - name: web
        permission: push
`)

	c := permissionscmd.AddCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "API", "--permission", "admin", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !decodeConfigOperationOutput(t, out.String()).Data.Changed {
		t.Fatal("expected changed=true for permission update")
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	repositories := cfg.Teams[0].Repositories
	if len(repositories) != 2 {
		t.Fatalf("expected entry count to stay 2, got %#v", repositories)
	}
	if repositories[0].Name != "api" || repositories[0].Permission != "admin" {
		t.Fatalf("expected api updated in place, got %#v", repositories[0])
	}
	if repositories[1].Name != "web" || repositories[1].Permission != "push" {
		t.Fatalf("unrelated permission changed: %#v", repositories[1])
	}
}

func TestAddTeamRepoPermissionToConfigRepeatedValueIsNoOp(t *testing.T) {
	before := `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
    repositories:
      - name: api
        permission: push
`
	configPath := writePermissionsConfig(t, before)

	c := permissionscmd.AddCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--permission", "push", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "No changes needed for repository permission o/platform" {
		t.Fatalf("unexpected no-op message: %q", result.Message)
	}
	if result.Data.Changed {
		t.Fatalf("expected changed=false, got %#v", result.Data)
	}
	if got := readPermissionsConfig(t, configPath); got != before {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestAddTeamRepoPermissionToConfigMissingTeamLeavesFileUnchanged(t *testing.T) {
	before := permissionsBaseConfig
	configPath := writePermissionsConfig(t, before)

	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "missing", "--repo", "api", "--permission", "push", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team o/missing not found in config") {
		t.Fatalf("expected missing-team error, got %v", err)
	}
	if got := readPermissionsConfig(t, configPath); got != before {
		t.Fatalf("config changed after rejection:\n%s", got)
	}
}

func TestAddTeamRepoPermissionToConfigOrganizationMismatchLeavesFileUnchanged(t *testing.T) {
	before := permissionsBaseConfig
	configPath := writePermissionsConfig(t, before)

	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "other", "--slug", "platform", "--repo", "api", "--permission", "push", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "organization mismatch") {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
	if got := readPermissionsConfig(t, configPath); got != before {
		t.Fatalf("config changed after mismatch:\n%s", got)
	}
}

func TestAddTeamRepoPermissionToConfigInvalidPermissionRejectedBeforeConfigAccess(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--permission", "owner", "--to-config", configPath})
	if err := c.Execute(); !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("expected invalid permission error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}

func TestAddTeamRepoPermissionToConfigRejectsDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "organization.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--permission", "push", "--to-config", target})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected directory rejection, got %v", err)
	}
}

func TestAddTeamRepoPermissionToConfigSkipsTeamService(t *testing.T) {
	configPath := writePermissionsConfig(t, permissionsBaseConfig)

	svc := &captureAddTeamRepoBySlugService{}
	c := permissionscmd.AddCmd(svc)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--permission", "push", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.addCalled {
		t.Fatal("expected config mode not to call team service")
	}
}

func TestAddTeamRepoPermissionExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := permissionscmd.AddCmd(nil)
			c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--permission", "push", "--to-config", test.path})
			err := c.Execute()
			if err == nil {
				t.Fatal("expected invalid config path error")
			}
			if errors.Is(err, github.ErrNoValidCredentials) {
				t.Fatalf("explicit config mode attempted GitHub authentication: %v", err)
			}
		})
	}
}

func TestAddTeamRepoPermissionRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--permission", "push", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
