package github

import (
	"errors"
	"fmt"
	"io"
	"net/http"

	gh "github.com/google/go-github/v55/github"
)

// APIError represents a GitHub API error with additional context.
type APIError struct {
	StatusCode       int
	Message          string
	DocumentationURL string
	Body             []byte
	err              error
}

// Error implements the error interface.
func (e *APIError) Error() string {
	return fmt.Sprintf("status %d: %s (docs: %s) body: %s", e.StatusCode, e.Message, e.DocumentationURL, string(e.Body))
}

// Unwrap returns the underlying error.
func (e *APIError) Unwrap() error { return e.err }

var (
	// ErrUnauthorized is returned when the request is unauthorized.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrNotFound is returned when the resource cannot be found.
	ErrNotFound = errors.New("not found")
	// ErrValidationFailed is returned when validation fails.
	ErrValidationFailed = errors.New("validation failed")
	// ErrNilService is returned when a required GitHub service is nil.
	ErrNilService = errors.New("service is nil")
	// ErrMissingRequiredField is returned when a required field is missing.
	ErrMissingRequiredField = errors.New("missing required field")
)

// WrapError converts a go-github ErrorResponse into an APIError with additional context.
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	var resp *gh.ErrorResponse
	if !errors.As(err, &resp) {
		return err
	}
	var body []byte
	if resp.Response != nil && resp.Response.Body != nil {
		if b, readErr := io.ReadAll(resp.Response.Body); readErr == nil {
			body = b
		}
	}
	status := 0
	if resp.Response != nil {
		status = resp.Response.StatusCode
	}
	apiErr := &APIError{
		StatusCode:       status,
		Message:          resp.Message,
		DocumentationURL: resp.DocumentationURL,
		Body:             body,
		err:              resp,
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %w: %w", message, ErrUnauthorized, apiErr)
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w: %w", message, ErrNotFound, apiErr)
	case http.StatusUnprocessableEntity:
		return fmt.Errorf("%s: %w: %w", message, ErrValidationFailed, apiErr)
	default:
		return apiErr
	}
}
