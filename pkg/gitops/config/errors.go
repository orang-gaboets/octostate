package config

import "fmt"

// ExpectedOrganizationErrorKind classifies why target binding failed.
type ExpectedOrganizationErrorKind string

const (
	// ExpectedOrganizationErrorMissingExpected indicates the caller omitted the expected organization.
	ExpectedOrganizationErrorMissingExpected ExpectedOrganizationErrorKind = "missing_expected"
	// ExpectedOrganizationErrorInvalidExpected indicates the expected organization is not a valid GitHub login.
	ExpectedOrganizationErrorInvalidExpected ExpectedOrganizationErrorKind = "invalid_expected"
	// ExpectedOrganizationErrorMissingConfigured indicates the desired configuration omitted its organization.
	ExpectedOrganizationErrorMissingConfigured ExpectedOrganizationErrorKind = "missing_configured"
	// ExpectedOrganizationErrorInvalidConfigured indicates the configured organization is not a valid GitHub login.
	ExpectedOrganizationErrorInvalidConfigured ExpectedOrganizationErrorKind = "invalid_configured"
	// ExpectedOrganizationErrorMismatch indicates the expected and configured organizations differ.
	ExpectedOrganizationErrorMismatch ExpectedOrganizationErrorKind = "mismatch"
)

// ExpectedOrganizationError describes a failure to bind a configured target
// to an independently supplied expected organization. Inspect Kind to classify
// the error; Error returns its human-readable message.
type ExpectedOrganizationError struct {
	// Kind classifies the validation failure.
	Kind ExpectedOrganizationErrorKind

	configured string
	expected   string
}

// Error implements the error interface.
func (e *ExpectedOrganizationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	switch e.Kind {
	case ExpectedOrganizationErrorMissingExpected:
		return "expected organization is required"
	case ExpectedOrganizationErrorInvalidExpected:
		return "expected organization must be a valid GitHub login"
	case ExpectedOrganizationErrorMissingConfigured:
		return "configured organization is required"
	case ExpectedOrganizationErrorInvalidConfigured:
		return "configured organization must be a valid GitHub login"
	case ExpectedOrganizationErrorMismatch:
		return fmt.Sprintf("configured organization %q does not match expected organization %q", e.configured, e.expected)
	default:
		return "expected organization validation failed"
	}
}

// LoadErrorKind classifies the loader failure so later commands can turn it
// into structured reports without reparsing free-form strings.
type LoadErrorKind string

const (
	// LoadErrorInvalidDir indicates the provided config directory was invalid.
	LoadErrorInvalidDir LoadErrorKind = "invalid_dir"
	// LoadErrorMissingFile indicates the canonical organization file was absent.
	LoadErrorMissingFile LoadErrorKind = "missing_file"
	// LoadErrorReadFile indicates the organization file could not be read.
	LoadErrorReadFile LoadErrorKind = "read_file"
	// LoadErrorDecodeFile indicates the organization file could not be decoded.
	LoadErrorDecodeFile LoadErrorKind = "decode_file"
)

// LoadError describes a configuration loading failure with a stable kind and
// the path involved.
type LoadError struct {
	Kind LoadErrorKind
	Path string
	Err  error
}

// Error implements the error interface.
func (e *LoadError) Error() string {
	switch e.Kind {
	case LoadErrorInvalidDir:
		return fmt.Sprintf("invalid config directory %q: %v", e.Path, e.Err)
	case LoadErrorMissingFile:
		return fmt.Sprintf("required config file %q not found: %v", e.Path, e.Err)
	case LoadErrorReadFile:
		return fmt.Sprintf("read config file %q: %v", e.Path, e.Err)
	case LoadErrorDecodeFile:
		return fmt.Sprintf("decode config file %q: %v", e.Path, e.Err)
	default:
		return fmt.Sprintf("load config %q: %v", e.Path, e.Err)
	}
}

// Unwrap returns the underlying error.
func (e *LoadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
