// Package testconfig provides shared test helpers for loading GitOps config.
package testconfig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// LoadDesiredConfig writes one organization.yaml fixture and loads it through
// the real config loader so tests exercise the same YAML decoding path.
func LoadDesiredConfig(t testing.TB, contents string) config.OrganizationConfig {
	t.Helper()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "organization.yaml")
	if err := os.WriteFile(configPath, []byte(contents), 0o600); err != nil {
		t.Fatalf("write organization config: %v", err)
	}

	cfg, err := config.LoadDir(configDir)
	if err != nil {
		t.Fatalf("load desired config: %v", err)
	}
	return cfg
}
