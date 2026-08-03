package team_test

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
	teamcmd "github.com/orang-gaboets/octostate/cmd/octostate/team"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type captureEditTeamBySlugService struct {
	auth.MockTeamsService
	editCalled bool
}

func (s *captureEditTeamBySlugService) EditTeamBySlug(_ context.Context, _, _ string, _ gh.NewTeam, _ bool) (*gh.Team, *gh.Response, error) {
	s.editCalled = true
	return &gh.Team{}, nil, nil
}

func TestEditTeamNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestEditTeamAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditTeamAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditTeamPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestEditTeamBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestEditTeamWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestEditTeamParentAndClearParentConflict(t *testing.T) {
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--parent", "parent", "--clear-parent"})
	err := c.Execute()
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("expected error %v, got %v", github.ErrValidationFailed, err)
	}
}

func TestEditTeamEmptyParentSlugRejected(t *testing.T) {
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--parent", "   "})
	err := c.Execute()
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestEditTeamDryRunSkipsEditService(t *testing.T) {
	svc := &captureEditTeamBySlugService{}
	c := teamcmd.EditTeamCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--desc", "d", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.editCalled {
		t.Fatalf("expected edit team service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would edit team o/s") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestEditTeamWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Edited team o/s") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

const editTeamBaseConfig = `organization: o
teams:
  - slug: platform
    name: Platform
    description: Old description
    privacy: closed
`

func TestEditTeamToConfigAppliesPartialFields(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " o ", "--slug", " PLATFORM ", "--desc", "New description", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed team o/PLATFORM edit in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	data := result.Data
	if data.Organization != "o" || data.Slug != "PLATFORM" || data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected operation data: %#v", data)
	}
	if got, want := strings.Join(data.ChangedFields, ","), "desc"; got != want {
		t.Fatalf("unexpected changed fields: got %q want %q", got, want)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	team := cfg.Teams[0]
	if team.Description != "New description" || team.Name != "Platform" || team.Privacy != "closed" || team.Slug != "platform" {
		t.Fatalf("unexpected team after partial edit: %#v", team)
	}
}

func TestEditTeamToConfigDisplayNameEditPreservingSlug(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--name", " PLATFORM ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if got, want := strings.Join(result.Data.ChangedFields, ","), "name"; got != want {
		t.Fatalf("unexpected changed fields: got %q want %q", got, want)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Teams[0].Name != "PLATFORM" || cfg.Teams[0].Slug != "platform" {
		t.Fatalf("unexpected team after rename: %#v", cfg.Teams[0])
	}
}

func TestEditTeamToConfigRejectsSlugChangingRename(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--name", "New Name", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "would change team slug from platform to new-name") {
		t.Fatalf("expected slug-change rejection, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != editTeamBaseConfig {
		t.Fatalf("config changed after slug-change rejection:\n%s", got)
	}
}

func TestEditTeamToConfigClearsDescription(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--desc", "", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if got, want := strings.Join(result.Data.ChangedFields, ","), "desc"; got != want {
		t.Fatalf("unexpected changed fields: got %q want %q", got, want)
	}
	got := readTeamConfig(t, configPath)
	if strings.Contains(got, "description:") {
		t.Fatalf("expected description removed from file, got:\n%s", got)
	}
}

func TestEditTeamToConfigSecretTrue(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--secret=true", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if got, want := strings.Join(result.Data.ChangedFields, ","), "secret"; got != want {
		t.Fatalf("unexpected changed fields: got %q want %q", got, want)
	}
	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Teams[0].Privacy != "secret" {
		t.Fatalf("expected secret privacy, got %q", cfg.Teams[0].Privacy)
	}
}

func TestEditTeamToConfigSecretFalse(t *testing.T) {
	configPath := writeTeamConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: secret
`)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--secret=false", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if got, want := strings.Join(result.Data.ChangedFields, ","), "secret"; got != want {
		t.Fatalf("unexpected changed fields: got %q want %q", got, want)
	}
	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Teams[0].Privacy != "closed" {
		t.Fatalf("expected closed privacy, got %q", cfg.Teams[0].Privacy)
	}
}

func TestEditTeamToConfigAssignsCanonicalParent(t *testing.T) {
	configPath := writeTeamConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
  - slug: devs
    name: Devs
    privacy: closed
`)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "devs", "--parent", " PLATFORM ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if got, want := strings.Join(result.Data.ChangedFields, ","), "parent"; got != want {
		t.Fatalf("unexpected changed fields: got %q want %q", got, want)
	}
	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Teams[1].ParentSlug != "platform" {
		t.Fatalf("expected canonical parent slug platform, got %q", cfg.Teams[1].ParentSlug)
	}
}

func TestEditTeamToConfigClearsParentOnly(t *testing.T) {
	configPath := writeTeamConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
  - slug: devs
    name: Devs
    description: Keep me
    privacy: secret
    parent_slug: platform
`)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "devs", "--clear-parent", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if got, want := strings.Join(result.Data.ChangedFields, ","), "clear-parent"; got != want {
		t.Fatalf("unexpected changed fields: got %q want %q", got, want)
	}
	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	team := cfg.Teams[1]
	if team.ParentSlug != "" {
		t.Fatalf("expected parent cleared, got %q", team.ParentSlug)
	}
	if team.Description != "Keep me" || team.Privacy != "secret" || team.Name != "Devs" {
		t.Fatalf("unrelated fields changed: %#v", team)
	}
}

func TestEditTeamToConfigParentAndClearParentRejected(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "devs", "--parent", "platform", "--clear-parent", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "cannot set --parent and --clear-parent together") {
		t.Fatalf("expected conflicting parent flags error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}

func TestEditTeamToConfigMissingParentLeavesFileUnchanged(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--parent", "missing", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "parent team missing not found in config") {
		t.Fatalf("expected missing-parent error, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != editTeamBaseConfig {
		t.Fatalf("config changed after missing-parent rejection:\n%s", got)
	}
}

func TestEditTeamToConfigSelfParentRejectedByValidation(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--parent", "platform", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team_parent_cycle") {
		t.Fatalf("expected parent-cycle validation error, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != editTeamBaseConfig {
		t.Fatalf("config changed after cycle rejection:\n%s", got)
	}
}

func TestEditTeamToConfigMissingTargetLeavesFileUnchanged(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "missing", "--desc", "d", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team o/missing not found in config") {
		t.Fatalf("expected missing-target error, got %v", err)
	}
	if got := readTeamConfig(t, configPath); got != editTeamBaseConfig {
		t.Fatalf("config changed after missing-target rejection:\n%s", got)
	}
}

func TestEditTeamToConfigNoFlagsIsNoOp(t *testing.T) {
	before := "# keep this comment\norganization: o\nteams:\n  - slug: platform\n    name: Platform\n    privacy: closed\n"
	configPath := writeTeamConfig(t, before)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "No changes needed for edit o/platform" {
		t.Fatalf("unexpected no-op message: %q", result.Message)
	}
	if result.Data.Changed || len(result.Data.ChangedFields) != 0 {
		t.Fatalf("unexpected no-op operation data: %#v", result.Data)
	}
	if got := readTeamConfig(t, configPath); got != before {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestEditTeamToConfigRepeatedValuesIsNoOp(t *testing.T) {
	before := `organization: o
teams:
  - slug: platform
    name: Platform
    description: Old description
    privacy: closed
  - slug: devs
    name: Devs
    privacy: closed
    parent_slug: platform
`
	configPath := writeTeamConfig(t, before)

	c := teamcmd.EditTeamCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "devs", "--name", "Devs", "--secret=false", "--parent", "platform", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "No changes needed for edit o/devs" {
		t.Fatalf("unexpected no-op message: %q", result.Message)
	}
	if result.Data.Changed || len(result.Data.ChangedFields) != 0 {
		t.Fatalf("unexpected no-op operation data: %#v", result.Data)
	}
	if got := readTeamConfig(t, configPath); got != before {
		t.Fatalf("semantic no-op rewrote config:\n%s", got)
	}
}

func TestEditTeamToConfigEmptyNameRejected(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--name", " ", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team name cannot be empty") {
		t.Fatalf("expected empty-name error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}

func TestEditTeamToConfigSkipsTeamService(t *testing.T) {
	configPath := writeTeamConfig(t, editTeamBaseConfig)

	svc := &captureEditTeamBySlugService{}
	c := teamcmd.EditTeamCmd(svc)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--desc", "d", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.editCalled {
		t.Fatal("expected config mode not to call team service")
	}
}

func TestEditTeamExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := teamcmd.EditTeamCmd(nil)
			c.SetArgs([]string{"--org", "o", "--slug", "platform", "--desc", "d", "--to-config", test.path})
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

func TestEditTeamRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := teamcmd.EditTeamCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--desc", "d", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
