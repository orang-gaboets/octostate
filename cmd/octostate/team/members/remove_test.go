package members_test

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
	memberscmd "github.com/orang-gaboets/octostate/cmd/octostate/team/members"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type captureRemoveTeamMembershipBySlugService struct {
	auth.MockTeamsService
	removeCalled bool
	org          string
	slug         string
	username     string
}

func (s *captureRemoveTeamMembershipBySlugService) RemoveTeamMembershipBySlug(_ context.Context, org, slug, user string) (*gh.Response, error) {
	s.removeCalled = true
	s.org = org
	s.slug = slug
	s.username = user
	return &gh.Response{}, nil
}

func TestRemoveTeamMemberNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestRemoveTeamMemberAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTeamMemberAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTeamMemberPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestRemoveTeamMemberBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestRemoveTeamMemberWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestRemoveTeamMemberWithWhitespaceUsernameRejected(t *testing.T) {
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "   "})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestRemoveTeamMemberDryRunSkipsRemoveService(t *testing.T) {
	svc := &captureRemoveTeamMembershipBySlugService{}
	c := memberscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "u", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.removeCalled {
		t.Fatalf("expected remove team membership service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run envelope, got %q", got)
	}
	if !strings.Contains(got, `Dry run: would remove user \"u\" from team o/s`) {
		t.Fatalf("unexpected dry-run message: %q", got)
	}
}

func TestRemoveTeamMemberWritesSuccessToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success envelope, got %q", got)
	}
	if !strings.Contains(got, `Removed user \"u\" from team o/s`) {
		t.Fatalf("unexpected success message: %q", got)
	}
	if !strings.Contains(got, `"username": "u"`) || !strings.Contains(got, `"slug": "s"`) {
		t.Fatalf("expected operation metadata, got %q", got)
	}
}

func TestRemoveTeamMemberUsesProvidedService(t *testing.T) {
	svc := &captureRemoveTeamMembershipBySlugService{}
	c := memberscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--username", " u "})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.removeCalled {
		t.Fatalf("expected remove team membership service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.username != "u" {
		t.Fatalf("expected trimmed username %q, got %q", "u", svc.username)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success envelope, got %q", got)
	}
	if !strings.Contains(got, `Removed user \"u\" from team o/s`) {
		t.Fatalf("unexpected success message: %q", got)
	}
}

func TestRemoveTeamMemberToConfigRemovesTargetedMember(t *testing.T) {
	configPath := writeMembersConfig(t, `organization: o
members:
  - username: alice
    role: member
  - username: bob
    role: member
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: aLiCe
        role: maintainer
      - username: bob
        role: member
`)

	c := memberscmd.RemoveCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " o ", "--slug", " PLATFORM ", "--username", " ALICE ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed member remove for team o/PLATFORM in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	data := result.Data
	if data.Organization != "o" || data.Slug != "PLATFORM" || data.Username != "aLiCe" || data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected operation data: %#v", data)
	}

	want := `organization: o
members:
  - username: alice
    role: member
  - username: bob
    role: member
invites: []
repositories: []
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: bob
        role: member
`
	if got := readMembersConfig(t, configPath); got != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemoveTeamMemberToConfigKeepsTopLevelMember(t *testing.T) {
	configPath := writeMembersConfig(t, `organization: o
members:
  - username: alice
    role: member
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: alice
        role: member
`)

	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Members) != 1 || cfg.Members[0].Username != "alice" {
		t.Fatalf("top-level member should be preserved, got %#v", cfg.Members)
	}
	if len(cfg.Teams[0].Members) != 0 {
		t.Fatalf("expected team membership removed, got %#v", cfg.Teams[0].Members)
	}
}

func TestRemoveTeamMemberToConfigMissingMemberIsNoOp(t *testing.T) {
	before := `organization: o
members:
  - username: Alice
    role: member
  - username: bob
    role: member
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: bob
        role: member
`
	configPath := writeMembersConfig(t, before)

	c := memberscmd.RemoveCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "aLiCe", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "No changes needed for remove member o/platform" {
		t.Fatalf("unexpected no-op message: %q", result.Message)
	}
	if result.Data.Changed {
		t.Fatalf("expected changed=false, got %#v", result.Data)
	}
	if result.Data.Username != "aLiCe" {
		t.Fatalf("expected reported username aLiCe, got %q", result.Data.Username)
	}
	if got := readMembersConfig(t, configPath); got != before {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestRemoveTeamMemberToConfigMissingTeamLeavesFileUnchanged(t *testing.T) {
	before := membersBaseConfig
	configPath := writeMembersConfig(t, before)

	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "missing", "--username", "alice", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team o/missing not found in config") {
		t.Fatalf("expected missing-team error, got %v", err)
	}
	if got := readMembersConfig(t, configPath); got != before {
		t.Fatalf("config changed after rejection:\n%s", got)
	}
}

func TestRemoveTeamMemberToConfigOrganizationMismatchLeavesFileUnchanged(t *testing.T) {
	before := membersBaseConfig
	configPath := writeMembersConfig(t, before)

	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "other", "--slug", "platform", "--username", "alice", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "organization mismatch") {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
	if got := readMembersConfig(t, configPath); got != before {
		t.Fatalf("config changed after mismatch:\n%s", got)
	}
}

func TestRemoveTeamMemberToConfigRejectsDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "organization.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--to-config", target})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected directory rejection, got %v", err)
	}
}

func TestRemoveTeamMemberToConfigSkipsTeamService(t *testing.T) {
	configPath := writeMembersConfig(t, membersBaseConfig)

	svc := &captureRemoveTeamMembershipBySlugService{}
	c := memberscmd.RemoveCmd(svc)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.removeCalled {
		t.Fatal("expected config mode not to call team service")
	}
}

func TestRemoveTeamMemberExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := memberscmd.RemoveCmd(nil)
			c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--to-config", test.path})
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

func TestRemoveTeamMemberRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
