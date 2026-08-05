package members_test

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
	memberscmd "github.com/orang-gaboets/octostate/cmd/octostate/team/members"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type captureAddTeamMembershipBySlugService struct {
	auth.MockTeamsService
	addCalled bool
	org       string
	slug      string
	username  string
	role      string
}

func (s *captureAddTeamMembershipBySlugService) AddTeamMembershipBySlug(_ context.Context, org, slug, user string, opts *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
	s.addCalled = true
	s.org = org
	s.slug = slug
	s.username = user
	if opts != nil {
		s.role = opts.Role
	}
	return &gh.Membership{
		State: github.Ptr("active"),
		Role:  github.Ptr(s.role),
	}, &gh.Response{}, nil
}

func TestAddTeamMemberNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestAddTeamMemberAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTeamMemberAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTeamMemberPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestAddTeamMemberBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestAddTeamMemberWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestAddTeamMemberWithInvalidRole(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u", "--role", "invalid"})
	if err := c.Execute(); !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("expected error %v, got %v", github.ErrInvalidFieldValue, err)
	}
}

func TestAddTeamMemberWithWhitespaceUsernameRejected(t *testing.T) {
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "   "})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestAddTeamMemberDryRunSkipsAddService(t *testing.T) {
	svc := &captureAddTeamMembershipBySlugService{}
	c := memberscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "u", "--role", "maintainer", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.addCalled {
		t.Fatalf("expected add team membership service not to be called in dry-run mode")
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, `Dry run: would add user "u" to team o/s with role maintainer`) {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestAddTeamMemberWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got == "" {
		t.Fatalf("expected stdout output, got empty string")
	}
	if !strings.HasPrefix(got, "{") {
		t.Fatalf("expected JSON object output, got %q", got)
	}
}

func TestAddTeamMemberUsesProvidedServiceAndRole(t *testing.T) {
	svc := &captureAddTeamMembershipBySlugService{}
	c := memberscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--username", " u ", "--role", "maintainer"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.addCalled {
		t.Fatalf("expected add team membership service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.username != "u" {
		t.Fatalf("expected trimmed username %q, got %q", "u", svc.username)
	}
	if svc.role != "maintainer" {
		t.Fatalf("expected role %q, got %q", "maintainer", svc.role)
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, `"Role": "maintainer"`) {
		t.Fatalf("expected JSON output to contain membership role, got %q", got)
	}
}

type configOperationData struct {
	Organization string `json:"organization"`
	Slug         string `json:"slug"`
	Username     string `json:"username"`
	Role         string `json:"role"`
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

func writeMembersConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readMembersConfig(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

const membersBaseConfig = `organization: o
members:
  - username: alice
    role: member
teams:
  - slug: platform
    name: Platform
    privacy: closed
`

func TestAddTeamMemberToConfigAppendsNewMember(t *testing.T) {
	configPath := writeMembersConfig(t, membersBaseConfig)

	c := memberscmd.AddCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " o ", "--slug", " PLATFORM ", "--username", " alice ", "--role", "maintainer", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed member add for team o/PLATFORM in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	data := result.Data
	if data.Organization != "o" || data.Slug != "PLATFORM" || data.Username != "alice" || data.Role != "maintainer" {
		t.Fatalf("unexpected identity data: %#v", data)
	}
	if data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected operation data: %#v", data)
	}

	want := `organization: o
members:
  - username: alice
    role: member
invites: []
repositories: []
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: alice
        role: maintainer
`
	if got := readMembersConfig(t, configPath); got != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
}

func TestAddTeamMemberToConfigUpdatesExistingRole(t *testing.T) {
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
      - username: alice
        role: member
      - username: bob
        role: member
`)

	c := memberscmd.AddCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--role", "maintainer", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !decodeConfigOperationOutput(t, out.String()).Data.Changed {
		t.Fatal("expected changed=true for role update")
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	members := cfg.Teams[0].Members
	if len(members) != 2 {
		t.Fatalf("expected member count to stay 2, got %#v", members)
	}
	if members[0].Username != "alice" || members[0].Role != "maintainer" {
		t.Fatalf("expected alice updated in place, got %#v", members[0])
	}
	if members[1].Username != "bob" || members[1].Role != "member" {
		t.Fatalf("unrelated member changed: %#v", members[1])
	}
}

func TestAddTeamMemberToConfigUsesCanonicalUsernameFromMembers(t *testing.T) {
	configPath := writeMembersConfig(t, `organization: o
members:
  - username: Alice
    role: member
teams:
  - slug: platform
    name: Platform
    privacy: closed
`)

	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "ALICE", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Teams[0].Members[0].Username; got != "Alice" {
		t.Fatalf("expected canonical username Alice from top-level members, got %q", got)
	}
}

func TestAddTeamMemberToConfigRepeatedValueIsNoOp(t *testing.T) {
	before := `organization: o
members:
  - username: alice
    role: member
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: alice
        role: maintainer
`
	configPath := writeMembersConfig(t, before)

	c := memberscmd.AddCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--role", "maintainer", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "No changes needed for add member o/platform" {
		t.Fatalf("unexpected no-op message: %q", result.Message)
	}
	if result.Data.Changed {
		t.Fatalf("expected changed=false, got %#v", result.Data)
	}
	if got := readMembersConfig(t, configPath); got != before {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestAddTeamMemberToConfigRequiresTopLevelMember(t *testing.T) {
	before := membersBaseConfig
	configPath := writeMembersConfig(t, before)

	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "carol", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), `member "carol" must be declared in top-level members`) {
		t.Fatalf("expected missing top-level member error, got %v", err)
	}
	if got := readMembersConfig(t, configPath); got != before {
		t.Fatalf("config changed after rejection:\n%s", got)
	}
}

func TestAddTeamMemberToConfigMissingTeamLeavesFileUnchanged(t *testing.T) {
	before := membersBaseConfig
	configPath := writeMembersConfig(t, before)

	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "missing", "--username", "alice", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team o/missing not found in config") {
		t.Fatalf("expected missing-team error, got %v", err)
	}
	if got := readMembersConfig(t, configPath); got != before {
		t.Fatalf("config changed after rejection:\n%s", got)
	}
}

func TestAddTeamMemberToConfigOrganizationMismatchLeavesFileUnchanged(t *testing.T) {
	before := membersBaseConfig
	configPath := writeMembersConfig(t, before)

	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "other", "--slug", "platform", "--username", "alice", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "organization mismatch") {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
	if got := readMembersConfig(t, configPath); got != before {
		t.Fatalf("config changed after mismatch:\n%s", got)
	}
}

func TestAddTeamMemberToConfigInvalidRoleRejectedBeforeConfigAccess(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--role", "owner", "--to-config", configPath})
	if err := c.Execute(); !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("expected invalid role error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}

func TestAddTeamMemberToConfigRejectsDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "organization.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--to-config", target})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected directory rejection, got %v", err)
	}
}

func TestAddTeamMemberToConfigSkipsTeamService(t *testing.T) {
	configPath := writeMembersConfig(t, membersBaseConfig)

	svc := &captureAddTeamMembershipBySlugService{}
	c := memberscmd.AddCmd(svc)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.addCalled {
		t.Fatal("expected config mode not to call team service")
	}
}

func TestAddTeamMemberExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := memberscmd.AddCmd(nil)
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

func TestAddTeamMemberRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--username", "alice", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
