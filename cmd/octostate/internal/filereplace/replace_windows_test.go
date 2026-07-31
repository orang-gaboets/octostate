//go:build windows

package filereplace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceUpdatesContentsOnWindows(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "organization.yaml")

	if err := os.WriteFile(path, []byte("organization: old\n"), 0o600); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	if err := Replace(path, []byte("organization: new\n")); err != nil {
		t.Fatalf("replace config file: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced file: %v", err)
	}
	if string(got) != "organization: new\n" {
		t.Fatalf("unexpected file contents:\n%s", got)
	}
}

func TestSyncParentDirPlatformOnWindows(t *testing.T) {
	dir := t.TempDir()
	if err := syncParentDirPlatform(dir); err != nil {
		t.Fatalf("sync parent directory: %v", err)
	}
}
