// Package configproposal applies validated local mutations to organization
// configuration files.
package configproposal

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/filereplace"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// Mutation updates a loaded organization config before it is validated and
// written back to disk.
type Mutation func(*gitopsconfig.OrganizationConfig) error

// ApplyToConfigFile loads, validates, mutates, and atomically replaces an
// organization config file with its canonical YAML representation.
// It returns whether the file contents changed.
func ApplyToConfigFile(path string, expectedOrg string, mutate Mutation) (bool, error) {
	if _, err := filereplace.StatExistingRegularFile(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, &gitopsconfig.LoadError{
				Kind: gitopsconfig.LoadErrorMissingFile,
				Path: path,
				Err:  err,
			}
		}
		return false, err
	}

	cfg, err := gitopsconfig.LoadFile(path)
	if err != nil {
		return false, err
	}
	if report := gitopsconfig.Validate(cfg); !report.Valid {
		return false, fmt.Errorf("validate loaded config: %s", formatValidationIssues(report.Errors))
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
		return false, fmt.Errorf("validate mutated config: %s", formatValidationIssues(report.Errors))
	}
	after, err := gitopsconfig.EncodeYAML(cfg)
	if err != nil {
		return false, err
	}
	if bytes.Equal(before, after) {
		return false, nil
	}

	if err := filereplace.Replace(path, after); err != nil {
		return false, fmt.Errorf("replace config file: %w", err)
	}

	return true, nil
}

func formatValidationIssues(issues []gitopsconfig.ValidationIssue) string {
	parts := make([]string, 0, len(issues))
	for _, issue := range issues {
		switch {
		case issue.Path != "" && issue.Code != "":
			parts = append(parts, fmt.Sprintf("%s (%s): %s", issue.Path, issue.Code, issue.Message))
		case issue.Path != "":
			parts = append(parts, fmt.Sprintf("%s: %s", issue.Path, issue.Message))
		case issue.Code != "":
			parts = append(parts, fmt.Sprintf("%s: %s", issue.Code, issue.Message))
		default:
			parts = append(parts, issue.Message)
		}
	}
	return strings.Join(parts, "; ")
}
