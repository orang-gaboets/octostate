package exitcode

import (
	"errors"
	"fmt"
)

// Error represents a process exit request with a specific status code.
type Error struct {
	Code int
	Err  error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e == nil {
		return "exit error: <nil>"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit with code %d", e.Code)
}

// Unwrap returns the wrapped error, if present.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// New constructs an exit error with the provided status code and optional
// wrapped error.
func New(code int, err error) error {
	return &Error{
		Code: code,
		Err:  err,
	}
}

// Code extracts a typed exit code from err, if present.
func Code(err error) (int, bool) {
	var exitErr *Error
	if !errors.As(err, &exitErr) || exitErr == nil {
		return 0, false
	}
	return exitErr.Code, true
}
