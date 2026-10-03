package snapshot

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// openSnapshotParent opens the snapshot's parent directory and walks to it
// through stable directory handles. Component identity checks detect a path
// entry changed between Lstat and OpenRoot; later path changes cannot redirect
// operations rooted at the returned handle.
func openSnapshotParent(path string, createParents bool) (*os.Root, string, error) {
	if runtime.GOOS == "js" || runtime.GOOS == "plan9" {
		return nil, "", fmt.Errorf("snapshot path operations are unsupported on %s", runtime.GOOS)
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return nil, "", fmt.Errorf("resolve snapshot path %q: %w", path, err)
	}
	absolutePath = filepath.Clean(absolutePath)

	rootPath := filepath.VolumeName(absolutePath) + string(filepath.Separator)
	current, err := os.OpenRoot(rootPath)
	if err != nil {
		// Do not include the OS error: it can contain a raced symlink target.
		return nil, "", fmt.Errorf("open snapshot path root %q safely", rootPath)
	}
	keepCurrent := false
	defer func() {
		if !keepCurrent {
			_ = current.Close()
		}
	}()

	rootInfo, err := current.Stat(".")
	if err != nil {
		return nil, "", fmt.Errorf("inspect opened snapshot path root %q: %w", rootPath, err)
	}
	if isSymlinkOrReparsePoint(rootInfo) || !rootInfo.IsDir() {
		return nil, "", fmt.Errorf("unsafe snapshot path component %q: root must be a directory", rootPath)
	}

	relativePath, err := filepath.Rel(rootPath, absolutePath)
	if err != nil {
		return nil, "", fmt.Errorf("split snapshot path %q: %w", absolutePath, err)
	}
	components := strings.Split(relativePath, string(filepath.Separator))
	if len(components) == 0 {
		return nil, "", fmt.Errorf("snapshot path %q has no file name", absolutePath)
	}
	name := components[len(components)-1]
	if name == "" || name == "." || name == ".." {
		return nil, "", fmt.Errorf("snapshot path %q has no file name", absolutePath)
	}

	currentPath := rootPath
	for _, component := range components[:len(components)-1] {
		if component == "" || component == "." {
			continue
		}
		currentPath = filepath.Join(currentPath, component)

		info, err := current.Lstat(component)
		if errors.Is(err, fs.ErrNotExist) && createParents {
			if err := current.Mkdir(component, 0o755); err != nil && !errors.Is(err, fs.ErrExist) {
				return nil, "", fmt.Errorf("create snapshot path component %q: %w", currentPath, err)
			}
			info, err = current.Lstat(component)
		}
		if err != nil {
			return nil, "", fmt.Errorf("inspect snapshot path component %q: %w", currentPath, err)
		}
		if isSymlinkOrReparsePoint(info) {
			return nil, "", fmt.Errorf("unsafe snapshot path component %q: symbolic links and reparse points are not allowed", currentPath)
		}
		if !info.IsDir() {
			return nil, "", fmt.Errorf("unsafe snapshot path component %q: parent must be a directory", currentPath)
		}

		next, err := current.OpenRoot(component)
		if err != nil {
			// Do not include the OS error: it can contain a raced symlink target.
			return nil, "", fmt.Errorf("open snapshot path component %q safely", currentPath)
		}
		nextInfo, err := next.Stat(".")
		if err != nil {
			_ = next.Close()
			return nil, "", fmt.Errorf("inspect opened snapshot path component %q: %w", currentPath, err)
		}
		if !os.SameFile(info, nextInfo) {
			_ = next.Close()
			return nil, "", fmt.Errorf("unsafe snapshot path component %q: changed while opening", currentPath)
		}

		previous := current
		current = next
		_ = previous.Close()
	}

	keepCurrent = true
	return current, name, nil
}
