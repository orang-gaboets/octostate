package filereplace

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	replaceFile = func(gotTempPath, gotTargetPath string) error {
		tempPath = gotTempPath
		if gotTargetPath != path {
			t.Fatalf("unexpected target path %q", gotTargetPath)
		}
		return errors.New("replace failed")
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
