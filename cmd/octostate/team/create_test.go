package team_test

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
	teamcmd "github.com/orang-gaboets/octostate/cmd/octostate/team"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type captureCreateTeamService struct {
	auth.MockTeamsService
	createCalled bool
}

func (s *captureCreateTeamService) CreateTeam(_ context.Context, _ string, _ gh.NewTeam) (*gh.Team, *gh.Response, error) {
	s.createCalled = true
	return &gh.Team{}, nil, nil
}

func TestCreateTeamNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestCreateTeamAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Created team o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestCreateTeamAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateTeamPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestCreateTeamBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestCreateTeamWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestCreateTeamDryRunSkipsCreateService(t *testing.T) {
	svc := &captureCreateTeamService{}
	c := teamcmd.CreateTeamCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.createCalled {
		t.Fatalf("expected create team service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would create team o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

type configOperationData struct {
	Organization  string   `json:"organization"`
	Name          string   `json:"name"`
	Slug          string   `json:"slug"`
	Privacy       string   `json:"privacy"`
	ParentSlug    string   `json:"parent_slug"`
	ConfigPath    string   `json:"config_path"`
	Changed       bool     `json:"changed"`
	ChangedFields []string `json:"changed_fields"`
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

func writeTeamConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readTeamConfig(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func TestCreateTeamToConfigTopLevelDefaultClosed(t *testing.T) {
	configPath := writeTeamConfig(t, "organization: o\nteams: []\n")

	c := teamcmd.CreateTeamCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " o ", "--name", " Platform Team ", "--desc", " Team description ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed team o/platform-team in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	data := result.Data
	if data.Organization != "o" || data.Name != "Platform Team" || data.Slug != "platform-team" {
		t.Fatalf("unexpected identity data: %#v", data)
	}
	if data.Privacy != "closed" || data.ParentSlug != "" || data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected operation data: %#v", data)
	}

	want := `organization: o
invites: []
repositories: []
teams:
  - slug: platform-team
    name: Platform Team
    description: Team description
    privacy: closed
`
	if got := readTeamConfig(t, configPath); got != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
}

func TestCreateTeamToConfigSecretPrivacy(t *testing.T) {
	configPath := writeTeamConfig(t, "organization: o\nteams: []\n")

	c := teamcmd.CreateTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "Sec Team", "--secret=true", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Data.Privacy != "secret" {
		t.Fatalf("expected secret privacy, got %q", result.Data.Privacy)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Teams[0].Privacy != "secret" {
		t.Fatalf("unexpected team privacy: %#v", cfg.Teams[0])
	}
}

func TestCreateTeamToConfigEmptyDescriptionOmitted(t *testing.T) {
	configPath := writeTeamConfig(t, "organization: o\nteams: []\n")

	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "Core", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	got := readTeamConfig(t, configPath)
	if strings.Contains(got, "description:") {
		t.Fatalf("expected description to be omitted, got:\n%s", got)
	}
	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Teams[0].Description != "" {
		t.Fatalf("expected empty description, got %q", cfg.Teams[0].Description)
	}
}

func TestCreateTeamToConfigChildTeamCanonicalizesParent(t *testing.T) {
	configPath := writeTeamConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
`)

	c := teamcmd.CreateTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "Platform Devs", "--parent", " PLATFORM ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Data.ParentSlug != "platform" {
		t.Fatalf("expected canonical parent slug platform, got %q", result.Data.ParentSlug)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Teams) != 2 || cfg.Teams[1].Slug != "platform-devs" || cfg.Teams[1].ParentSlug != "platform" {
		t.Fatalf("unexpected child team: %#v", cfg.Teams)
	}
}

func TestCreateTeamToConfigMissingParentLeavesFileUnchanged(t *testing.T) {
	before := "organization: o\nteams: []\n"
	configPath := writeTeamConfig(t, before)

	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "Devs", "--parent", "missing", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "parent team missing not found in config") {
		t.Fatalf("expected missing-parent error, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != before {
		t.Fatalf("config changed after missing-parent rejection:\n%s", got)
	}
}

func TestCreateTeamToConfigEmptyParentRejected(t *testing.T) {
	before := "organization: o\nteams: []\n"
	configPath := writeTeamConfig(t, before)

	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "Devs", "--parent", " ", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "parent team slug cannot be empty") {
		t.Fatalf("expected empty-parent error, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != before {
		t.Fatalf("config changed after empty-parent rejection:\n%s", got)
	}
}

func TestCreateTeamToConfigDuplicateSlugRejected(t *testing.T) {
	before := `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
`
	configPath := writeTeamConfig(t, before)

	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", " PLATFORM ", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team platform already exists in config") {
		t.Fatalf("expected duplicate-slug error, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != before {
		t.Fatalf("config changed after duplicate rejection:\n%s", got)
	}
}

func TestCreateTeamToConfigInvalidNameRejected(t *testing.T) {
	before := "organization: o\nteams: []\n"
	configPath := writeTeamConfig(t, before)

	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "***", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "does not produce a valid team slug") {
		t.Fatalf("expected invalid-name error, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != before {
		t.Fatalf("config changed after invalid-name rejection:\n%s", got)
	}
}

func TestCreateTeamToConfigOrganizationMismatchLeavesFileUnchanged(t *testing.T) {
	before := "organization: o\nteams: []\n"
	configPath := writeTeamConfig(t, before)

	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--org", "other", "--name", "Devs", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "organization mismatch") {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != before {
		t.Fatalf("config changed after organization mismatch:\n%s", got)
	}
}

func TestCreateTeamToConfigRejectsDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "organization.yaml")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}

	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "Devs", "--to-config", target})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected directory rejection, got %v", err)
	}
}

func TestCreateTeamToConfigSkipsTeamService(t *testing.T) {
	configPath := writeTeamConfig(t, "organization: o\nteams: []\n")

	svc := &captureCreateTeamService{}
	c := teamcmd.CreateTeamCmd(svc)
	c.SetArgs([]string{"--org", "o", "--name", "Devs", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.createCalled {
		t.Fatal("expected config mode not to call team service")
	}
}

func TestCreateTeamExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := teamcmd.CreateTeamCmd(nil)
			c.SetArgs([]string{"--org", "o", "--name", "Devs", "--to-config", test.path})
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

func TestCreateTeamRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := teamcmd.CreateTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "Devs", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
