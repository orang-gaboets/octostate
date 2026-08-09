package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type proposalRun struct {
	configPath string
	before     []byte
	after      []byte
	stdout     string
	stderr     string
	err        error
}

type proposalCase struct {
	name   string
	args   func(configPath string) []string
	assert func(t *testing.T, configPath string)
}

func copyProposalFixture(t *testing.T) string {
	t.Helper()

	contents, err := os.ReadFile(filepath.Join("testdata", "proposal-mode", "organization.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func installProposalClientGuard(t *testing.T) {
	t.Helper()

	auth.SetNewPATClient(func(context.Context, string) (auth.Client, error) {
		t.Fatalf("proposal mode constructed a PAT GitHub client")
		return nil, errors.New("unexpected PAT client construction")
	})
	auth.SetNewAppClient(func(int64, int64, string) (auth.Client, error) {
		t.Fatalf("proposal mode constructed a GitHub App client")
		return nil, errors.New("unexpected App client construction")
	})
	t.Cleanup(auth.ResetClients)
}

func runProposalCommand(t *testing.T, args []string) proposalRun {
	t.Helper()

	var configPath string
	for index := 0; index+1 < len(args); index++ {
		if args[index] == "--to-config" {
			configPath = args[index+1]
			break
		}
	}
	if configPath == "" {
		t.Fatal("proposal command is missing --to-config")
	}
	before, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	installProposalClientGuard(t)
	cmd := newRootCmd()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err = cmd.Execute()

	after, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatal(readErr)
	}

	return proposalRun{
		configPath: configPath,
		before:     before,
		after:      after,
		stdout:     stdout.String(),
		stderr:     stderr.String(),
		err:        err,
	}
}

func decodeProposalResult(t *testing.T, stdout string) map[string]any {
	t.Helper()

	var result map[string]any
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("decode proposal result: %v", err)
	}
	if result["status"] != "success" {
		t.Fatalf("expected success status, got %#v", result["status"])
	}
	return result
}

func TestProposalModeAllMutations(t *testing.T) {
	for _, test := range proposalSuccessCases() {
		t.Run(test.name, func(t *testing.T) {
			path := copyProposalFixture(t)
			run := runProposalCommand(t, test.args(path))
			if run.err != nil {
				t.Fatalf("unexpected error: %v", run.err)
			}
			if run.stdout == "" {
				t.Fatal("expected JSON stdout")
			}
			decodeProposalResult(t, run.stdout)
			if run.stderr != "" {
				t.Fatalf("expected empty stderr without --verbose, got %q", run.stderr)
			}
			if bytes.Equal(run.before, run.after) {
				t.Fatal("expected config mutation")
			}
			test.assert(t, path)
		})
	}
}

func TestProposalModeFailuresLeaveFilesUnchanged(t *testing.T) {
	tests := []struct {
		name string
		args func(string) []string
	}{
		{name: "missing target", args: func(path string) []string {
			return []string{"repo", "delete", "--org", "proposal-org", "--name", "missing", "--token", "unreachable", "--to-config", path}
		}},
		{name: "organization mismatch", args: func(path string) []string {
			return []string{"repo", "edit", "--org", "other-org", "--name", "api", "--desc", "ignored", "--token", "unreachable", "--to-config", path}
		}},
		{name: "post-mutation validation", args: func(path string) []string {
			return []string{"team", "create", "--org", "proposal-org", "--name", "Platform", "--token", "unreachable", "--to-config", path}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			run := runProposalCommand(t, test.args(copyProposalFixture(t)))
			if run.err == nil {
				t.Fatal("expected error")
			}
			if !bytes.Equal(run.before, run.after) {
				t.Fatal("config changed after failed proposal")
			}
		})
	}
}

func TestProposalModeOutputIsDeterministic(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "proposal-mode", "organization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "organization.yaml")

	for _, test := range proposalSuccessCases() {
		t.Run(test.name, func(t *testing.T) {
			if err := os.WriteFile(path, fixture, 0o600); err != nil {
				t.Fatal(err)
			}
			first := runProposalCommand(t, test.args(path))
			if first.err != nil {
				t.Fatalf("first run: %v", first.err)
			}

			if err := os.WriteFile(path, fixture, 0o600); err != nil {
				t.Fatal(err)
			}
			second := runProposalCommand(t, test.args(path))
			if second.err != nil {
				t.Fatalf("second run: %v", second.err)
			}

			if first.stdout != second.stdout {
				t.Fatalf("proposal output is not deterministic")
			}
			if !bytes.Equal(first.after, second.after) {
				t.Fatalf("proposal config bytes are not deterministic")
			}
		})
	}
}

func TestProposalModeNoOpsAreIdempotent(t *testing.T) {
	tests := []struct {
		name string
		args func(string) []string
	}{
		{name: "repository", args: func(path string) []string {
			return []string{"repo", "edit", "--org", "proposal-org", "--name", "api", "--desc", "Old API description", "--token", "unreachable", "--to-config", path}
		}},
		{name: "topics", args: func(path string) []string {
			return []string{"topic", "add", "--org", "proposal-org", "--name", "api", "--topics", "legacy", "--token", "unreachable", "--to-config", path}
		}},
		{name: "team", args: func(path string) []string {
			return []string{"team", "edit", "--org", "proposal-org", "--slug", "platform", "--desc", "Platform engineering", "--token", "unreachable", "--to-config", path}
		}},
		{name: "membership", args: func(path string) []string {
			return []string{"team", "members", "add", "--org", "proposal-org", "--slug", "platform", "--username", "alice", "--role", "maintainer", "--token", "unreachable", "--to-config", path}
		}},
		{name: "permission", args: func(path string) []string {
			return []string{"team", "repo", "permissions", "remove", "--org", "proposal-org", "--slug", "platform", "--repo", "web", "--token", "unreachable", "--to-config", path}
		}},
		{name: "invitation", args: func(path string) []string {
			return []string{"organization", "invite", "--org", "proposal-org", "--username", "octocat", "--token", "unreachable", "--to-config", path}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := copyProposalFixture(t)
			first := runProposalCommand(t, test.args(path))
			if first.err != nil {
				t.Fatalf("first run: %v", first.err)
			}
			result := decodeProposalResult(t, first.stdout)
			data, ok := result["data"].(map[string]any)
			if !ok || data["changed"] != false {
				t.Fatalf("expected changed=false, got %#v", result["data"])
			}
			if !bytes.Equal(first.before, first.after) {
				t.Fatal("no-op rewrote config")
			}

			second := runProposalCommand(t, test.args(path))
			if second.err != nil {
				t.Fatalf("second run: %v", second.err)
			}
			if first.stdout != second.stdout {
				t.Fatal("no-op output is not deterministic")
			}
			if !bytes.Equal(first.after, second.after) {
				t.Fatal("no-op config bytes are not deterministic")
			}
		})
	}
}

func proposalSuccessCases() []proposalCase {
	return []proposalCase{
		{name: "repository create", args: func(path string) []string {
			return []string{"repo", "create-from-template", "--org", "proposal-org", "--template-org", "proposal-org", "--template-name", "repo-template", "--name", "new-repo", "--desc", "New repository", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			cfg := loadProposalConfig(t, path)
			repo := findRepository(t, cfg, "new-repo")
			if repo.Template.Owner != "proposal-org" || repo.Template.Name != "repo-template" || repo.Description != "New repository" {
				t.Fatalf("unexpected created repository: %#v", repo)
			}
		}},
		{name: "repository edit", args: func(path string) []string {
			return []string{"repo", "edit", "--org", "proposal-org", "--name", "api", "--desc", "Updated API description", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			if repo := findRepository(t, loadProposalConfig(t, path), "api"); repo.Description != "Updated API description" {
				t.Fatalf("unexpected description: %q", repo.Description)
			}
		}},
		{name: "repository delete", args: func(path string) []string {
			return []string{"repo", "delete", "--org", "proposal-org", "--name", "legacy", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			for _, repo := range loadProposalConfig(t, path).Repositories {
				if repo.Name == "legacy" {
					t.Fatal("legacy repository remains")
				}
			}
		}},
		{name: "topic add", args: func(path string) []string {
			return []string{"topic", "add", "--org", "proposal-org", "--name", "api", "--topics", "observability", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			topics := findRepository(t, loadProposalConfig(t, path), "api").Topics
			if !contains(topics, "legacy") || !contains(topics, "observability") {
				t.Fatalf("unexpected topics: %#v", topics)
			}
		}},
		{name: "topic replace", args: func(path string) []string {
			return []string{"topic", "replace", "--org", "proposal-org", "--name", "api", "--topics", "api,stable", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			topics := findRepository(t, loadProposalConfig(t, path), "api").Topics
			if !equalStrings(topics, []string{"api", "stable"}) {
				t.Fatalf("unexpected topics: %#v", topics)
			}
		}},
		{name: "team create", args: func(path string) []string {
			return []string{"team", "create", "--org", "proposal-org", "--name", "Reliability", "--desc", "Reliability team", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			team := findTeam(t, loadProposalConfig(t, path), "reliability")
			if team.Name != "Reliability" || team.Description != "Reliability team" {
				t.Fatalf("unexpected created team: %#v", team)
			}
		}},
		{name: "team edit", args: func(path string) []string {
			return []string{"team", "edit", "--org", "proposal-org", "--slug", "platform", "--desc", "Platform operations", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			if team := findTeam(t, loadProposalConfig(t, path), "platform"); team.Description != "Platform operations" {
				t.Fatalf("unexpected description: %q", team.Description)
			}
		}},
		{name: "team delete", args: func(path string) []string {
			return []string{"team", "delete-by-slug", "--org", "proposal-org", "--slug", "legacy-team", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			for _, team := range loadProposalConfig(t, path).Teams {
				if team.Slug == "legacy-team" {
					t.Fatal("legacy team remains")
				}
			}
		}},
		{name: "membership add", args: func(path string) []string {
			return []string{"team", "members", "add", "--org", "proposal-org", "--slug", "platform", "--username", "bob", "--role", "maintainer", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			members := findTeam(t, loadProposalConfig(t, path), "platform").Members
			for _, member := range members {
				if member.Username == "bob" && member.Role == "maintainer" {
					return
				}
			}
			t.Fatalf("expected bob maintainer, got %#v", members)
		}},
		{name: "membership remove", args: func(path string) []string {
			return []string{"team", "members", "remove", "--org", "proposal-org", "--slug", "platform", "--username", "alice", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			cfg := loadProposalConfig(t, path)
			for _, member := range findTeam(t, cfg, "platform").Members {
				if member.Username == "alice" {
					t.Fatal("alice remains in platform")
				}
			}
			for _, member := range cfg.Members {
				if member.Username == "alice" {
					return
				}
			}
			t.Fatal("alice removed from top-level members")
		}},
		{name: "permission add", args: func(path string) []string {
			return []string{"team", "repo", "permissions", "add", "--org", "proposal-org", "--slug", "platform", "--repo", "web", "--permission", "admin", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			for _, repo := range findTeam(t, loadProposalConfig(t, path), "platform").Repositories {
				if repo.Name == "web" && repo.Permission == "admin" {
					return
				}
			}
			t.Fatal("expected web admin permission")
		}},
		{name: "permission remove", args: func(path string) []string {
			return []string{"team", "repo", "permissions", "remove", "--org", "proposal-org", "--slug", "platform", "--repo", "api", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			for _, repo := range findTeam(t, loadProposalConfig(t, path), "platform").Repositories {
				if repo.Name == "api" {
					t.Fatal("api permission remains")
				}
			}
		}},
		{name: "organization invite", args: func(path string) []string {
			return []string{"organization", "invite", "--org", "proposal-org", "--username", "hubber", "--token", "unreachable", "--to-config", path}
		}, assert: func(t *testing.T, path string) {
			for _, invite := range loadProposalConfig(t, path).Invites {
				if invite.Username.Value == "hubber" && invite.Role == "direct_member" {
					return
				}
			}
			t.Fatal("expected hubber direct_member invite")
		}},
	}
}

func loadProposalConfig(t *testing.T, path string) gitopsconfig.OrganizationConfig {
	t.Helper()
	cfg, err := gitopsconfig.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func findRepository(t *testing.T, cfg gitopsconfig.OrganizationConfig, name string) gitopsconfig.RepositorySpec {
	t.Helper()
	for _, repo := range cfg.Repositories {
		if repo.Name == name {
			return repo
		}
	}
	t.Fatalf("repository %q not found", name)
	return gitopsconfig.RepositorySpec{}
}

func findTeam(t *testing.T, cfg gitopsconfig.OrganizationConfig, slug string) gitopsconfig.TeamSpec {
	t.Helper()
	for _, team := range cfg.Teams {
		if team.Slug == slug {
			return team
		}
	}
	t.Fatalf("team %q not found", slug)
	return gitopsconfig.TeamSpec{}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
