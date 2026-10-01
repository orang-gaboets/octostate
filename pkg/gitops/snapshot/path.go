package snapshot

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// checkedActualPath rejects symlinks, Windows reparse points, and
// non-directory parents in the complete absolute path to the snapshot. Writes
// create missing parent directories one component at a time so each new entry
// is checked before proceeding.
func checkedActualPath(path string, createParents bool) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve snapshot path %q: %w", path, err)
	}
	absolutePath = filepath.Clean(absolutePath)

	root := filepath.VolumeName(absolutePath) + string(filepath.Separator)
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return "", fmt.Errorf("inspect snapshot path root %q: %w", root, err)
	}
	if isSymlinkOrReparsePoint(rootInfo) || !rootInfo.IsDir() {
		return "", fmt.Errorf("unsafe snapshot path component %q: root must be a directory", root)
	}

	relativePath, err := filepath.Rel(root, absolutePath)
	if err != nil {
		return "", fmt.Errorf("split snapshot path %q: %w", absolutePath, err)
	}
	components := strings.Split(relativePath, string(filepath.Separator))
	current := root
	for index, component := range components {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		isDestination := index == len(components)-1

		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			if !createParents || isDestination {
				// Let the eventual open preserve the ordinary missing-file error.
				return absolutePath, nil
			}
			if err := os.Mkdir(current, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return "", fmt.Errorf("create snapshot path component %q: %w", current, err)
			}
			info, err = os.Lstat(current)
		}
		if err != nil {
			return "", fmt.Errorf("inspect snapshot path component %q: %w", current, err)
		}
		if isSymlinkOrReparsePoint(info) {
			return "", fmt.Errorf("unsafe snapshot path component %q: symbolic links and reparse points are not allowed", current)
		}
		if isDestination {
			if !info.Mode().IsRegular() {
				return "", fmt.Errorf("unsafe snapshot path component %q: destination must be a regular file", current)
			}
			continue
		}
		if !info.IsDir() {
			return "", fmt.Errorf("unsafe snapshot path component %q: parent must be a directory", current)
		}
	}

	return absolutePath, nil
}
