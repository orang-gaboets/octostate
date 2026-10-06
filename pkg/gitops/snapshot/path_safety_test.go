package snapshot

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func physicalTempDir(t *testing.T) string {
	t.Helper()

	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temporary directory: %v", err)
	}
	return dir
}

func TestReadActualRejectsSymlinkedPathComponents(t *testing.T) {
	for _, component := range []string{"ancestor", "state-dir", "actual-dir", "snapshot-file"} {
		t.Run(component, func(t *testing.T) {
			root := physicalTempDir(t)
			targetStateDir := filepath.Join(root, "target-state")
			targetPath, err := WriteActual(targetStateDir, sampleSnapshot())
			if err != nil {
				t.Fatalf("write target snapshot: %v", err)
			}

			stateDir, linkPath := readPathWithSymlink(t, root, targetStateDir, targetPath, component)
			_, err = ReadActual(stateDir)
			assertUnsafeSnapshotPathError(t, err, linkPath)
			if strings.Contains(err.Error(), targetPath) {
				t.Fatalf("rejection error exposed the symlink target %q: %v", targetPath, err)
			}
		})
	}
}

func TestWriteActualRejectsSymlinkedPathComponentsAndPreservesTarget(t *testing.T) {
	for _, component := range []string{"ancestor", "state-dir", "actual-dir", "snapshot-file"} {
		t.Run(component, func(t *testing.T) {
			root := physicalTempDir(t)
			stateDir, linkPath, targetPath := writePathWithSymlink(t, root, component)
			before, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatalf("read redirected target before write: %v", err)
			}

			_, err = WriteActual(stateDir, sampleSnapshot())
			assertUnsafeSnapshotPathError(t, err, linkPath)
			if strings.Contains(err.Error(), targetPath) {
				t.Fatalf("rejection error exposed the symlink target %q: %v", targetPath, err)
			}

			after, err := os.ReadFile(targetPath)
			if err != nil {
				t.Fatalf("read redirected target after write: %v", err)
			}
			if !bytes.Equal(after, before) {
				t.Fatalf("rejected write changed redirected target: before %q, after %q", before, after)
			}
		})
	}
}

func TestActualSnapshotOperationsStayInOpenedDirectoryAfterSymlinkSwap(t *testing.T) {
	root := physicalTempDir(t)
	stateDir := filepath.Join(root, "state")
	snapshotPath, err := WriteActual(stateDir, sampleSnapshot())
	if err != nil {
		t.Fatalf("write initial snapshot: %v", err)
	}

	externalStateDir := filepath.Join(root, "external-state")
	externalPath, err := WriteActual(externalStateDir, sampleSnapshot())
	if err != nil {
		t.Fatalf("write external snapshot: %v", err)
	}
	externalBefore, err := os.ReadFile(externalPath)
	if err != nil {
		t.Fatalf("read external snapshot before swap: %v", err)
	}

	parent, name, err := openSnapshotParent(snapshotPath, false)
	if err != nil {
		t.Fatalf("open snapshot parent: %v", err)
	}
	defer func() {
		_ = parent.Close()
	}()

	actualDir := filepath.Dir(snapshotPath)
	movedActualDir := filepath.Join(root, "moved-actual")
	if err := os.Rename(actualDir, movedActualDir); err != nil {
		t.Fatalf("move opened snapshot directory: %v", err)
	}
	replaceDirectoryWithLink(t, actualDir, filepath.Dir(externalPath))

	replacement := sampleSnapshot()
	replacement.Organization = "replacement"
	if err := writeActualSnapshotAt(parent, name, snapshotPath, replacement); err != nil {
		t.Fatalf("write through opened snapshot directory: %v", err)
	}

	got, err := readActualSnapshotAt(parent, name, snapshotPath)
	if err != nil {
		t.Fatalf("read through opened snapshot directory: %v", err)
	}
	if got.Organization != replacement.Organization {
		t.Fatalf("organization = %q, want %q", got.Organization, replacement.Organization)
	}

	externalAfter, err := os.ReadFile(externalPath)
	if err != nil {
		t.Fatalf("read external snapshot after swap: %v", err)
	}
	if !bytes.Equal(externalAfter, externalBefore) {
		t.Fatalf("directory swap changed external snapshot: before %q, after %q", externalBefore, externalAfter)
	}
}

func TestOpenSnapshotParentRejectsComponentChangedAfterLstat(t *testing.T) {
	t.Parallel()

	root := physicalTempDir(t)
	stateDir := filepath.Join(root, "state")
	snapshotPath, err := WriteActual(stateDir, sampleSnapshot())
	if err != nil {
		t.Fatalf("write initial snapshot: %v", err)
	}

	replacementStateDir := filepath.Join(root, "replacement-state")
	replacementSnapshotPath, err := WriteActual(replacementStateDir, sampleSnapshot())
	if err != nil {
		t.Fatalf("write replacement snapshot: %v", err)
	}
	replacementBefore, err := os.ReadFile(replacementSnapshotPath)
	if err != nil {
		t.Fatalf("read replacement snapshot before swap: %v", err)
	}

	movedStateDir := filepath.Join(root, "moved-state")
	parent, _, err := openSnapshotParentWithHook(snapshotPath, false, func(componentPath string) {
		if componentPath != stateDir {
			return
		}
		if err := os.Rename(stateDir, movedStateDir); err != nil {
			t.Fatalf("move checked state directory: %v", err)
		}
		if err := os.Rename(replacementStateDir, stateDir); err != nil {
			t.Fatalf("replace checked state directory: %v", err)
		}
	})
	if parent != nil {
		_ = parent.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "changed while opening") {
		t.Fatalf("expected replaced component to be rejected after opening, got %v", err)
	}
	if strings.Contains(err.Error(), replacementStateDir) {
		t.Fatalf("component replacement error exposed the redirected path %q: %v", replacementStateDir, err)
	}

	replacementAfter, err := os.ReadFile(ActualPath(stateDir))
	if err != nil {
		t.Fatalf("read replacement snapshot after swap: %v", err)
	}
	if !bytes.Equal(replacementAfter, replacementBefore) {
		t.Fatalf("component replacement changed redirected snapshot: before %q, after %q", replacementBefore, replacementAfter)
	}
}

func TestReadActualRejectsSnapshotChangedAfterLstat(t *testing.T) {
	t.Parallel()

	root := physicalTempDir(t)
	snapshotPath, err := WriteActual(filepath.Join(root, "state"), sampleSnapshot())
	if err != nil {
		t.Fatalf("write initial snapshot: %v", err)
	}

	parent, name, err := openSnapshotParent(snapshotPath, false)
	if err != nil {
		t.Fatalf("open snapshot parent: %v", err)
	}
	t.Cleanup(func() {
		if err := parent.Close(); err != nil {
			t.Errorf("close snapshot parent: %v", err)
		}
	})

	movedPath := filepath.Join(root, "moved-snapshot.json")
	replacement := []byte(`{"organization":"redirected"}`)
	got, err := readActualSnapshotAtWithHook(parent, name, snapshotPath, func(path string) {
		if path != snapshotPath {
			t.Fatalf("snapshot hook path = %q, want %q", path, snapshotPath)
		}
		if err := os.Rename(snapshotPath, movedPath); err != nil {
			t.Fatalf("move checked snapshot: %v", err)
		}
		if err := os.WriteFile(snapshotPath, replacement, 0o600); err != nil {
			t.Fatalf("replace checked snapshot: %v", err)
		}
	})
	if got != nil {
		t.Fatalf("read returned replacement snapshot: %#v", got)
	}
	if err == nil || !strings.Contains(err.Error(), "changed while opening") {
		t.Fatalf("expected replaced snapshot to be rejected after opening, got %v", err)
	}
	if strings.Contains(err.Error(), "redirected") {
		t.Fatalf("snapshot replacement error exposed file contents: %v", err)
	}
}

func TestWriteActualCreatesCompletelyMissingStateDirectory(t *testing.T) {
	t.Parallel()

	stateDir := filepath.Join(physicalTempDir(t), "not-created", "nested", "state")
	path, err := WriteActual(stateDir, sampleSnapshot())
	if err != nil {
		t.Fatalf("write snapshot under missing state directory: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat newly created snapshot: %v", err)
	}
}

func TestSnapshotPathRejectsNonDirectoryParentAndNonRegularDestination(t *testing.T) {
	t.Parallel()

	root := physicalTempDir(t)
	parentFile := filepath.Join(root, "parent-file")
	if err := os.WriteFile(parentFile, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteActual(filepath.Join(parentFile, "state"), sampleSnapshot()); err == nil || !strings.Contains(strings.ToLower(err.Error()), "directory") {
		t.Fatalf("expected a directory-component error, got %v", err)
	}

	stateDir := filepath.Join(root, "state")
	destination := ActualPath(stateDir)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteActual(stateDir, sampleSnapshot()); err == nil || !strings.Contains(strings.ToLower(err.Error()), "regular file") {
		t.Fatalf("expected a regular-file destination error, got %v", err)
	}
	if _, err := ReadActual(stateDir); err == nil || !strings.Contains(strings.ToLower(err.Error()), "regular file") {
		t.Fatalf("expected a regular-file read error, got %v", err)
	}
}

func TestWriteActualRejectsSymlinkedSystemTempAliases(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("system temporary-path aliases are specific to macOS")
	}

	for _, aliasRoot := range []string{"/tmp", "/var/tmp"} {
		t.Run(aliasRoot, func(t *testing.T) {
			aliasDir, err := os.MkdirTemp(aliasRoot, "octostate-symlink-path-")
			if err != nil {
				t.Skipf("cannot create a temporary directory under %s: %v", aliasRoot, err)
			}
			defer func() {
				_ = os.RemoveAll(aliasDir)
			}()

			physicalDir, err := filepath.EvalSymlinks(aliasDir)
			if err != nil {
				t.Fatalf("resolve %s: %v", aliasRoot, err)
			}
			if physicalDir == aliasDir {
				t.Skipf("%s is not a symlink alias on this host", aliasRoot)
			}

			if _, err := WriteActual(aliasDir, sampleSnapshot()); err == nil {
				t.Fatalf("expected aliased path %s to be rejected", aliasDir)
			} else {
				assertUnsafeSnapshotPathError(t, err, aliasRoot)
			}
			if _, err := WriteActual(physicalDir, sampleSnapshot()); err != nil {
				t.Fatalf("write through physical path %s: %v", physicalDir, err)
			}
		})
	}
}

func readPathWithSymlink(t *testing.T, root, targetStateDir, targetPath, component string) (string, string) {
	t.Helper()

	switch component {
	case "ancestor":
		targetParent := filepath.Dir(targetStateDir)
		linkPath := filepath.Join(root, "state-parent-link")
		if err := os.Symlink(targetParent, linkPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		return filepath.Join(linkPath, filepath.Base(targetStateDir)), linkPath
	case "state-dir":
		linkPath := filepath.Join(root, "state-dir-link")
		if err := os.Symlink(targetStateDir, linkPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		return linkPath, linkPath
	case "actual-dir":
		stateDir := filepath.Join(root, "state")
		if err := os.Mkdir(stateDir, 0o755); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(stateDir, "actual")
		if err := os.Symlink(filepath.Dir(targetPath), linkPath); err != nil {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		return stateDir, linkPath
	case "snapshot-file":
		stateDir := filepath.Join(root, "state")
		if err := os.MkdirAll(filepath.Join(stateDir, "actual"), 0o755); err != nil {
			t.Fatal(err)
		}
		linkPath := ActualPath(stateDir)
		if err := os.Symlink(targetPath, linkPath); err != nil {
			t.Skipf("file symlinks unavailable: %v", err)
		}
		return stateDir, linkPath
	default:
		t.Fatalf("unknown symlink component %q", component)
		return "", ""
	}
}

func writePathWithSymlink(t *testing.T, root, component string) (string, string, string) {
	t.Helper()

	switch component {
	case "ancestor":
		targetParent := filepath.Join(root, "state-parent")
		stateDir := filepath.Join(targetParent, "state")
		if err := os.MkdirAll(filepath.Join(stateDir, "actual"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ActualPath(stateDir), []byte("unchanged target"), 0o600); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(root, "state-parent-link")
		if err := os.Symlink(targetParent, linkPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		return filepath.Join(linkPath, "state"), linkPath, ActualPath(stateDir)
	case "state-dir":
		stateDir := filepath.Join(root, "target-state")
		if err := os.MkdirAll(filepath.Join(stateDir, "actual"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ActualPath(stateDir), []byte("unchanged target"), 0o600); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(root, "state-dir-link")
		if err := os.Symlink(stateDir, linkPath); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		return linkPath, linkPath, ActualPath(stateDir)
	case "actual-dir":
		stateDir := filepath.Join(root, "state")
		if err := os.Mkdir(stateDir, 0o755); err != nil {
			t.Fatal(err)
		}
		targetPath := filepath.Join(root, "redirected", "actual", "snapshot.json")
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(targetPath, []byte("unchanged target"), 0o600); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(stateDir, "actual")
		if err := os.Symlink(filepath.Dir(targetPath), linkPath); err != nil {
			t.Skipf("directory symlinks unavailable: %v", err)
		}
		return stateDir, linkPath, targetPath
	case "snapshot-file":
		stateDir := filepath.Join(root, "state")
		if err := os.MkdirAll(filepath.Join(stateDir, "actual"), 0o755); err != nil {
			t.Fatal(err)
		}
		targetPath := filepath.Join(root, "redirected", "actual", "snapshot.json")
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(targetPath, []byte("unchanged target"), 0o600); err != nil {
			t.Fatal(err)
		}
		linkPath := ActualPath(stateDir)
		if err := os.Symlink(targetPath, linkPath); err != nil {
			t.Skipf("file symlinks unavailable: %v", err)
		}
		return stateDir, linkPath, targetPath
	default:
		t.Fatalf("unknown symlink component %q", component)
		return "", "", ""
	}
}

func assertUnsafeSnapshotPathError(t *testing.T, err error, component string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected unsafe snapshot path to be rejected")
	}
	message := strings.ToLower(err.Error())
	if !strings.Contains(message, "symlink") && !strings.Contains(message, "symbolic link") {
		t.Fatalf("error does not explain symlink rejection: %v", err)
	}
	if !strings.Contains(err.Error(), component) {
		t.Fatalf("error does not identify unsafe component %q: %v", component, err)
	}
}
