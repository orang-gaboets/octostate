//go:build windows

package filereplace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

var replaceFileW = syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")
var replaceFileWCall = func(targetPathPtr, tempPathPtr, backupPathPtr uintptr) (uintptr, error) {
	r1, _, callErr := replaceFileW.Call(
		targetPathPtr,
		tempPathPtr,
		backupPathPtr,
		0,
		0,
		0,
	)
	return r1, callErr
}
var renameFile = os.Rename

func replaceFilePlatform(tempPath, targetPath string) (bool, error) {
	backupPath, err := createBackupPath(targetPath)
	if err != nil {
		return false, err
	}
	keepBackup := false
	defer func() {
		if !keepBackup {
			_ = os.Remove(backupPath) //nolint:errcheck // best-effort cleanup for backup files
		}
	}()

	tempPathPtr, err := syscall.UTF16PtrFromString(tempPath)
	if err != nil {
		return false, fmt.Errorf("convert temporary file path: %w", err)
	}
	targetPathPtr, err := syscall.UTF16PtrFromString(targetPath)
	if err != nil {
		return false, fmt.Errorf("convert destination file path: %w", err)
	}
	backupPathPtr, err := syscall.UTF16PtrFromString(backupPath)
	if err != nil {
		return false, fmt.Errorf("convert backup file path: %w", err)
	}

	r1, callErr := replaceFileWCall(
		uintptr(unsafe.Pointer(targetPathPtr)),
		uintptr(unsafe.Pointer(tempPathPtr)),
		uintptr(unsafe.Pointer(backupPathPtr)),
	)
	if r1 != 0 {
		keepBackup = false
		return false, nil
	}
	if callErr == nil || callErr == syscall.Errno(0) {
		keepBackup = false
		return false, syscall.EINVAL
	}

	if errors.Is(callErr, syscall.Errno(1177)) {
		restoreErr := renameFile(backupPath, targetPath)
		if restoreErr == nil {
			keepBackup = false
			return false, fmt.Errorf("replace existing file %s: %w", targetPath, callErr)
		}
		keepBackup = true
		return true, fmt.Errorf("replace existing file %s: %w; restore original from %s: %v", targetPath, callErr, backupPath, restoreErr)
	}

	keepBackup = false
	return false, callErr
}

func createBackupPath(targetPath string) (string, error) {
	dir := filepath.Dir(targetPath)
	file, err := os.CreateTemp(dir, "."+filepath.Base(targetPath)+".backup-*")
	if err != nil {
		return "", fmt.Errorf("create backup file in %s: %w", dir, err)
	}
	backupPath := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(backupPath)
		return "", fmt.Errorf("close backup file %s: %w", backupPath, err)
	}
	if err := os.Remove(backupPath); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("remove backup file %s: %w", backupPath, err)
	}
	return backupPath, nil
}

// Windows has no direct directory sync equivalent here; replacement is the
// commit point and the caller treats the later sync as best effort.
func syncParentDirPlatform(string) error {
	return nil
}
