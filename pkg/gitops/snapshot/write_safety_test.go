package snapshot

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func sampleSnapshot() ActualSnapshot {
	return ActualSnapshot{
		PulledAt:     time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Organization: "acme",
	}
}

// The snapshot file is machine-consumed by audit diff, so switching to the
// shared writer must not alter a single byte of its encoding.
func TestWriteActualOutputIsUnchangedByteForByte(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path, err := WriteActual(dir, sampleSnapshot())
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Encoded with two-space indentation and a trailing newline, exactly as the
	// previous json.Encoder path produced.
	if len(got) == 0 || got[len(got)-1] != '\n' {
		t.Fatalf("snapshot must end with a newline, got %q", got)
	}
	if !containsSeq(got, []byte("\n  \"organization\": \"acme\",")) {
		t.Fatalf("snapshot indentation changed:\n%s", got)
	}
}

func TestWriteActualCreatesTheSnapshotOnFirstWrite(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path, err := WriteActual(dir, sampleSnapshot())
	if err != nil {
		t.Fatalf("first write must create the snapshot: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestWriteActualReplacesAnExistingSnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if _, err := WriteActual(dir, sampleSnapshot()); err != nil {
		t.Fatal(err)
	}
	second := sampleSnapshot()
	second.Organization = "beta"
	path, err := WriteActual(dir, second)
	if err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !containsSeq(body, []byte(`"organization": "beta"`)) {
		t.Fatalf("snapshot was not replaced:\n%s", body)
	}
}

// A partially written snapshot would be silently mis-parsed by audit diff, so
// the write must not leave staging files behind either.
func TestWriteActualLeavesNoTemporaryFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path, err := WriteActual(dir, sampleSnapshot())
	if err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("expected only snapshot.json, got %v", names)
	}
}

func TestWriteActualRefusesASymlinkedSnapshot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := ActualPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "elsewhere.json")
	if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if _, err := WriteActual(dir, sampleSnapshot()); err == nil {
		t.Fatal("writing the snapshot through a symlink must be refused")
	}
	body, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "{}" {
		t.Fatalf("symlink target was overwritten: %q", body)
	}
}

func containsSeq(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}
