package filereplace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/orang-gaboets/octostate/internal/filereplace"
)

func TestWriteFileCreatesAMissingDestination(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "snapshot.json")
	if err := filereplace.WriteFile(path, []byte("created\n"), 0o644); err != nil {
		t.Fatalf("WriteFile on a missing destination must create it: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "created\n" {
		t.Fatalf("contents = %q", body)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("new file mode = %v, want 0644", info.Mode().Perm())
	}
}

func TestWriteFileReplacesAnExistingDestination(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "readme.md")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The perm argument applies to creation only; an existing file keeps its own.
	if err := filereplace.WriteFile(path, []byte("fresh"), 0o644); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "fresh" {
		t.Fatalf("contents = %q", body)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("existing file mode = %v, want the original 0600 to be preserved", info.Mode().Perm())
	}
}

func TestWriteFileRejectsASymlinkDestination(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("target"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := filereplace.WriteFile(link, []byte("through the link"), 0o644); err == nil {
		t.Fatal("writing through a symlink must be rejected, matching Replace")
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "target" {
		t.Fatalf("symlink target was modified: %q", body)
	}
}

func TestWriteFileRejectsADirectoryDestination(t *testing.T) {
	t.Parallel()

	if err := filereplace.WriteFile(t.TempDir(), []byte("x"), 0o644); err == nil {
		t.Fatal("a directory destination must be rejected")
	}
}

func TestWriteFileRequiresAPath(t *testing.T) {
	t.Parallel()

	if err := filereplace.WriteFile("", []byte("x"), 0o644); err == nil {
		t.Fatal("an empty path must be rejected")
	}
}

func TestWriteFileLeavesNoTemporaryFileBehind(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := filereplace.WriteFile(path, []byte("body"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "out.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only the destination to remain, got %v", names)
	}
}
