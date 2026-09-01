//go:build !windows

package contributors

import (
	"os"
	"testing"
)

// Permission bits are only meaningful on Unix; Windows synthesizes them from
// the read-only attribute.
func TestUpdatePreservesTheExistingFileMode(t *testing.T) {
	t.Parallel()

	path := writeReadme(t, "# Title\n\n"+startMarker+"\n"+endMarker+"\n")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Update(path, []Contributor{{Login: "alice"}}, Config{}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want the original 0600 preserved", info.Mode().Perm())
	}
}
