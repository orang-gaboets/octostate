package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunClassifyWritesDeterministicJSONToStdout(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	goModPath := filepath.Join(repo, "go.mod")
	scanPath := filepath.Join(t.TempDir(), "scan.json")

	writeTestFile(t, goModPath, "module example.com/test\n\ngo 1.25.0\n\ntoolchain go1.25.13\n")
	writeTestFile(t, scanPath, "{"+
		`"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scanner_version":"v1.5.0","go_version":"go1.25.13"}`+
		"}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"classify", "--go-mod", goModPath, "--scan", scanPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	var got struct {
		Eligible         bool     `json:"eligible"`
		CurrentVersion   string   `json:"current_version"`
		TargetVersion    string   `json:"target_version"`
		VulnerabilityIDs []string `json:"vulnerability_ids"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal stdout: %v\n%s", err, stdout.Bytes())
	}
	if got.Eligible {
		t.Fatalf("Eligible = true, want false")
	}
	if got.CurrentVersion != "go1.25.13" {
		t.Fatalf("CurrentVersion = %q, want %q", got.CurrentVersion, "go1.25.13")
	}
	if got.TargetVersion != "" {
		t.Fatalf("TargetVersion = %q, want empty", got.TargetVersion)
	}
	if len(got.VulnerabilityIDs) != 0 {
		t.Fatalf("VulnerabilityIDs = %#v, want empty", got.VulnerabilityIDs)
	}
}

func TestRunPrintsErrorsToStderrOnly(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	goModPath := filepath.Join(repo, "go.mod")
	scanPath := filepath.Join(t.TempDir(), "scan.json")

	writeTestFile(t, goModPath, "module example.com/test\n\ngo 1.25.0\n")
	writeTestFile(t, scanPath, "{}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"classify", "--go-mod", goModPath, "--scan", scanPath}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit code = 0, want nonzero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if stderr.Len() == 0 {
		t.Fatal("expected stderr output")
	}
}

func TestRunApplyAndVerifyRoundTrip(t *testing.T) {
	t.Parallel()

	repo := toolchainremediationTestRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")
	scanPath := filepath.Join(t.TempDir(), "scan.json")

	var applyStdout bytes.Buffer
	var applyStderr bytes.Buffer
	code := run([]string{
		"apply",
		"--repo-root", repo,
		"--go-mod", goModPath,
		"--development-doc", docPath,
		"--target", "go1.25.14",
	}, &applyStdout, &applyStderr)
	if code != 0 {
		t.Fatalf("apply exit code = %d, want 0, stderr=%q", code, applyStderr.String())
	}
	if applyStderr.Len() != 0 {
		t.Fatalf("expected empty apply stderr, got %q", applyStderr.String())
	}

	writeTestFile(t, scanPath, "{"+
		`"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scanner_version":"v1.5.0","go_version":"go1.25.14"}`+
		"}\n")

	var verifyStdout bytes.Buffer
	var verifyStderr bytes.Buffer
	code = run([]string{
		"verify",
		"--repo-root", repo,
		"--go-mod", goModPath,
		"--development-doc", docPath,
		"--scan", scanPath,
		"--target", "go1.25.14",
	}, &verifyStdout, &verifyStderr)
	if code != 0 {
		t.Fatalf("verify exit code = %d, want 0, stderr=%q", code, verifyStderr.String())
	}
	if verifyStderr.Len() != 0 {
		t.Fatalf("expected empty verify stderr, got %q", verifyStderr.String())
	}
}

func TestRunApplyRejectsMalformedDirectiveSyntax(t *testing.T) {
	t.Parallel()

	repo := t.TempDir()
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")

	writeTestFile(t, goModPath, "module example.com/test\n\ngo go1.25.0\n\ntoolchain 1.25.13\n")
	writeTestFile(t, docPath, "placeholder\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{
		"apply",
		"--repo-root", repo,
		"--go-mod", goModPath,
		"--development-doc", docPath,
		"--target", "go1.25.14",
	}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("exit code = 0, want nonzero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected empty stdout, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "invalid go directive") {
		t.Fatalf("stderr = %q, want invalid go directive", stderr.String())
	}
}

func toolchainremediationTestRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	writeTestFile(t, filepath.Join(repo, "go.mod"), strings.Join([]string{
		"module example.com/test",
		"",
		"go 1.25.0",
		"",
		"toolchain go1.25.13",
		"",
		"require github.com/spf13/cobra v1.10.2",
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(repo, "README.md"), "test repo\n")
	writeTestFile(t, filepath.Join(repo, "docs", "maintainers", "development.md"), strings.Join([]string{
		"# Development",
		"",
		"2. Install Go 1.25.0 or higher:",
		"",
		"   ```bash",
		"   go version",
		"   ```",
		"",
		"   The module includes a `toolchain go1.25.13` directive, so Go commands will",
		"   automatically prefer the patched 1.25.13 toolchain when toolchain switching",
		"   is enabled.",
		"",
	}, "\n"))

	runTestGit(t, repo, "init")
	runTestGit(t, repo, "config", "user.email", "codex@example.com")
	runTestGit(t, repo, "config", "user.name", "Codex")
	runTestGit(t, repo, "add", ".")
	runTestGit(t, repo, "commit", "-m", "initial")

	return repo
}

func runTestGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = filteredGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func filteredGitEnv() []string {
	env := os.Environ()
	out := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
