//go:build !windows

package snapshot

import "os"

func isSymlinkOrReparsePoint(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}
