package toolchainremediation

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestImplicitToolchainApplyAndVerify(t *testing.T) {
	t.Parallel()

	for _, goLine := range []string{"go 1.25.13\n", "go 1.25.13 // security baseline\n", "go 1.25.13"} {
		t.Run(goLine, func(t *testing.T) {
			t.Parallel()
			original := "module example.com/test\n\n" + goLine
			repo := newMutationTestRepoWithGoMod(t, original)
			goModPath := filepath.Join(repo, "go.mod")
			docPath := filepath.Join(repo, "docs", "maintainers", "development.md")
			mustWriteFile(t, docPath, "# Development\n\n"+
				"   The module's `go 1.25.13` directive implies the `go1.25.13` toolchain.\n"+
				"   Go commands automatically select it when toolchain switching is enabled.\n")
			runGit(t, repo, "init")
			runGit(t, repo, "config", "user.email", "test@example.invalid")
			runGit(t, repo, "config", "user.name", "Test")
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "initial")

			directives, err := ReadDirectives(goModPath)
			if err != nil {
				t.Fatalf("ReadDirectives: %v", err)
			}
			if directives.GoDirective != "1.25.13" || directives.ToolchainVersion.String() != "go1.25.13" {
				t.Fatalf("unexpected implicit directives: %#v", directives)
			}
			target := GoVersion{Major: 1, Minor: 25, Patch: 14}
			if _, err := ApplyCandidate(repo, goModPath, docPath, target); err != nil {
				t.Fatalf("ApplyCandidate: %v", err)
			}
			want := strings.TrimSuffix(original, "\n") + "\n\ntoolchain go1.25.14\n"
			if got := string(mustReadFile(t, goModPath)); got != want {
				t.Fatalf("go.mod = %q, want %q", got, want)
			}
			wantDoc := "# Development\n\n" +
				"   The module includes a `toolchain go1.25.14` directive, so Go commands will\n" +
				"   automatically prefer the patched 1.25.14 toolchain when toolchain switching\n" +
				"   is enabled.\n"
			if got := string(mustReadFile(t, docPath)); got != wantDoc {
				t.Fatalf("development doc = %q, want %q", got, wantDoc)
			}
			scanPath := filepath.Join(t.TempDir(), "scan.json")
			writeScanFile(t, scanPath, cleanScan())
			if got, err := VerifyCandidate(repo, goModPath, docPath, scanPath, target); err != nil || !got.Verified {
				t.Fatalf("VerifyCandidate = %#v, %v", got, err)
			}

			mustWriteFile(t, goModPath, want+"\nrequire example.com/unexpected v1.0.0\n")
			if _, err := VerifyCandidate(repo, goModPath, docPath, scanPath, target); err == nil || !strings.Contains(err.Error(), "unexpected go.mod diff") {
				t.Fatalf("expected unexpected go.mod diff rejection, got %v", err)
			}
		})
	}
}

func TestImplicitToolchainRejectsUnsafeTargetsWithoutMutation(t *testing.T) {
	t.Parallel()

	for _, target := range []GoVersion{
		{Major: 1, Minor: 25, Patch: 12},
		{Major: 1, Minor: 25, Patch: 13},
		{Major: 1, Minor: 26, Patch: 1},
	} {
		t.Run(target.String(), func(t *testing.T) {
			t.Parallel()
			repo := newMutationTestRepoWithGoMod(t, "module example.com/test\n\ngo 1.25.13\n")
			goModPath := filepath.Join(repo, "go.mod")
			docPath := filepath.Join(repo, "docs", "maintainers", "development.md")
			beforeMod, beforeDoc := mustReadFile(t, goModPath), mustReadFile(t, docPath)
			_, err := ApplyCandidate(repo, goModPath, docPath, target)
			wantErr := "is not newer"
			if target.Minor != 25 {
				wantErr = "crosses the current Go minor line"
			}
			if err == nil || !strings.Contains(err.Error(), wantErr) {
				t.Fatalf("expected %q rejection, got %v", wantErr, err)
			}
			if !bytes.Equal(beforeMod, mustReadFile(t, goModPath)) || !bytes.Equal(beforeDoc, mustReadFile(t, docPath)) {
				t.Fatal("rejected target mutated files")
			}
		})
	}
}

func TestApplyCandidateRejectsMixedDocumentationSnippets(t *testing.T) {
	t.Parallel()
	repo := newMutationTestRepo(t)
	goModPath := filepath.Join(repo, "go.mod")
	docPath := filepath.Join(repo, "docs", "maintainers", "development.md")
	beforeMod := mustReadFile(t, goModPath)
	doc := string(mustReadFile(t, docPath)) +
		"\n   The module's `go 1.25.13` directive implies the `go1.25.13` toolchain.\n" +
		"   Go commands automatically select it when toolchain switching is enabled.\n"
	mustWriteFile(t, docPath, doc)
	_, err := ApplyCandidate(repo, goModPath, docPath, GoVersion{Major: 1, Minor: 25, Patch: 14})
	if err == nil || !strings.Contains(err.Error(), "documentation replacement match count") {
		t.Fatalf("expected ambiguous documentation rejection, got %v", err)
	}
	if !bytes.Equal(beforeMod, mustReadFile(t, goModPath)) || string(mustReadFile(t, docPath)) != doc {
		t.Fatal("ambiguous documentation mutated files")
	}
}
