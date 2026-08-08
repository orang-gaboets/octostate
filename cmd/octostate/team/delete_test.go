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
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/safety"
	teamcmd "github.com/orang-gaboets/octostate/cmd/octostate/team"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureDeleteTeamBySlugService struct {
	auth.MockTeamsService
	deleteCalled bool
	org          string
	slug         string
}

func (s *captureDeleteTeamBySlugService) DeleteTeamBySlug(_ context.Context, org, slug string) (*gh.Response, error) {
	s.deleteCalled = true
	s.org = org
	s.slug = slug
	return nil, nil
}

func TestDeleteTeamNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestDeleteTeamAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Deleted team o/s") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestDeleteTeamAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteTeamPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--yes"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestDeleteTeamBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--yes"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestDeleteTeamWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--yes", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestDeleteTeamRequiresYesUnlessDryRun(t *testing.T) {
	auth.PrepareClient(t)
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	err := c.Execute()
	if !errors.Is(err, safety.ErrConfirmationRequired) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestDeleteTeamDryRunSkipsDeleteService(t *testing.T) {
	svc := &captureDeleteTeamBySlugService{}
	c := teamcmd.DeleteTeamBySlugCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.deleteCalled {
		t.Fatalf("expected delete service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would delete team o/s") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestDeleteTeamDryRunUsesRawValuesInOutput(t *testing.T) {
	svc := &captureDeleteTeamBySlugService{}
	c := teamcmd.DeleteTeamBySlugCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.deleteCalled {
		t.Fatalf("expected delete service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would delete team  o / s ") {
		t.Fatalf("expected raw dry-run output, got: %q", got)
	}
}

func TestDeleteTeamUsesProvidedServiceWithRawValues(t *testing.T) {
	svc := &captureDeleteTeamBySlugService{}
	c := teamcmd.DeleteTeamBySlugCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.deleteCalled {
		t.Fatal("expected delete service to be called")
	}
	if svc.org != " o " || svc.slug != " s " {
		t.Fatalf("expected raw delete target \" o \"/\" s \", got %q/%q", svc.org, svc.slug)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Deleted team  o / s ") {
		t.Fatalf("expected raw success output, got: %q", got)
	}
}

func TestDeleteTeamToConfigRemovesNestedTeam(t *testing.T) {
	configPath := writeTeamConfig(t, `organization: o
members:
  - username: alice
    role: member
  - username: bob
    role: member
  - username: carol
    role: member
invites:
  - email: docs@example.com
    role: direct_member
    team_slugs:
      - docs
repositories:
  - name: api
    visibility: private
  - name: docs-site
    visibility: public
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: bob
        role: maintainer
    repositories:
      - name: api
        permission: admin
  - slug: devs
    name: Devs
    privacy: closed
    parent_slug: platform
    members:
      - username: alice
        role: member
      - username: carol
        role: maintainer
    repositories:
      - name: api
        permission: push
      - name: docs-site
        permission: pull
  - slug: docs
    name: Docs
    privacy: closed
    members:
      - username: carol
        role: member
    repositories:
      - name: docs-site
        permission: triage
`)

	svc := &captureDeleteTeamBySlugService{}
	c := teamcmd.DeleteTeamBySlugCmd(svc)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " O ", "--slug", " DeVs ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.deleteCalled {
		t.Fatal("expected proposal mode not to call the delete service")
	}

	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed team O/DeVs deletion in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if data := result.Data; data.Organization != "O" || data.Slug != "DeVs" || data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected config operation data: %#v", data)
	}

	got := readTeamConfig(t, configPath)
	want := `organization: o
members:
  - username: alice
    role: member
  - username: bob
    role: member
  - username: carol
    role: member
invites:
  - email: docs@example.com
    role: direct_member
    team_slugs:
      - docs
repositories:
  - name: api
    visibility: private
  - name: docs-site
    visibility: public
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: bob
        role: maintainer
    repositories:
      - name: api
        permission: admin
  - slug: docs
    name: Docs
    privacy: closed
    members:
      - username: carol
        role: member
    repositories:
      - name: docs-site
        permission: triage
`
	if got != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "slug: devs") {
		t.Fatalf("expected deleted team to be absent, got:\n%s", got)
	}
	if !strings.Contains(got, "slug: platform") || !strings.Contains(got, "slug: docs") {
		t.Fatalf("expected unrelated teams to remain, got:\n%s", got)
	}
}

func TestDeleteTeamExplicitEmptyToConfigReturnsProposalPathError(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := teamcmd.DeleteTeamBySlugCmd(nil)
			c.SetArgs([]string{"--org", "o", "--slug", "platform", "--to-config", test.path})
			err := c.Execute()
			if err == nil {
				t.Fatal("expected invalid config path error")
			}
			if !strings.Contains(err.Error(), "required config file") {
				t.Fatalf("expected config path error, got %v", err)
			}
			if errors.Is(err, safety.ErrConfirmationRequired) {
				t.Fatalf("proposal mode unexpectedly reached live confirmation: %v", err)
			}
			if errors.Is(err, github.ErrNoValidCredentials) {
				t.Fatalf("explicit config mode attempted GitHub authentication: %v", err)
			}
		})
	}
}

func TestDeleteTeamToConfigMissingTargetLeavesFileUnchanged(t *testing.T) {
	configPath := writeTeamConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
`)
	before := readTeamConfig(t, configPath)

	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "missing", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "team o/missing not found in config") {
		t.Fatalf("expected missing-target error, got %v", err)
	}

	after := readTeamConfig(t, configPath)
	if after != before {
		t.Fatalf("config changed after missing-target rejection:\n%s", after)
	}
}

func TestDeleteTeamToConfigRejectsBlockers(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "child team blocker",
			config: `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
  - slug: devs
    name: Devs
    privacy: closed
    parent_slug: platform
`,
			wantErr: "team o/platform cannot be deleted from config because it would violate the config validator's child-team invariant: child team devs(parent_slug=platform)",
		},
		{
			name: "invite blocker",
			config: `organization: o
invites:
  - email: dev@example.com
    role: direct_member
    team_slugs:
      - platform
      - PLATFORM
teams:
  - slug: platform
    name: Platform
    privacy: closed
`,
			wantErr: "team o/platform cannot be deleted from config while dependencies exist: invite[0](team_slug=platform)",
		},
		{
			name: "combined blockers keep deterministic order",
			config: `organization: o
invites:
  - email: dev1@example.com
    role: direct_member
    team_slugs:
      - platform
      - docs
  - email: dev2@example.com
    role: direct_member
    team_slugs:
      - other
      - PLATFORM
teams:
  - slug: platform
    name: Platform
    privacy: closed
  - slug: devs
    name: Devs
    privacy: closed
    parent_slug: platform
  - slug: docs
    name: Docs
    privacy: closed
  - slug: other
    name: Other
    privacy: closed
  - slug: ops
    name: Ops
    privacy: closed
    parent_slug: PLATFORM
`,
			wantErr: "team o/platform cannot be deleted from config while dependencies exist: child team devs(parent_slug=platform), child team ops(parent_slug=PLATFORM), invite[0](team_slug=platform), invite[1](team_slug=PLATFORM)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := writeTeamConfig(t, tt.config)
			before := readTeamConfig(t, configPath)

			c := teamcmd.DeleteTeamBySlugCmd(nil)
			c.SetArgs([]string{"--org", "o", "--slug", "platform", "--to-config", configPath})
			err := c.Execute()
			if err == nil || !strings.HasSuffix(err.Error(), tt.wantErr) {
				t.Fatalf("expected blocker error ending with %q, got %v", tt.wantErr, err)
			}

			after := readTeamConfig(t, configPath)
			if after != before {
				t.Fatalf("config changed after blocker rejection:\n%s", after)
			}
		})
	}
}

func TestDeleteTeamToConfigDoesNotRequireCredentialsOrYes(t *testing.T) {
	configPath := writeTeamConfig(t, `organization: o
teams:
  - slug: platform
    name: Platform
    privacy: closed
`)

	c := teamcmd.DeleteTeamBySlugCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decodeConfigOperationOutput(t, out.String()).Data.Changed {
		t.Fatalf("expected config proposal to report changed=true")
	}
}

func TestDeleteTeamRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := teamcmd.DeleteTeamBySlugCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "platform", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
