package filereplace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
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
//
// The destination must already exist as a regular file. That strictness is
// deliberate: callers such as sync-from-live rely on it to refuse writing
// through a symbolic link. Use WriteFile when the destination may legitimately
// be absent.
func Replace(path string, contents []byte) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	info, err := StatExistingRegularFile(path)
	if err != nil {
		return err
	}

	return writeAtomic(path, contents, info.Mode().Perm(), true)
}

// WriteFile writes contents to path, creating the file when it does not exist
// and replacing it when it does.
//
// Replacing an existing file is atomic, through the same staged path Replace
// uses. Creating a new one commits with os.Rename, which Go does not guarantee
// to be atomic on non-Unix platforms; there is no existing destination to
// protect from a partial write in that case.
//
// perm applies only when creating. An existing file keeps its own mode, so
// writing a generated file never silently widens or narrows its permissions.
//
// An existing destination must be a regular file: a symbolic link or any other
// non-regular target is refused rather than followed, matching Replace.
func WriteFile(path string, contents []byte, perm os.FileMode) error {
	if path == "" {
		return fmt.Errorf("path is required")
	}

	destinationExists := false
	switch _, err := os.Lstat(path); {
	case err == nil:
		existing, statErr := StatExistingRegularFile(path)
		if statErr != nil {
			return statErr
		}
		perm = existing.Mode().Perm()
		destinationExists = true
	case errors.Is(err, fs.ErrNotExist):
		// Creating the destination; perm applies as given.
	default:
		return fmt.Errorf("stat destination %s: %w", path, err)
	}

	return writeAtomic(path, contents, perm, destinationExists)
}

// writeAtomic stages contents in a same-directory temporary file and commits it
// over path, so a reader never observes a partially written file.
func writeAtomic(path string, contents []byte, perm os.FileMode, destinationExists bool) error {
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

	if err := tempFile.Chmod(perm); err != nil {
		return fmt.Errorf("set temporary file mode %s: %w", tempPath, err)
	}
	if err := tempFile.Sync(); err != nil {
		return fmt.Errorf("sync temporary file %s: %w", tempPath, err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary file %s: %w", tempPath, err)
	}

	if destinationExists {
		if keepTemp, err := replaceFile(tempPath, path); err != nil {
			keepTempOnFailure = keepTemp
			return fmt.Errorf("replace existing file %s: %w", path, err)
		}
	} else {
		// Windows ReplaceFileW requires an existing destination, so it cannot
		// commit a newly created file. Use os.Rename for the initial commit;
		// Go does not guarantee Rename is atomic on non-Unix platforms.
		if err := os.Rename(tempPath, path); err != nil {
			return fmt.Errorf("create file %s: %w", path, err)
		}
	}
	replaced = true

	_ = syncParentDir(dir) // best effort after commit; replacement already succeeded

	return nil
}
