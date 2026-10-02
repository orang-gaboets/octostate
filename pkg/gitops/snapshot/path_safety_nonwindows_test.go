//go:build !windows

package snapshot

import (
	"os"
	"testing"
)

func replaceDirectoryWithLink(t *testing.T, path, target string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("replace directory with symlink: %v", err)
	}
}
