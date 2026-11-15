package filereplace

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileInRootWithPreservesTargetAndCleansTempWhenPopulateFails(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	before := []byte("original\n")
	if err := os.WriteFile(filepath.Join(dir, "snapshot.json"), before, 0o640); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("serialize failed")
	err = WriteFileInRootWith(root, "snapshot.json", 0o600, func(writer io.Writer) error {
		if _, writeErr := writer.Write([]byte("partial")); writeErr != nil {
			return writeErr
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WriteFileInRootWith error = %v, want serializer error", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "snapshot.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(before) {
		t.Fatalf("target changed to %q, want %q", got, before)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "snapshot.json" {
		t.Fatalf("unexpected staging files: %v", entries)
	}
}

func TestWriteFileInRootWithWritesAndPreservesExistingMode(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	path := filepath.Join(dir, "snapshot.json")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileInRootWith(root, "snapshot.json", 0o600, func(writer io.Writer) error {
		_, err := io.WriteString(writer, "new\n")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("contents = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode = %o, want 640", info.Mode().Perm())
	}
}

func TestWriteFileInRootWithRejectsNonLocalNames(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	for _, name := range []string{"", "../escape", "sub/file"} {
		err := WriteFileInRootWith(root, name, 0o600, func(io.Writer) error { return nil })
		if err == nil || !strings.Contains(err.Error(), "single path component") {
			t.Fatalf("name %q error = %v", name, err)
		}
	}
}
