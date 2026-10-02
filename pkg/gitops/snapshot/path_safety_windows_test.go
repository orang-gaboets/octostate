//go:build windows

package snapshot

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func replaceDirectoryWithLink(t *testing.T, path, target string) {
	t.Helper()
	command := fmt.Sprintf(`mklink /J "%s" "%s"`, path, target)
	output, err := exec.Command("cmd.exe", "/c", command).CombinedOutput()
	if err != nil {
		t.Fatalf("replace directory with junction: %v: %s", err, output)
	}
}

func TestWriteActualRejectsWindowsJunctionPath(t *testing.T) {
	root := physicalTempDir(t)
	stateDir := filepath.Join(root, "state")
	if err := os.Mkdir(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	externalStateDir := filepath.Join(root, "external-state")
	targetPath, err := WriteActual(externalStateDir, sampleSnapshot())
	if err != nil {
		t.Fatalf("write target snapshot: %v", err)
	}
	externalActual := filepath.Dir(targetPath)
	before, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}

	junctionPath := filepath.Join(stateDir, "actual")
	replaceDirectoryWithLink(t, junctionPath, externalActual)

	_, err = ReadActual(stateDir)
	if err == nil || !strings.Contains(err.Error(), junctionPath) || !hasSymlinkOrReparseReason(err) {
		t.Fatalf("expected junction path to be identified in read rejection error, got %v", err)
	}
	if strings.Contains(err.Error(), targetPath) {
		t.Fatalf("read rejection error exposed the junction target %q: %v", targetPath, err)
	}

	_, err = WriteActual(stateDir, sampleSnapshot())
	if err == nil || !strings.Contains(err.Error(), junctionPath) || !hasSymlinkOrReparseReason(err) {
		t.Fatalf("expected junction path to be identified in rejection error, got %v", err)
	}
	if strings.Contains(err.Error(), targetPath) {
		t.Fatalf("write rejection error exposed the junction target %q: %v", targetPath, err)
	}

	after, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("rejected write changed junction target: before %q, after %q", before, after)
	}
}

func hasSymlinkOrReparseReason(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "symbolic link") || strings.Contains(message, "reparse")
}
