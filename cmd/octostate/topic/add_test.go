package topic_test

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
	topicscmd "github.com/orang-gaboets/octostate/cmd/octostate/topic"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type configOperationData struct {
	Owner      string   `json:"owner"`
	Name       string   `json:"name"`
	ConfigPath string   `json:"config_path"`
	Changed    bool     `json:"changed"`
	Topics     []string `json:"topics"`
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

type captureAddTopicsService struct {
	auth.MockRepoService
	listAllTopicsCalled    bool
	replaceAllTopicsCalled bool
}

func (s *captureAddTopicsService) ListAllTopics(_ context.Context, _, _ string) ([]string, *gh.Response, error) {
	s.listAllTopicsCalled = true
	return []string{}, nil, nil
}

func (s *captureAddTopicsService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
	s.replaceAllTopicsCalled = true
	return topics, nil, nil
}

func TestAddTopicsNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestAddTopicsAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
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
	if !strings.Contains(got, "Added topics to repository o/n") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestAddTopicsAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTopicsPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestAddTopicsBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--name", "n", "--topics", "topic1,topic2"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestAddTopicsWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n", "--topics", "topic1,topic2", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestAddTopicsDryRunSkipsTopicServices(t *testing.T) {
	svc := &captureAddTopicsService{}
	c := topicscmd.AddTopicsCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "a,b", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.listAllTopicsCalled || svc.replaceAllTopicsCalled {
		t.Fatalf("expected topic services not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would add topics to repository o/n") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestAddTopicsToConfigMergesTrimmedExactTopics(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: Repo
    visibility: public
    description: keep me
    topics: [existing, duplicate, duplicate]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	c := topicscmd.AddTopicsCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", " O ", "--name", "repo", "--topics", " new, existing,new,new ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	data := result.Data
	if result.Message != "Proposed topics add for repository O/repo in config" {
		t.Fatalf("unexpected config operation message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if data.Owner != "O" || data.Name != "repo" || data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected config operation data: %#v", data)
	}
	if got, want := strings.Join(data.Topics, ","), "existing,duplicate,new"; got != want {
		t.Fatalf("unexpected output topics: got %q want %q", got, want)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	repository := cfg.Repositories[0]
	if got, want := strings.Join(repository.Topics, ","), "existing,duplicate,new"; got != want {
		t.Fatalf("unexpected topics: got %q want %q", got, want)
	}
	if repository.Description != "keep me" {
		t.Fatalf("unexpected unrelated field: %q", repository.Description)
	}
}

func TestAddTopicsToConfigRejectsInvalidTopicWithoutWriting(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	contents := []byte(`organization: o
repositories:
  - name: repo
    visibility: public
    topics: [existing]
`)
	if err := os.WriteFile(configPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}

	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "repo", "--topics", "Go", "--to-config", configPath})
	err := c.Execute()
	if err == nil {
		t.Fatal("expected invalid topic error")
	}
	if !strings.Contains(err.Error(), "repositories[0].topics[1] (invalid_repository_topic)") {
		t.Fatalf("expected invalid repository topic code, got %v", err)
	}

	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, contents) {
		t.Fatalf("config changed after invalid topic error:\n%s\nwant:\n%s", got, contents)
	}
}

func TestAddTopicsToConfigSkipsTopicService(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte(`organization: o
repositories:
  - name: n
    visibility: public
    topics: [a]
`), 0o600); err != nil {
		t.Fatal(err)
	}

	svc := &captureAddTopicsService{}
	c := topicscmd.AddTopicsCmd(svc)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "b", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if svc.listAllTopicsCalled || svc.replaceAllTopicsCalled {
		t.Fatalf("expected config mode not to call topic service")
	}
}

func TestAddTopicsToConfigRejectsEmptyTopicBeforeLoadingConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "a,,b", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "topic cannot be empty") {
		t.Fatalf("expected empty-topic error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}

func TestAddTopicsToConfigRejectsExplicitEmptyTopics(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := []byte("organization: o\nrepositories: []\n")
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics=", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "topic cannot be empty") {
		t.Fatalf("expected empty-topic error, got %v", err)
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(before) {
		t.Fatalf("config changed after empty-topic rejection:\n%s", got)
	}
}

func TestAddTopicsExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := topicscmd.AddTopicsCmd(nil)
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

func TestAddTopicsToConfigMissingTargetLeavesFileUnchanged(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := []byte("organization: o\nrepositories: []\n")
	if err := os.WriteFile(configPath, before, 0o600); err != nil {
		t.Fatal(err)
	}

	c := topicscmd.AddTopicsCmd(nil)
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

func TestAddTopicsToConfigAlreadyPresentIsNoOp(t *testing.T) {
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

	c := topicscmd.AddTopicsCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", " b,a ", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	data := result.Data
	if result.Message != "No changes needed for add topics o/n" {
		t.Fatalf("unexpected no-op message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if data.Owner != "o" || data.Name != "n" || data.ConfigPath != configPath || data.Changed {
		t.Fatalf("unexpected no-op operation data: %#v", data)
	}
	if got, want := strings.Join(data.Topics, ","), "a,b"; got != want {
		t.Fatalf("unexpected no-op output topics: got %q want %q", got, want)
	}
	if !strings.Contains(out.String(), "No changes needed for add topics o/n") {
		t.Fatalf("expected no-op message, got %q", out.String())
	}
	got, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(before) {
		t.Fatalf("no-op rewrote config:\n%s", got)
	}
}

func TestAddTopicsRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := topicscmd.AddTopicsCmd(nil)
	c.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "a", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
