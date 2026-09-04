//go:build !windows

package filereplace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/orang-gaboets/octostate/internal/filereplace"
)

// Permission bits are only meaningful on Unix; Windows synthesizes them from
// the read-only attribute, so these assertions are build-tagged.
func TestWriteFileAppliesPermOnlyWhenCreating(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	created := filepath.Join(dir, "created.json")
	if err := filereplace.WriteFile(created, []byte("new"), 0o640); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(created)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("created mode = %v, want 0640", info.Mode().Perm())
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
