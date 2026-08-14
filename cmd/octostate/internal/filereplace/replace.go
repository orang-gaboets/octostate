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

// StatExistingRegularFile inspects path without following symlinks and
// requires it to be an existing regular file.
func StatExistingRegularFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("stat existing file %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("stat existing file %s: target is a symbolic link", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("stat existing file %s: target is not a regular file", path)
	}
	return info, nil
}

// Replace writes contents to a same-directory temporary file and atomically
// replaces the destination file.
func Replace(path string, contents []byte) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	info, err := StatExistingRegularFile(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return fmt.Errorf("create temporary file in %s: %w", dir, err)
	}
	tempPath := tempFile.Name()
	replaced := false
	keepTempOnFailure := false
	defer func() {
		if !replaced && !keepTempOnFailure {
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

	if keepTemp, err := replaceFile(tempPath, path); err != nil {
		keepTempOnFailure = keepTemp
		return fmt.Errorf("replace existing file %s: %w", path, err)
	}
	replaced = true

	_ = syncParentDir(dir) // best effort after commit; replacement already succeeded

	return nil
}
