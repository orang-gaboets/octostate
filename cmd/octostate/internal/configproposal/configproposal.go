// Package configproposal applies validated local mutations to organization
// configuration files.
package configproposal

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// Mutation updates a loaded organization config before it is validated and
// written back to disk.
type Mutation func(*gitopsconfig.OrganizationConfig) error

// ApplyToConfigFile loads, validates, mutates, and atomically replaces an
// organization config file with its canonical YAML representation.
// It returns whether the file contents changed.
func ApplyToConfigFile(path string, expectedOrg string, mutate Mutation) (bool, error) {
	cfg, err := gitopsconfig.LoadFile(path)
	if err != nil {
		return false, err
	}
	if report := gitopsconfig.Validate(cfg); !report.Valid {
		return false, fmt.Errorf("validate loaded config: %#v", report.Errors)
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Organization), strings.TrimSpace(expectedOrg)) {
		return false, fmt.Errorf("organization mismatch: config organization %q does not match expected organization %q", cfg.Organization, expectedOrg)
	}
	if mutate == nil {
		return false, fmt.Errorf("config mutation is required")
	}
	before, err := gitopsconfig.EncodeYAML(cfg)
	if err != nil {
		return false, err
	}
	if err := mutate(&cfg); err != nil {
		return false, fmt.Errorf("mutate config: %w", err)
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Organization), strings.TrimSpace(expectedOrg)) {
		return false, fmt.Errorf("organization mismatch: config organization %q does not match expected organization %q", cfg.Organization, expectedOrg)
	}
	if report := gitopsconfig.Validate(cfg); !report.Valid {
		return false, fmt.Errorf("validate mutated config: %#v", report.Errors)
	}
	after, err := gitopsconfig.EncodeYAML(cfg)
	if err != nil {
		return false, err
	}
	if bytes.Equal(before, after) {
		return false, nil
	}

	sourceInfo, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("stat config file: %w", err)
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".organization.yaml-*")
	if err != nil {
		return false, fmt.Errorf("create temporary config file: %w", err)
	}
	tempPath := tempFile.Name()
	replaced := false
	defer func() {
		if !replaced {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(after); err != nil {
		return false, fmt.Errorf("write temporary config file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return false, fmt.Errorf("close temporary config file: %w", err)
	}
	if err := os.Chmod(tempPath, sourceInfo.Mode().Perm()); err != nil {
		return false, fmt.Errorf("set temporary config file permissions: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return false, fmt.Errorf("replace config file: %w", err)
	}

	replaced = true
	return true, nil
}
