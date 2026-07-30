// Package configproposal applies validated local mutations to organization
// configuration files.
package configproposal

import (
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
func ApplyToConfigFile(path string, expectedOrg string, mutate Mutation) error {
	cfg, err := gitopsconfig.LoadFile(path)
	if err != nil {
		return err
	}
	if report := gitopsconfig.Validate(cfg); !report.Valid {
		return fmt.Errorf("validate loaded config: %#v", report.Errors)
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.Organization), strings.TrimSpace(expectedOrg)) {
		return fmt.Errorf("organization mismatch: config organization %q does not match expected organization %q", cfg.Organization, expectedOrg)
	}
	if mutate == nil {
		return fmt.Errorf("config mutation is required")
	}
	if err := mutate(&cfg); err != nil {
		return fmt.Errorf("mutate config: %w", err)
	}
	if report := gitopsconfig.Validate(cfg); !report.Valid {
		return fmt.Errorf("validate mutated config: %#v", report.Errors)
	}

	contents, err := gitopsconfig.EncodeYAML(cfg)
	if err != nil {
		return err
	}

	tempFile, err := os.CreateTemp(filepath.Dir(path), ".organization.yaml-*")
	if err != nil {
		return fmt.Errorf("create temporary config file: %w", err)
	}
	tempPath := tempFile.Name()
	replaced := false
	defer func() {
		if !replaced {
			_ = tempFile.Close()
			_ = os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(contents); err != nil {
		return fmt.Errorf("write temporary config file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temporary config file: %w", err)
	}
	if err := os.Chmod(tempPath, 0o644); err != nil {
		return fmt.Errorf("set temporary config file permissions: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config file: %w", err)
	}

	replaced = true
	return nil
}
