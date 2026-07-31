//go:build windows

package filereplace

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
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
	matches, err := filepath.Glob(filepath.Join(dir, ".organization.yaml.backup-*"))
	if err != nil {
		t.Fatalf("glob backup files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("unexpected backup files left behind: %v", matches)
	}
}

func TestReplacePreservesTempOnWindowsRecoveryFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "blocked"), 0o755); err != nil {
		t.Fatalf("seed blocking dir: %v", err)
	}
	tempFile, err := os.CreateTemp(dir, ".organization.yaml-*")
	if err != nil {
		t.Fatalf("seed temp file: %v", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}

	oldCall := replaceFileWCall
	t.Cleanup(func() { replaceFileWCall = oldCall })

	replaceFileWCall = func(_, _, _ uintptr) (uintptr, error) {
		return 0, syscall.Errno(1177)
	}

	keepTemp, err := replaceFilePlatform(tempPath, filepath.Join(dir, "blocked"))
	if err == nil || !strings.Contains(err.Error(), "replace existing file") {
		t.Fatalf("expected recovery failure, got %v", err)
	}
	if !keepTemp {
		t.Fatal("expected backend to request temp preservation")
	}
	if _, statErr := os.Stat(tempPath); statErr != nil {
		t.Fatalf("expected temp file to remain, got %v", statErr)
	}
}

func TestSyncParentDirPlatformOnWindows(t *testing.T) {
	dir := t.TempDir()
	if err := syncParentDirPlatform(dir); err != nil {
		t.Fatalf("sync parent directory: %v", err)
	}
}
