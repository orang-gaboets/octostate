//go:build windows

package filereplace

import (
	"fmt"
	"syscall"
	"unsafe"
)

var replaceFileW = syscall.NewLazyDLL("kernel32.dll").NewProc("ReplaceFileW")

func replaceFilePlatform(tempPath, targetPath string) error {
	tempPathPtr, err := syscall.UTF16PtrFromString(tempPath)
	if err != nil {
		return fmt.Errorf("convert temporary file path: %w", err)
	}
	targetPathPtr, err := syscall.UTF16PtrFromString(targetPath)
	if err != nil {
		return fmt.Errorf("convert destination file path: %w", err)
	}

	r1, _, callErr := replaceFileW.Call(
		uintptr(unsafe.Pointer(targetPathPtr)),
		uintptr(unsafe.Pointer(tempPathPtr)),
		0,
		0,
		0,
		0,
	)
	if r1 != 0 {
		return nil
	}
	if callErr != syscall.Errno(0) {
		return callErr
	}
	return syscall.EINVAL
}

// Windows has no direct directory Sync equivalent here; ReplaceFileW covers
// the file swap and the filesystem handles the durability boundary.
func syncParentDirPlatform(string) error {
	return nil
}
