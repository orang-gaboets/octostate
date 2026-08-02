package topic_test

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
	topicscmd "github.com/orang-gaboets/octostate/cmd/octostate/topic"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type captureReplaceAllTopicsService struct {
	auth.MockRepoService
	replaceAllTopicsCalled bool
}

func (s *captureReplaceAllTopicsService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
	s.replaceAllTopicsCalled = true
	return topics, nil, nil
}

func TestReplaceAllTopicsNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestReplaceAllTopicsAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Replaced topics for repository o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestReplaceAllTopicsAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReplaceAllTopicsPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestReplaceAllTopicsBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestReplaceAllTopicsWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestReplaceAllTopicsDryRunSkipsTopicService(t *testing.T) {
	svc := &captureReplaceAllTopicsService{}
	c := topicscmd.ReplaceAllTopicsCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "a,b", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.replaceAllTopicsCalled {
		t.Fatalf("expected replace topics service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would replace topics for repository o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestReplaceAllTopicsToConfigReplacesTrimmedUniqueTopics(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: Repo
    visibility: public
    description: keep me
    topics: [old, keep, old]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	c := topicscmd.ReplaceAllTopicsCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " O ", "--name", "repo", "--topics", " new, keep,NEW,new ", "--to-config", configPath})
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
	if got, want := strings.Join(repository.Topics, ","), "new,keep,NEW"; got != want {
		t.Fatalf("unexpected topics: got %q want %q", got, want)
	}
	if repository.Description != "keep me" {
		t.Fatalf("unexpected unrelated field: %q", repository.Description)
	}
}

func TestReplaceAllTopicsToConfigAllowsExplicitEmptyReplacement(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: n
    visibility: public
    topics: [a, b]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Repositories[0].Topics) != 0 {
		t.Fatalf("expected topics to be empty, got %#v", cfg.Repositories[0].Topics)
	}
}

func TestReplaceAllTopicsToConfigRejectsEmptyTopicBeforeLoadingConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "a,,b", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "topic cannot be empty") {
		t.Fatalf("expected empty-topic error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}

func TestReplaceAllTopicsExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := topicscmd.ReplaceAllTopicsCmd(nil)
			c.SetArgs([]string{
				"--org", "o",
				"--name", "n",
				"--topics", "a",
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

func TestReplaceAllTopicsToConfigMissingTargetLeavesFileUnchanged(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := []byte("organization: o\nrepositories: []\n")
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	c := topicscmd.ReplaceAllTopicsCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "missing", "--topics", "a", "--to-config", configPath})
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

func TestReplaceAllTopicsToConfigAlreadyPresentIsNoOp(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := []byte(`organization: o
repositories:
  - name: n
    visibility: public
    topics: [a, b]
`)
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	c := topicscmd.ReplaceAllTopicsCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", " a,b,b ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"changed": false`) || !strings.Contains(out.String(), "No changes needed for replace topics o/n") {
		t.Fatalf("expected no-op output, got %q", out.String())
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(before) {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestReplaceAllTopicsToConfigSkipsTopicService(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: n
    visibility: public
    topics: [a]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := &captureReplaceAllTopicsService{}
	c := topicscmd.ReplaceAllTopicsCmd(svc)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "b", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.replaceAllTopicsCalled {
		t.Fatalf("expected config mode not to call topic service")
	}
}

func TestReplaceAllTopicsDryRunWinsOverToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := topicscmd.ReplaceAllTopicsCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "a", "--dry-run", "--to-config", configPath})
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
