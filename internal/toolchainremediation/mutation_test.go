package toolchainremediation

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyCandidateReplacesExactTargets(t *testing.T) {
	t.Parallel()

	repo := newMutationTestRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")

	got, err := ApplyCandidate(repo, goModPath, docPath, GoVersion{Major: 1, Minor: 25, Patch: 14})
	if err != nil {
		t.Fatalf("ApplyCandidate returned error: %v", err)
	}

	if got.CurrentVersion != "go1.25.13" {
		t.Fatalf("CurrentVersion = %q, want %q", got.CurrentVersion, "go1.25.13")
	}
	if got.TargetVersion != "go1.25.14" {
		t.Fatalf("TargetVersion = %q, want %q", got.TargetVersion, "go1.25.14")
	}
	if joinPaths(got.ChangedFiles) != "docs/maintainers/development.md\ngo.mod" {
		t.Fatalf("ChangedFiles = %#v", got.ChangedFiles)
	}

	goModBytes := mustReadFile(t, goModPath)
	if !bytes.Contains(goModBytes, []byte("go 1.25.0")) {
		t.Fatalf("expected go directive to remain unchanged, got %q", goModBytes)
	}
	if !bytes.Contains(goModBytes, []byte("toolchain go1.25.14")) {
		t.Fatalf("expected toolchain directive update, got %q", goModBytes)
	}
	if bytes.Contains(goModBytes, []byte("toolchain go1.25.13")) {
		t.Fatalf("expected old toolchain directive to be removed, got %q", goModBytes)
	}

	docBytes := mustReadFile(t, docPath)
	if !bytes.Contains(docBytes, []byte("   The module includes a `toolchain go1.25.14` directive")) {
		t.Fatalf("expected documentation directive update, got %q", docBytes)
	}
	if !bytes.Contains(docBytes, []byte("   automatically prefer the patched 1.25.14 toolchain when toolchain switching")) {
		t.Fatalf("expected documentation prose update, got %q", docBytes)
	}
}

func TestApplyCandidateRejectsMalformedGoMod(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		goMod string
		want  string
	}{
		{
			name: "missing toolchain directive",
			goMod: strings.Join([]string{
				"module example.com/test",
				"",
				"go 1.25.0",
				"",
			}, "\n"),
			want: "missing toolchain directive",
		},
		{
			name: "duplicate go directive",
			goMod: strings.Join([]string{
				"module example.com/test",
				"",
				"go 1.25.0",
				"go 1.25.0",
				"",
				"toolchain go1.25.13",
				"",
			}, "\n"),
			want: "duplicate go directive",
		},
		{
			name: "malformed toolchain directive",
			goMod: strings.Join([]string{
				"module example.com/test",
				"",
				"go 1.25.0",
				"",
				"toolchain default",
				"",
			}, "\n"),
			want: "invalid toolchain directive",
		},
		{
			name: "go directive must not use go prefix",
			goMod: strings.Join([]string{
				"module example.com/test",
				"",
				"go go1.25.0",
				"",
				"toolchain go1.25.13",
				"",
			}, "\n"),
			want: "invalid go directive",
		},
		{
			name: "toolchain directive requires go prefix",
			goMod: strings.Join([]string{
				"module example.com/test",
				"",
				"go 1.25.0",
				"",
				"toolchain 1.25.13",
				"",
			}, "\n"),
			want: "invalid toolchain directive",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := newMutationTestRepoWithGoMod(t, tc.goMod)
			_, err := ApplyCandidate(
				repo,
				filepath.Join(repo, "go.mod"),
				filepath.Join(repo, "docs", "maintainers", "development.md"),
				GoVersion{Major: 1, Minor: 25, Patch: 14},
			)
			if err == nil {
				t.Fatal("ApplyCandidate returned nil error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestApplyCandidateAllowsIndependentGoAndToolchainVersions(t *testing.T) {
	t.Parallel()

	repo := newMutationTestRepoWithGoMod(t, strings.Join([]string{
		"module example.com/test",
		"",
		"go 1.24.0",
		"",
		"toolchain go1.25.13",
		"",
	}, "\n"))
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")

	if _, err := ApplyCandidate(repo, goModPath, docPath, GoVersion{Major: 1, Minor: 25, Patch: 14}); err != nil {
		t.Fatalf("ApplyCandidate returned error: %v", err)
	}

	goMod := string(mustReadFile(t, goModPath))
	if !strings.Contains(goMod, "go 1.24.0") {
		t.Fatalf("go directive changed unexpectedly: %q", goMod)
	}
	if !strings.Contains(goMod, "toolchain go1.25.14") {
		t.Fatalf("toolchain directive was not updated: %q", goMod)
	}
}

func TestApplyCandidateRejectsAmbiguousDocumentationReplacement(t *testing.T) {
	t.Parallel()

	repo := newMutationTestRepo(t)
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")
	doc := string(mustReadFile(t, docPath))
	duplicate := doc + "\n" + doc
	if err := os.WriteFile(docPath, []byte(duplicate), 0o644); err != nil {
		t.Fatalf("write duplicated doc: %v", err)
	}

	_, err := ApplyCandidate(
		repo,
		filepath.Join(repo, "go.mod"),
		docPath,
		GoVersion{Major: 1, Minor: 25, Patch: 14},
	)
	if err == nil {
		t.Fatal("ApplyCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "documentation replacement match count") {
		t.Fatalf("error = %q, want documentation replacement match count", err.Error())
	}
}

func TestVerifyCandidateAcceptsExactExpectedPatch(t *testing.T) {
	t.Parallel()

	repo := newCommittedMutationRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")

	if _, err := ApplyCandidate(repo, goModPath, docPath, GoVersion{Major: 1, Minor: 25, Patch: 14}); err != nil {
		t.Fatalf("ApplyCandidate returned error: %v", err)
	}

	scanPath := filepath.Join(t.TempDir(), "scan.json")
	writeScanFile(t, scanPath, cleanScan())

	got, err := VerifyCandidate(repo, goModPath, docPath, scanPath, GoVersion{Major: 1, Minor: 25, Patch: 14})
	if err != nil {
		t.Fatalf("VerifyCandidate returned error: %v", err)
	}

	if !got.Verified {
		t.Fatalf("Verified = false, want true")
	}
	if got.TargetVersion != "go1.25.14" {
		t.Fatalf("TargetVersion = %q, want %q", got.TargetVersion, "go1.25.14")
	}
	if joinPaths(got.ChangedFiles) != "docs/maintainers/development.md\ngo.mod" {
		t.Fatalf("ChangedFiles = %#v", got.ChangedFiles)
	}
}

func TestVerifyCandidateRejectsUnexpectedRepositoryState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		mutate func(t *testing.T, repo string)
		want   string
	}{
		{
			name: "unexpected extra modified file",
			mutate: func(t *testing.T, repo string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("changed\n"), 0o644); err != nil {
					t.Fatalf("write README.md: %v", err)
				}
			},
			want: "unexpected modified path",
		},
		{
			name: "unexpected untracked file",
			mutate: func(t *testing.T, repo string) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(repo, "extra.txt"), []byte("extra\n"), 0o644); err != nil {
					t.Fatalf("write extra.txt: %v", err)
				}
			},
			want: "unexpected repository change",
		},
		{
			name: "changed go directive",
			mutate: func(t *testing.T, repo string) {
				t.Helper()
				goModPath := filepath.Join(repo, "go.mod")
				contents := strings.ReplaceAll(string(mustReadFile(t, goModPath)), "go 1.25.0", "go 1.25.1")
				if err := os.WriteFile(goModPath, []byte(contents), 0o644); err != nil {
					t.Fatalf("rewrite go.mod: %v", err)
				}
			},
			want: "go directive changed",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			repo := newCommittedMutationRepo(t)
			goModPath := filepath.Join(repo, "go.mod")
			docPath := filepath.Join(repo, "docs", "maintainers", "development.md")

			if _, err := ApplyCandidate(repo, goModPath, docPath, GoVersion{Major: 1, Minor: 25, Patch: 14}); err != nil {
				t.Fatalf("ApplyCandidate returned error: %v", err)
			}
			tc.mutate(t, repo)

			scanPath := filepath.Join(t.TempDir(), "scan.json")
			writeScanFile(t, scanPath, cleanScan())

			_, err := VerifyCandidate(repo, goModPath, docPath, scanPath, GoVersion{Major: 1, Minor: 25, Patch: 14})
			if err == nil {
				t.Fatal("VerifyCandidate returned nil error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want substring %q", err.Error(), tc.want)
			}
		})
	}
}

func TestVerifyCandidateRejectsDirtyPostUpdateScan(t *testing.T) {
	t.Parallel()

	repo := newCommittedMutationRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")

	if _, err := ApplyCandidate(repo, goModPath, docPath, GoVersion{Major: 1, Minor: 25, Patch: 14}); err != nil {
		t.Fatalf("ApplyCandidate returned error: %v", err)
	}

	scanPath := filepath.Join(t.TempDir(), "scan.json")
	writeScanFile(t, scanPath, dirtyScan())

	_, err := VerifyCandidate(repo, goModPath, docPath, scanPath, GoVersion{Major: 1, Minor: 25, Patch: 14})
	if err == nil {
		t.Fatal("VerifyCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "post-update scan still reports findings") {
		t.Fatalf("error = %q, want dirty post-update scan message", err.Error())
	}
}

func TestVerifyCandidateRejectsUnexpectedTargetVersion(t *testing.T) {
	t.Parallel()

	repo := newCommittedMutationRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")

	if _, err := ApplyCandidate(repo, goModPath, docPath, GoVersion{Major: 1, Minor: 25, Patch: 14}); err != nil {
		t.Fatalf("ApplyCandidate returned error: %v", err)
	}

	scanPath := filepath.Join(t.TempDir(), "scan.json")
	writeScanFile(t, scanPath, cleanScan())

	_, err := VerifyCandidate(repo, goModPath, docPath, scanPath, GoVersion{Major: 1, Minor: 25, Patch: 15})
	if err == nil {
		t.Fatal("VerifyCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "candidate toolchain does not match expected target") {
		t.Fatalf("error = %q, want target mismatch message", err.Error())
	}
}

func TestApplyCandidateRejectsPathsOutsideRepositoryAllowlist(t *testing.T) {
	t.Parallel()

	repo := newMutationTestRepo(t)
	outsideDoc := filepath.Join(repo, "docs", "maintainers", "other.md")
	mustWriteFile(t, outsideDoc, "unexpected\n")

	_, err := ApplyCandidate(
		repo,
		filepath.Join(repo, "go.mod"),
		outsideDoc,
		GoVersion{Major: 1, Minor: 25, Patch: 14},
	)
	if err == nil {
		t.Fatal("ApplyCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "outside the apply allowlist") {
		t.Fatalf("error = %q, want allowlist rejection", err.Error())
	}
}

func TestApplyCandidateRejectsNonCanonicalRepositoryRoot(t *testing.T) {
	t.Parallel()

	repo := newMutationTestRepo(t)
	repoWithTraversal := repo + string(os.PathSeparator) + "docs" + string(os.PathSeparator) + ".."

	_, err := ApplyCandidate(
		repoWithTraversal,
		filepath.Join(repo, "go.mod"),
		filepath.Join(repo, "docs", "maintainers", "development.md"),
		GoVersion{Major: 1, Minor: 25, Patch: 14},
	)
	if err == nil {
		t.Fatal("ApplyCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "repository root path must be canonical") {
		t.Fatalf("error = %q, want canonical repo-root rejection", err.Error())
	}
}

func TestApplyCandidateRejectsSymlinkedAllowlistedInputWithoutMutation(t *testing.T) {
	t.Parallel()

	repo := newMutationTestRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")
	linkPath := filepath.Join(repo, "go.mod.link")

	beforeGoMod := string(mustReadFile(t, goModPath))
	beforeDoc := string(mustReadFile(t, docPath))

	if err := os.Symlink(goModPath, linkPath); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := ApplyCandidate(
		repo,
		linkPath,
		docPath,
		GoVersion{Major: 1, Minor: 25, Patch: 14},
	)
	if err == nil {
		t.Fatal("ApplyCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "refusing symlink path") {
		t.Fatalf("error = %q, want symlink rejection", err.Error())
	}

	afterGoMod := string(mustReadFile(t, goModPath))
	afterDoc := string(mustReadFile(t, docPath))
	if afterGoMod != beforeGoMod {
		t.Fatalf("go.mod mutated after symlink rejection:\nBEFORE:\n%s\nAFTER:\n%s", beforeGoMod, afterGoMod)
	}
	if afterDoc != beforeDoc {
		t.Fatalf("development doc mutated after symlink rejection:\nBEFORE:\n%s\nAFTER:\n%s", beforeDoc, afterDoc)
	}
}

func TestApplyCandidateRejectsSymlinkedParentWithoutMutation(t *testing.T) {
	t.Parallel()

	repo := newMutationTestRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")
	linkParent := filepath.Join(repo, "docs", "maintainers-link")
	symlinkedDocPath := filepath.Join(linkParent, "development.md")

	beforeGoMod := string(mustReadFile(t, goModPath))
	beforeDoc := string(mustReadFile(t, docPath))

	if err := os.Symlink(filepath.Join(repo, "docs", "maintainers"), linkParent); err != nil {
		t.Fatalf("create symlinked parent: %v", err)
	}

	_, err := ApplyCandidate(
		repo,
		goModPath,
		symlinkedDocPath,
		GoVersion{Major: 1, Minor: 25, Patch: 14},
	)
	if err == nil {
		t.Fatal("ApplyCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "refusing symlink path") {
		t.Fatalf("error = %q, want symlink rejection", err.Error())
	}

	afterGoMod := string(mustReadFile(t, goModPath))
	afterDoc := string(mustReadFile(t, docPath))
	if afterGoMod != beforeGoMod {
		t.Fatalf("go.mod mutated after symlinked-parent rejection:\nBEFORE:\n%s\nAFTER:\n%s", beforeGoMod, afterGoMod)
	}
	if afterDoc != beforeDoc {
		t.Fatalf("development doc mutated through symlinked parent:\nBEFORE:\n%s\nAFTER:\n%s", beforeDoc, afterDoc)
	}
}

func TestApplyCandidateRejectsSymlinkedParentTraversalWithoutMutation(t *testing.T) {
	t.Parallel()

	repo := newMutationTestRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")
	linkParent := filepath.Join(repo, "docs", "maintainers-link")
	traversalDocPath := linkParent + string(os.PathSeparator) + ".." + string(os.PathSeparator) + "maintainers" + string(os.PathSeparator) + "development.md"

	beforeGoMod := string(mustReadFile(t, goModPath))
	beforeDoc := string(mustReadFile(t, docPath))

	if err := os.Symlink(filepath.Join(repo, "docs", "maintainers"), linkParent); err != nil {
		t.Fatalf("create symlinked parent: %v", err)
	}

	_, err := ApplyCandidate(
		repo,
		goModPath,
		traversalDocPath,
		GoVersion{Major: 1, Minor: 25, Patch: 14},
	)
	if err == nil {
		t.Fatal("ApplyCandidate returned nil error")
	}
	if !strings.Contains(err.Error(), "parent traversal") {
		t.Fatalf("error = %q, want parent traversal rejection", err.Error())
	}

	afterGoMod := string(mustReadFile(t, goModPath))
	afterDoc := string(mustReadFile(t, docPath))
	if afterGoMod != beforeGoMod {
		t.Fatalf("go.mod mutated after symlink traversal rejection:\nBEFORE:\n%s\nAFTER:\n%s", beforeGoMod, afterGoMod)
	}
	if afterDoc != beforeDoc {
		t.Fatalf("development doc mutated through symlink traversal:\nBEFORE:\n%s\nAFTER:\n%s", beforeDoc, afterDoc)
	}
}

func newMutationTestRepo(t *testing.T) string {
	t.Helper()
	return newMutationTestRepoWithGoMod(t, strings.Join([]string{
		"module example.com/test",
		"",
		"go 1.25.0",
		"",
		"toolchain go1.25.13",
		"",
		"require github.com/spf13/cobra v1.10.2",
		"",
	}, "\n"))
}

func newMutationTestRepoWithGoMod(t *testing.T, goMod string) string {
	t.Helper()

	repo := t.TempDir()
	mustWriteFile(t, filepath.Join(repo, "go.mod"), goMod)
	mustWriteFile(t, filepath.Join(repo, "README.md"), "test repo\n")
	mustWriteFile(t, filepath.Join(repo, "docs", "maintainers", "development.md"), strings.Join([]string{
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
	return repo
}

func newCommittedMutationRepo(t *testing.T) string {
	t.Helper()

	repo := newMutationTestRepo(t)
	runGit(t, repo, "init")
	runGit(t, repo, "config", "user.email", "codex@example.com")
	runGit(t, repo, "config", "user.name", "Codex")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "initial")
	return repo
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = filteredGitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeScanFile(t *testing.T, path string, contents string) {
	t.Helper()
	mustWriteFile(t, path, contents)
}

func cleanScan() string {
	return strings.Join([]string{
		`{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scanner_version":"v1.5.0","go_version":"go1.25.14"}}`,
		`{"progress":{"message":"scanning"}}`,
		"",
	}, "\n")
}

func dirtyScan() string {
	return strings.Join([]string{
		`{"config":{"protocol_version":"v1.0.0","scanner_name":"govulncheck","scanner_version":"v1.5.0","go_version":"go1.25.14"}}`,
		`{"finding":{"osv":"GO-2026-0001","fixed_version":"go1.25.15","trace":[{"module":"stdlib"}]}}`,
		"",
	}, "\n")
}

func mustWriteFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func joinPaths(paths []string) string {
	return strings.Join(paths, "\n")
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
