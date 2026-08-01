//go:build !windows

package filereplace

import (
	"os"
)

func replaceFilePlatform(tempPath, targetPath string) (bool, error) {
	return false, os.Rename(tempPath, targetPath)
}

func syncParentDirPlatform(dir string) error {
	parentDir, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() {
		_ = parentDir.Close() //nolint:errcheck // best-effort cleanup for directory handles
	}()

	return parentDir.Sync()
}
