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
	reposcmd "github.com/orang-gaboets/octostate/cmd/octostate/repo"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type captureEditRepoService struct {
	auth.MockRepoService
	editCalled bool
}

func (s *captureEditRepoService) Edit(_ context.Context, _, _ string, _ *gh.Repository) (*gh.Repository, *gh.Response, error) {
	s.editCalled = true
	return &gh.Repository{}, nil, nil
}

func TestEditRepoNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestEditRepoAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{
		"--token", "t",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Edited repository o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestEditRepoAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{
		"--app-id", "123",
		"--installation-id", "456",
		"--app-key-path", "path/to/key.pem",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditRepoPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{
		"--app-id", "123",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestEditRepoBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{
		"--token", "t",
		"--app-id", "123",
		"--installation-id", "456",
		"--app-key-path", "path/to/key.pem",
		"--org", "o",
		"--name", "n",
	})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestEditRepoWithOptionalFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{
		"--token", "t",
		"--org", "o",
		"--name", "n",
		"--desc", "New description",
		"--homepage", "https://example.com",
		"--private=true",
		"--is-template=false",
	})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEditRepoWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{
		"--token", "t",
		"--org", "o",
		"--name", "n",
		"--private=invalid", // Invalid boolean value
	})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag value")
	}
}

func TestEditRepoDryRunSkipsEditService(t *testing.T) {
	svc := &captureEditRepoService{}
	c := reposcmd.EditRepo(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--desc", "d", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.editCalled {
		t.Fatalf("expected edit service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would edit repository o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestEditRepoToConfigAppliesPartialAndExplicitValues(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := `organization: o
repositories:
  - name: Repo
    visibility: public
    description: old description
    homepage: https://old.example.com
    allow_forking: true
    archived: true
    is_template: true
`
	if err := os.WriteFile(configPath, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.EditRepo(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{
		"--org", " O ",
		"--name", "repo",
		"--desc", "",
		"--homepage", "",
		"--private=false",
		"--is-template=false",
		"--archived=false",
		"--allow-forking=false",
		"--to-config", configPath,
	})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"changed": true`) {
		t.Fatalf("expected changed proposal output, got %q", out.String())
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	repository := cfg.Repositories[0]
	if repository.Description != "" || !repository.DescriptionOption().Present {
		t.Fatalf("expected explicitly cleared description, got %#v", repository.DescriptionOption())
	}
	if repository.Homepage != "" || !repository.HomepageOption().Present {
		t.Fatalf("expected explicitly cleared homepage, got %#v", repository.HomepageOption())
	}
	if repository.Visibility != "public" {
		t.Fatalf("expected public visibility, got %q", repository.Visibility)
	}
	if !repository.IsTemplateOption().Present || repository.IsTemplate {
		t.Fatalf("expected explicit false is_template, got %#v", repository.IsTemplateOption())
	}
	if !repository.ArchivedOption().Present || repository.Archived {
		t.Fatalf("expected explicit false archived, got %#v", repository.ArchivedOption())
	}
	if !repository.AllowForkingOption().Present || repository.AllowForking {
		t.Fatalf("expected explicit false allow_forking, got %#v", repository.AllowForkingOption())
	}
}

func TestEditRepoExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := reposcmd.EditRepo(nil)
			c.SetArgs([]string{
				"--org", "o",
				"--name", "n",
				"--to-config", test.path,
			})
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

func TestEditRepoToConfigMissingTargetLeavesFileUnchanged(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := []byte("organization: o\nrepositories: []\n")
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{"--org", "o", "--name", "missing", "--desc", "new", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "not found in config") {
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

func TestEditRepoToConfigNoFlagsIsNoOp(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := []byte("organization: o\nrepositories:\n  - name: n\n    visibility: public\n")
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.EditRepo(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"changed": false`) || !strings.Contains(out.String(), "No changes needed for edit o/n") {
		t.Fatalf("expected no-op output, got %q", out.String())
	}
	if got, err := os.ReadFile(configPath); err != nil {
		t.Fatal(err)
	} else if string(got) != string(before) {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestEditRepoToConfigPreservesOmittedFields(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: n
    visibility: public
    description: old description
    homepage: https://example.com
`), 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--desc", "new description", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	repository := cfg.Repositories[0]
	if repository.Description != "new description" || repository.Homepage != "https://example.com" {
		t.Fatalf("unexpected partial edit result: %#v", repository)
	}
}

func TestEditRepoToConfigMatchingExplicitValuesIsNoOp(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := []byte(`organization: o
repositories:
  - name: n
    visibility: public
    description: description
    is_template: false
`)
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.EditRepo(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--desc", "description", "--private=false", "--is-template=false", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"changed": false`) || !strings.Contains(out.String(), `"changed_fields": []`) {
		t.Fatalf("expected semantic no-op output, got %q", out.String())
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(before) {
		t.Fatalf("semantic no-op rewrote config:\n%s", got)
	}
}

func TestEditRepoToConfigAllowsPrivateRepositoryForkingField(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: n
    visibility: private
    allow_forking: true
`), 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.EditRepo(nil)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--allow-forking=false", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	repository := cfg.Repositories[0]
	if repository.Visibility != "private" || !repository.AllowForkingOption().Present || repository.AllowForking {
		t.Fatalf("unexpected private repository forking setting: %#v", repository)
	}
}

func TestEditRepoDryRunWinsOverToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := reposcmd.EditRepo(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--desc", "d", "--dry-run", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"status": "dry-run"`) {
		t.Fatalf("expected dry-run output, got %q", out.String())
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
