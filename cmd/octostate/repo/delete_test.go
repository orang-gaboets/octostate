package repo_test

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
	reposcmd "github.com/orang-gaboets/octostate/cmd/octostate/repo"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureDeleteRepoService struct {
	auth.MockRepoService
	deleteCalled bool
	owner        string
	repo         string
}

func (s *captureDeleteRepoService) Delete(_ context.Context, owner, repo string) (*gh.Response, error) {
	s.deleteCalled = true
	s.owner = owner
	s.repo = repo
	return nil, nil
}

func TestDeleteRepoNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestDeleteRepoAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Deleted repository o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestDeleteRepoAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--app-id", "1", "--installation-id", "2", "--app-key-path", "path", "--org", "o", "--name", "n", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteRepoPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--app-id", "1", "--org", "o", "--name", "n", "--yes"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestDeleteRepoBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "1", "--installation-id", "2", "--app-key-path", "path", "--org", "o", "--name", "n", "--yes"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestDeleteRepoWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--yes", "--invalid-flag", "invalid-value"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestDeleteRepoRequiresYesUnlessDryRun(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	err := c.Execute()
	if !errors.Is(err, safety.ErrConfirmationRequired) {
		t.Fatalf("expected confirmation error, got %v", err)
	}
}

func TestDeleteRepoDryRunSkipsDeleteService(t *testing.T) {
	svc := &captureDeleteRepoService{}
	c := reposcmd.DeleteRepoCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--dry-run"})
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
	if !strings.Contains(got, "Dry run: would delete repository o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestDeleteRepoDryRunUsesRawValuesInOutput(t *testing.T) {
	svc := &captureDeleteRepoService{}
	c := reposcmd.DeleteRepoCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--name", " n ", "--dry-run"})
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
	if !strings.Contains(got, "Dry run: would delete repository  o / n ") {
		t.Fatalf("expected raw dry-run output, got: %q", got)
	}
}

func TestDeleteRepoUsesProvidedServiceWithRawValues(t *testing.T) {
	svc := &captureDeleteRepoService{}
	c := reposcmd.DeleteRepoCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--name", " n ", "--yes"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.deleteCalled {
		t.Fatal("expected delete service to be called")
	}
	if svc.owner != " o " || svc.repo != " n " {
		t.Fatalf("expected raw delete target \" o \"/\" n \", got %q/%q", svc.owner, svc.repo)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Deleted repository  o / n ") {
		t.Fatalf("expected raw success output, got: %q", got)
	}
}

func TestDeleteRepoToConfigRemovesRepository(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: keep
    visibility: public
  - name: Repo
    visibility: private
teams: []
`), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := &captureDeleteRepoService{}
	c := reposcmd.DeleteRepoCmd(svc)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " O ", "--name", " RePo ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.deleteCalled {
		t.Fatal("expected proposal mode not to call the delete service")
	}

	result := decodeConfigOperationOutput(t, out.String())
	if result.Message != "Proposed repository O/RePo deletion in config" {
		t.Fatalf("unexpected message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if data := result.Data; data.Owner != "O" || data.Name != "RePo" || data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected config operation data: %#v", data)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	want := `organization: o
members: []
invites: []
repositories:
  - name: keep
    visibility: public
teams: []
`
	if string(got) != want {
		t.Fatalf("unexpected config contents:\n%s\nwant:\n%s", got, want)
	}
}

func TestDeleteRepoToConfigDoesNotRequireCredentialsOrYes(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: api
    visibility: private
`), 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.DeleteRepoCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "api", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decodeConfigOperationOutput(t, out.String()).Data.Changed {
		t.Fatalf("expected config proposal to report changed=true")
	}
}

func TestDeleteRepoExplicitEmptyToConfigReturnsProposalPathError(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := reposcmd.DeleteRepoCmd(nil)
			c.SetArgs([]string{"--org", "o", "--name", "n", "--to-config", test.path})
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

func TestDeleteRepoToConfigMissingTargetLeavesFileUnchanged(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := []byte("organization: o\nrepositories:\n  - name: keep\n    visibility: public\n")
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "missing", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "repository o/missing not found in config") {
		t.Fatalf("expected missing-target error, got %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(before) {
		t.Fatalf("config changed after missing-target rejection:\n%s", got)
	}
}

func TestDeleteRepoToConfigRejectsPermissionBlockers(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{
			name: "single blocker",
			config: `organization: o
repositories:
  - name: api
    visibility: public
teams:
  - slug: platform
    name: Platform
    privacy: closed
    repositories:
      - name: api
        permission: push
`,
			wantErr: "repository o/api cannot be deleted from config while team permissions exist: platform(o/api:push)",
		},
		{
			name: "multiple blockers follow declared config order",
			config: `organization: o
repositories:
  - name: api
    visibility: public
teams:
  - slug: zebra
    name: Zebra
    privacy: closed
    repositories:
      - name: api
        permission: pull
  - slug: alpha
    name: Alpha
    privacy: closed
    repositories:
      - name: api
        permission: admin
`,
			wantErr: "repository o/api cannot be deleted from config while team permissions exist: zebra(o/api:pull), alpha(o/api:admin)",
		},
		{
			name: "omitted permission owner defaults to organization",
			config: `organization: o
repositories:
  - name: api
    visibility: public
teams:
  - slug: platform
    name: Platform
    privacy: closed
    repositories:
      - name: api
        permission: maintain
`,
			wantErr: "repository o/api cannot be deleted from config while team permissions exist: platform(o/api:maintain)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configPath := filepath.Join(t.TempDir(), "organization.yaml")
			if err := os.WriteFile(configPath, []byte(tt.config), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}

			c := reposcmd.DeleteRepoCmd(nil)
			c.SetArgs([]string{"--org", "o", "--name", "api", "--to-config", configPath})
			err = c.Execute()
			if err == nil || !strings.HasSuffix(err.Error(), tt.wantErr) {
				t.Fatalf("expected blocker error ending with %q, got %v", tt.wantErr, err)
			}

			after, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatalf("config changed after blocker rejection:\n%s", after)
			}
		})
	}
}

func TestDeleteRepoRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := reposcmd.DeleteRepoCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
