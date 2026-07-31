// Package filereplace atomically replaces an existing file with new contents.
package filereplace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	replaceFile   = replaceFilePlatform
	syncParentDir = syncParentDirPlatform
)

// Replace writes contents to a same-directory temporary file and atomically
// replaces the destination file.
func Replace(path string, contents []byte) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat existing file %s: %w", path, err)
	}
	if info.IsDir() {
		return fmt.Errorf("stat existing file %s: target is a directory", path)
	}

	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create temporary file in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	replaced := false
	defer func() {
		if !replaced {
			_ = tempFile.Close()    //nolint:errcheck // best-effort cleanup for temp files
			_ = os.Remove(tempPath) //nolint:errcheck // best-effort cleanup for temp files
		}
	}()

	if n, err := tempFile.Write(contents); err != nil {
		return fmt.Errorf("write temporary file %s: %w", tempPath, err)
	} else if n != len(contents) {
		return fmt.Errorf("write temporary file %s: %w", tempPath, io.ErrShortWrite)
	}

	if err := tempFile.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("set temporary file mode %s: %w", tempPath, err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("sync temporary file %s: %w", tempPath, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary file %s: %w", tempPath, err)
	}

	if err := replaceFile(tempPath, path); err != nil {
		return fmt.Errorf("replace existing file %s: %w", path, err)
	}
	replaced = true

	if err := syncParentDir(dir); err != nil {
		return fmt.Errorf("sync parent directory %s: %w", dir, err)
	}

	return nil
}
