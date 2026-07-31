package filereplace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReplaceCleansUpTempFileWhenBackendFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "organization.yaml")

	before := []byte("organization: orang-gaboets\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	oldReplaceFile := replaceFile
	oldSyncParentDir := syncParentDir
	t.Cleanup(func() {
		replaceFile = oldReplaceFile
		syncParentDir = oldSyncParentDir
	})

	var tempPath string
	replaceFile = func(gotTempPath, gotTargetPath string) (bool, error) {
		tempPath = gotTempPath
		if gotTargetPath != path {
			t.Fatalf("unexpected target path %q", gotTargetPath)
		}
		return false, errors.New("replace failed")
	}
	syncParentDir = func(string) error {
		t.Fatal("syncParentDir should not run after replace failure")
		return nil
	}

	err := Replace(path, []byte("organization: octostate\n"))
	if err == nil || !strings.Contains(err.Error(), "replace failed") {
		t.Fatalf("expected backend failure, got %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read original file: %v", err)
	}
	if string(got) != string(before) {
		t.Fatalf("original file changed:\n%s\nwant:\n%s", got, before)
	}
	if tempPath == "" {
		t.Fatal("expected temp path to be captured")
	}
	if _, err := os.Stat(tempPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temp file cleanup, got %v", err)
	}
}

func TestReplacePreservesTempFileWhenBackendRequestsIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "organization.yaml")

	if err := os.WriteFile(path, []byte("organization: old\n"), 0o600); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	oldReplaceFile := replaceFile
	oldSyncParentDir := syncParentDir
	t.Cleanup(func() {
		replaceFile = oldReplaceFile
		syncParentDir = oldSyncParentDir
	})

	var tempPath string
	replaceFile = func(gotTempPath, gotTargetPath string) (bool, error) {
		tempPath = gotTempPath
		if gotTargetPath != path {
			t.Fatalf("unexpected target path %q", gotTargetPath)
		}
		return true, errors.New("replace uncertain")
	}
	syncParentDir = func(string) error {
		t.Fatal("syncParentDir should not run after replace failure")
		return nil
	}

	err := Replace(path, []byte("organization: new\n"))
	if err == nil || !strings.Contains(err.Error(), "replace uncertain") {
		t.Fatalf("expected backend failure, got %v", err)
	}
	if tempPath == "" {
		t.Fatal("expected temp path to be captured")
	}
	if _, err := os.Stat(tempPath); err != nil {
		t.Fatalf("expected temp file to remain, got %v", err)
	}
}

func TestReplaceIgnoresParentDirSyncFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "organization.yaml")

	if err := os.WriteFile(path, []byte("organization: old\n"), 0o600); err != nil {
		t.Fatalf("seed config file: %v", err)
	}

	oldReplaceFile := replaceFile
	oldSyncParentDir := syncParentDir
	t.Cleanup(func() {
		replaceFile = oldReplaceFile
		syncParentDir = oldSyncParentDir
	})

	replaceFile = func(gotTempPath, gotTargetPath string) (bool, error) {
		if gotTempPath == "" || gotTargetPath != path {
			t.Fatalf("unexpected replace args temp=%q target=%q", gotTempPath, gotTargetPath)
		}
		if err := os.WriteFile(gotTargetPath, []byte("organization: new\n"), 0o600); err != nil {
			return false, err
		}
		if err := os.Remove(gotTempPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("remove temp path: %v", err)
		}
		return false, nil
	}
	syncParentDir = func(string) error {
		return errors.New("sync failed")
	}

	if err := Replace(path, []byte("organization: new\n")); err != nil {
		t.Fatalf("expected sync failure to be ignored, got %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read replaced file: %v", err)
	}
	if string(got) != "organization: new\n" {
		t.Fatalf("unexpected file contents:\n%s", got)
	}
}

func TestReplaceRejectsSymlinkTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.yaml")
	linkPath := filepath.Join(dir, "organization.yaml")
	before := []byte("organization: orang-gaboets\n")

	if err := os.WriteFile(targetPath, before, 0o600); err != nil {
		t.Fatalf("seed target file: %v", err)
	}
	mustCreateSymlink(t, targetPath, linkPath)

	err := Replace(linkPath, []byte("organization: octostate\n"))
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}

	if got, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("stat symlink: %v", err)
	} else if got.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink to remain, got mode %v", got.Mode())
	}
	got, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target file: %v", err)
	}
	if string(got) != string(before) {
		t.Fatalf("target file changed:\n%s\nwant:\n%s", got, before)
	}
	assertNoTempFiles(t, dir)
}

func TestReplaceRejectsDirectoryTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "organization.yaml")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("seed directory: %v", err)
	}

	err := Replace(path, []byte("organization: octostate\n"))
	if err == nil || !strings.Contains(err.Error(), "target is not a regular file") {
		t.Fatalf("expected directory rejection, got %v", err)
	}
	assertNoTempFiles(t, dir)
}

func mustCreateSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.Errno(1314)) || os.IsPermission(err) {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".organization.yaml-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("unexpected temp files left behind: %v", matches)
	}
}
