package config

import "fmt"

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
