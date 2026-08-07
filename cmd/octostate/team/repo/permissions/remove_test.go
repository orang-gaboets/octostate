package permissions_test

import (
	"bytes"
	"context"
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

type captureRemoveTeamRepoBySlugService struct {
	auth.MockTeamsService
	removeCalled bool
	org          string
	slug         string
	repoOwner    string
	repoName     string
}

func (s *captureRemoveTeamRepoBySlugService) RemoveTeamRepoBySlug(_ context.Context, org, slug, owner, repo string) (*gh.Response, error) {
	s.removeCalled = true
	s.org = org
	s.slug = slug
	s.repoOwner = owner
	s.repoName = repo
	return &gh.Response{}, nil
}

func TestRemoveTeamRepoPermissionNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestRemoveTeamRepoPermissionAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTeamRepoPermissionAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTeamRepoPermissionPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestRemoveTeamRepoPermissionBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestRemoveTeamRepoPermissionWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestRemoveTeamRepoPermissionWithWhitespaceRepoRejected(t *testing.T) {
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "   "})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestRemoveTeamRepoPermissionDryRunSkipsRemoveService(t *testing.T) {
	svc := &captureRemoveTeamRepoBySlugService{}
	c := permissionscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "r", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.removeCalled {
		t.Fatalf("expected remove team repo permission service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would remove team o/s access to repository o/r") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestRemoveTeamRepoPermissionWritesSuccessToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Removed team o/s access to repository o/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestRemoveTeamRepoPermissionUsesProvidedServiceAndDefaultsRepoOrg(t *testing.T) {
	svc := &captureRemoveTeamRepoBySlugService{}
	c := permissionscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--repo", " r "})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.removeCalled {
		t.Fatalf("expected remove team repo permission service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.repoOwner != "o" || svc.repoName != "r" {
		t.Fatalf("expected default repo target o/r, got %q/%q", svc.repoOwner, svc.repoName)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Removed team o/s access to repository o/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestRemoveTeamRepoPermissionUsesProvidedServiceAndExplicitRepoOrg(t *testing.T) {
	svc := &captureRemoveTeamRepoBySlugService{}
	c := permissionscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--repo-org", " ro ", "--repo", " r "})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.removeCalled {
		t.Fatalf("expected remove team repo permission service to be called")
	}
	if svc.repoOwner != "ro" || svc.repoName != "r" {
		t.Fatalf("expected explicit repo target ro/r, got %q/%q", svc.repoOwner, svc.repoName)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Removed team o/s access to repository ro/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestRemoveTeamRepoPermissionToConfigRemovesTargetedEntry(t *testing.T) {
	configPath := writePermissionsConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
    repositories:
      - name: api
        permission: push
      - name: web
        permission: pull
`)

	c := permissionscmd.RemoveCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " o ", "--slug", " PLATFORM ", "--repo", " API ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed repository permission removal for team o/PLATFORM in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	data := result.Data
	if data.Organization != "o" || data.Slug != "PLATFORM" || data.RepoOwner != "o" || data.RepoName != "API" || !data.Changed {
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
      - name: web
        permission: pull
`
	if got := readPermissionsConfig(t, configPath); got != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveTeamRepoPermissionToConfigMatchesExplicitRepoOwner(t *testing.T) {
	configPath := writePermissionsConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
    repositories:
      - owner: other-org
        name: api
        permission: push
      - name: api
        permission: pull
`)

	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo-org", "other-org", "--repo", "api", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	repositories := cfg.Teams[0].Repositories
	if len(repositories) != 1 {
		t.Fatalf("expected one remaining entry, got %#v", repositories)
	}
	if repositories[0].Owner != "o" || repositories[0].Name != "api" || repositories[0].Permission != "pull" {
		t.Fatalf("expected same-org entry preserved, got %#v", repositories[0])
	}
}

func TestRemoveTeamRepoPermissionToConfigMissingEntryIsNoOp(t *testing.T) {
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

	c := permissionscmd.RemoveCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "missing", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "No changes needed for repository permission removal o/platform" {
		t.Fatalf("unexpected no-op message: %q", result.Message)
	}
	if result.Data.Changed {
		t.Fatalf("expected changed=false, got %#v", result.Data)
	}
	if got := readPermissionsConfig(t, configPath); got != before {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestRemoveTeamRepoPermissionToConfigMissingTeamLeavesFileUnchanged(t *testing.T) {
	before := permissionsBaseConfig
	configPath := writePermissionsConfig(t, before)

	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "missing", "--repo", "api", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team o/missing not found in config") {
		t.Fatalf("expected missing-team error, got %v", err)
	}
	if got := readPermissionsConfig(t, configPath); got != before {
		t.Fatalf("config changed after rejection:\n%s", got)
	}
}

func TestRemoveTeamRepoPermissionToConfigOrganizationMismatchLeavesFileUnchanged(t *testing.T) {
	before := permissionsBaseConfig
	configPath := writePermissionsConfig(t, before)

	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "other", "--slug", "platform", "--repo", "api", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "organization mismatch") {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
	if got := readPermissionsConfig(t, configPath); got != before {
		t.Fatalf("config changed after mismatch:\n%s", got)
	}
}

func TestRemoveTeamRepoPermissionToConfigRejectsDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "organization.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--to-config", target})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected directory rejection, got %v", err)
	}
}

func TestRemoveTeamRepoPermissionToConfigSkipsTeamService(t *testing.T) {
	configPath := writePermissionsConfig(t, permissionsBaseConfig)

	svc := &captureRemoveTeamRepoBySlugService{}
	c := permissionscmd.RemoveCmd(svc)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.removeCalled {
		t.Fatal("expected config mode not to call team service")
	}
}

func TestRemoveTeamRepoPermissionExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := permissionscmd.RemoveCmd(nil)
			c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--to-config", test.path})
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

func TestRemoveTeamRepoPermissionRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--repo", "api", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
