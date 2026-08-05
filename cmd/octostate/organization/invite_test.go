package organization_test

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
	organizationcmd "github.com/orang-gaboets/octostate/cmd/octostate/organization"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type captureInviteOrganizationService struct {
	auth.MockOrganizationService
	inviteCalled bool
}

func (s *captureInviteOrganizationService) CreateOrgInvitation(_ context.Context, _ string, _ *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	s.inviteCalled = true
	return &gh.Invitation{}, nil, nil
}

type captureInviteUserLookupService struct {
	auth.MockUserService
	getCalled bool
}

func (s *captureInviteUserLookupService) Get(_ context.Context, _ string) (*gh.User, *gh.Response, error) {
	s.getCalled = true
	return &gh.User{}, nil, nil
}

func TestInviteCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestInviteCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInviteCmdAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInviteCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestInviteCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestInviteCmdWithUsername(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, `"username": "u"`) {
		t.Fatalf("expected username in output data, got: %q", got)
	}
}

func TestInviteCmdWithUserID(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--id", "123"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, `"user_id": 123`) {
		t.Fatalf("expected user id in output data, got: %q", got)
	}
}

func TestInviteCmdWithBothUsernameAndUserID(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u", "--id", "123"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestInviteCmdWithNonPositiveUserID(t *testing.T) {
	auth.PrepareClient(t)

	tests := []struct {
		name   string
		userID string
	}{
		{name: "zero", userID: "0"},
		{name: "negative", userID: "-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := organizationcmd.InviteCmd(nil, nil)
			c.SetArgs([]string{"--token", "t", "--org", "o", "--id", tc.userID})
			if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
				t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
			}
		})
	}
}

func TestInviteCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestInviteCmdDryRunSkipsUserLookupAndOrgInvite(t *testing.T) {
	orgSvc := &captureInviteOrganizationService{}
	userSvc := &captureInviteUserLookupService{}
	c := organizationcmd.InviteCmd(orgSvc, userSvc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--username", "u", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orgSvc.inviteCalled {
		t.Fatalf("expected org invite service not to be called in dry-run mode")
	}
	if userSvc.getCalled {
		t.Fatalf("expected user lookup service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "username lookup skipped") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

type configOperationData struct {
	Organization string `json:"organization"`
	Username     string `json:"username"`
	UserID       int64  `json:"user_id"`
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

func writeInviteConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readInviteConfig(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

const inviteBaseConfig = `organization: o
invites: []
`

func TestInviteToConfigAppendsUsernameInvite(t *testing.T) {
	configPath := writeInviteConfig(t, inviteBaseConfig)

	c := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " o ", "--username", " octocat ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed organization invite username:octocat in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	data := result.Data
	if data.Organization != "o" || data.Username != "octocat" || data.Role != "direct_member" {
		t.Fatalf("unexpected identity data: %#v", data)
	}
	if data.UserID != 0 {
		t.Fatalf("expected no user_id for username invite (no live lookup), got %#v", data)
	}
	if data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected operation data: %#v", data)
	}

	want := `organization: o
members: []
invites:
  - username: octocat
    role: direct_member
repositories: []
teams: []
`
	if got := readInviteConfig(t, configPath); got != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
}

func TestInviteToConfigAppendsUserIDInvite(t *testing.T) {
	configPath := writeInviteConfig(t, inviteBaseConfig)

	c := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--id", "42", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed organization invite user_id:42 in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	data := result.Data
	if data.UserID != 42 || data.Username != "" || data.Role != "direct_member" || !data.Changed {
		t.Fatalf("unexpected operation data: %#v", data)
	}

	want := `organization: o
members: []
invites:
  - user_id: 42
    role: direct_member
repositories: []
teams: []
`
	if got := readInviteConfig(t, configPath); got != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
}

func TestInviteToConfigDuplicateUsernameIsNoOp(t *testing.T) {
	before := `organization: o
invites:
  - username: octocat
    role: direct_member
`
	configPath := writeInviteConfig(t, before)

	c := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--username", "OCTOCAT", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "No changes needed for organization invite username:OCTOCAT" {
		t.Fatalf("unexpected no-op message: %q", result.Message)
	}
	if result.Data.Changed {
		t.Fatalf("expected changed=false, got %#v", result.Data)
	}
	if got := readInviteConfig(t, configPath); got != before {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestInviteToConfigDuplicateUserIDIsNoOp(t *testing.T) {
	before := `organization: o
invites:
  - user_id: 42
    role: direct_member
`
	configPath := writeInviteConfig(t, before)

	c := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--id", "42", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if decodeConfigOperationOutput(t, out.String()).Data.Changed {
		t.Fatal("expected changed=false for duplicate user_id invite")
	}
	if got := readInviteConfig(t, configPath); got != before {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestInviteToConfigPreservesUnrelatedInvites(t *testing.T) {
	configPath := writeInviteConfig(t, `organization: o
invites:
  - email: someone@example.com
    role: direct_member
  - user_id: 7
    role: admin
`)

	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--org", "o", "--username", "octocat", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Invites) != 3 {
		t.Fatalf("expected three invites, got %#v", cfg.Invites)
	}
	if cfg.Invites[0].Email.Value != "someone@example.com" || cfg.Invites[1].UserID.Value != 7 {
		t.Fatalf("unrelated invites changed: %#v", cfg.Invites)
	}
	if cfg.Invites[2].Username.Value != "octocat" || !cfg.Invites[2].Username.Present {
		t.Fatalf("expected appended username invite, got %#v", cfg.Invites[2])
	}
}

func TestInviteToConfigDuplicateOfDeclaredMemberIsRejected(t *testing.T) {
	before := `organization: o
members:
  - username: octocat
    role: member
invites: []
`
	configPath := writeInviteConfig(t, before)

	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--org", "o", "--username", "octocat", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "duplicate_organization_member_invite") {
		t.Fatalf("expected member/invite conflict from validation, got %v", err)
	}
	if got := readInviteConfig(t, configPath); got != before {
		t.Fatalf("config changed after validation rejection:\n%s", got)
	}
}

func TestInviteToConfigOrganizationMismatchLeavesFileUnchanged(t *testing.T) {
	before := inviteBaseConfig
	configPath := writeInviteConfig(t, before)

	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--org", "other", "--username", "octocat", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "organization mismatch") {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
	if got := readInviteConfig(t, configPath); got != before {
		t.Fatalf("config changed after mismatch:\n%s", got)
	}
}

func TestInviteToConfigRejectsNonPositiveUserID(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--org", "o", "--id", "0", "--to-config", configPath})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected non-positive user ID error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}

func TestInviteToConfigRejectsDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "organization.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--org", "o", "--username", "octocat", "--to-config", target})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected directory rejection, got %v", err)
	}
}

func TestInviteToConfigSkipsGitHubServices(t *testing.T) {
	configPath := writeInviteConfig(t, inviteBaseConfig)

	orgSvc := &captureInviteOrganizationService{}
	userSvc := &captureInviteUserLookupService{}
	c := organizationcmd.InviteCmd(orgSvc, userSvc)
	c.SetArgs([]string{"--org", "o", "--username", "octocat", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if orgSvc.inviteCalled {
		t.Fatal("expected config mode not to call the invitation service")
	}
	if userSvc.getCalled {
		t.Fatal("expected config mode not to perform a username lookup")
	}
}

func TestInviteExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := organizationcmd.InviteCmd(nil, nil)
			c.SetArgs([]string{"--org", "o", "--username", "octocat", "--to-config", test.path})
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

func TestInviteRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--org", "o", "--username", "octocat", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
