//go:build windows

package filereplace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"
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

func TestReplaceRecoversFromWindows1177WhenBackupExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "organization.yaml")
	before := []byte("organization: old\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	oldCall := replaceFileWCall
	oldRename := renameFile
	t.Cleanup(func() { replaceFileWCall = oldCall })
	t.Cleanup(func() { renameFile = oldRename })

	replaceFileWCall = func(targetPathPtr, _, backupPathPtr uintptr) (uintptr, error) {
		targetPath := utf16PtrString(targetPathPtr)
		backupPath := utf16PtrString(backupPathPtr)
		if targetPath != path {
			t.Fatalf("unexpected target path %q", targetPath)
		}
		if err := os.WriteFile(backupPath, before, 0o600); err != nil {
			t.Fatalf("seed backup file: %v", err)
		}
		if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove original target: %v", err)
		}
		return 0, syscall.Errno(1177)
	}

	if err := Replace(path, []byte("organization: new\n")); err == nil || !strings.Contains(err.Error(), "replace existing file") {
		t.Fatalf("expected recovery warning, got %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored file: %v", err)
	}
	if string(got) != string(before) {
		t.Fatalf("unexpected restored file:\n%s", got)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".organization.yaml.backup-*")); err != nil {
		t.Fatalf("glob backup files: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("unexpected backup files left behind: %v", matches)
	}
	if matches, err := filepath.Glob(filepath.Join(dir, ".organization.yaml-*")); err != nil {
		t.Fatalf("glob temp files: %v", err)
	} else if len(matches) != 0 {
		t.Fatalf("unexpected temp files left behind: %v", matches)
	}
}

func TestReplacePreservesBackupAndTempOnWindowsRecoveryFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "organization.yaml")
	before := []byte("organization: old\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	oldCall := replaceFileWCall
	oldRename := renameFile
	t.Cleanup(func() { replaceFileWCall = oldCall })
	t.Cleanup(func() { renameFile = oldRename })

	replaceFileWCall = func(targetPathPtr, _, backupPathPtr uintptr) (uintptr, error) {
		targetPath := utf16PtrString(targetPathPtr)
		backupPath := utf16PtrString(backupPathPtr)
		if targetPath != path {
			t.Fatalf("unexpected target path %q", targetPath)
		}
		if err := os.WriteFile(backupPath, before, 0o600); err != nil {
			t.Fatalf("seed backup file: %v", err)
		}
		if err := os.Remove(targetPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove original target: %v", err)
		}
		return 0, syscall.Errno(1177)
	}
	renameFile = func(string, string) error {
		return errors.New("restore failed")
	}

	err := Replace(path, []byte("organization: new\n"))
	if err == nil || !strings.Contains(err.Error(), "restore original from") {
		t.Fatalf("expected recovery failure, got %v", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected original target to stay missing, got %v", err)
	}

	if matches, err := filepath.Glob(filepath.Join(dir, ".organization.yaml.backup-*")); err != nil {
		t.Fatalf("glob backup files: %v", err)
	} else if len(matches) != 1 {
		t.Fatalf("expected one backup file, got %v", matches)
	} else if got, err := os.ReadFile(matches[0]); err != nil {
		t.Fatalf("read backup file: %v", err)
	} else if string(got) != string(before) {
		t.Fatalf("unexpected backup file:\n%s", got)
	}

	if matches, err := filepath.Glob(filepath.Join(dir, ".organization.yaml-*")); err != nil {
		t.Fatalf("glob temp files: %v", err)
	} else if len(matches) != 1 {
		t.Fatalf("expected one temp file, got %v", matches)
	} else if got, err := os.ReadFile(matches[0]); err != nil {
		t.Fatalf("read temp file: %v", err)
	} else if string(got) != "organization: new\n" {
		t.Fatalf("unexpected temp file:\n%s", got)
	}
}

func TestSyncParentDirPlatformOnWindows(t *testing.T) {
	dir := t.TempDir()
	if err := syncParentDirPlatform(dir); err != nil {
		t.Fatalf("sync parent directory: %v", err)
	}
}

func utf16PtrString(ptr uintptr) string {
	if ptr == 0 {
		return ""
	}

	var words []uint16
	for p := ptr; ; p += 2 {
		w := *(*uint16)(unsafe.Pointer(p))
		if w == 0 {
			break
		}
		words = append(words, w)
	}
	return string(utf16.Decode(words))
}
