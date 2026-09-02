//go:build !windows

package snapshot

import (
	"os"
	"testing"
)

// The snapshot carries organization member and invitation data, and the
// previous os.CreateTemp path created it 0600. Permission bits are only
// meaningful on Unix, so this assertion is build-tagged.
func TestWriteActualCreatesTheSnapshotPrivate(t *testing.T) {
	t.Parallel()

	path, err := WriteActual(t.TempDir(), sampleSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("new snapshot mode = %v, want 0600", info.Mode().Perm())
	}
}
