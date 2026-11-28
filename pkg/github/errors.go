package github

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	gh "github.com/google/go-github/v88/github"
)

// APIError represents a GitHub API error with additional context.
type APIError struct {
	StatusCode       int
	Message          string
	DocumentationURL string
	Body             []byte
	err              error
}

// Error implements the error interface. GitHub-supplied text is untrusted,
// so the string contains only the status code, its local status text, and
// validation details whose resource, field, and code are all on the reviewed
// lists below. Message, DocumentationURL, Body, and free-text validation
// messages are left out; callers can still inspect those fields explicitly.
func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString("status ")
	b.WriteString(strconv.Itoa(e.StatusCode))
	if text := http.StatusText(e.StatusCode); text != "" {
		b.WriteString(" ")
		b.WriteString(text)
	}
	var resp *gh.ErrorResponse
	if !errors.As(e.err, &resp) {
		return b.String()
	}
	seen := make(map[gh.Error]bool)
	omitted := 0
	for _, fieldErr := range resp.Errors {
		if !reviewedResources[fieldErr.Resource] || !reviewedFields[fieldErr.Field] || !reviewedCodes[fieldErr.Code] {
			omitted++
			continue
		}
		key := gh.Error{Resource: fieldErr.Resource, Field: fieldErr.Field, Code: fieldErr.Code}
		if seen[key] {
			continue
		}
		seen[key] = true
		b.WriteString("; ")
		b.WriteString(key.Resource)
		b.WriteString(".")
		b.WriteString(key.Field)
		b.WriteString(": ")
		b.WriteString(key.Code)
	}
	switch omitted {
	case 0:
	case 1:
		b.WriteString("; 1 unreviewed validation detail omitted")
	default:
		fmt.Fprintf(&b, "; %d unreviewed validation details omitted", omitted)
	}
	return b.String()
}

// Reviewed validation detail values. Codes are GitHub's documented
// validation codes; resources and fields are the ones Octostate writes.
var (
	reviewedCodes     = setOf("missing", "missing_field", "invalid", "already_exists", "unprocessable", "custom")
	reviewedResources = setOf("Team", "Repository", "Membership", "OrganizationInvitation", "Invitation")
	reviewedFields    = setOf(
		"name", "description", "privacy", "parent_team_id", "permission", "role",
		"visibility", "homepage", "topics", "allow_forking", "archived", "is_template",
		"email", "invitee_id", "team_ids",
	)
)

func setOf(values ...string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

// Unwrap returns the underlying error.
func (e *APIError) Unwrap() error { return e.err }

var (
	// ErrNoValidCredentials is returned when no valid credentials are provided.
	ErrNoValidCredentials = errors.New("no valid credentials provided, either a personal access token or GitHub App credentials must be supplied")
	// ErrConflictingCredentials is returned when both OAuth token and app credentials are provided.
	ErrConflictingCredentials = errors.New("conflicting credentials: both OAuth token and app credentials provided")
	// ErrNoContent is returned when no content is found.
	ErrNoContent = errors.New("no content found")
	// ErrTemporaryRedirect is returned when a temporary redirect occurs.
	ErrTemporaryRedirect = errors.New("temporary redirect")
	// ErrUnauthorized is returned when the request is unauthorized.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrForbidden is returned when the request is forbidden.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound is returned when the resource cannot be found.
	ErrNotFound = errors.New("not found")
	// ErrValidationFailed is returned when validation fails.
	ErrValidationFailed = errors.New("validation failed")
	// ErrNilService is returned when a required GitHub service is nil.
	ErrNilService = errors.New("service is nil")
	// ErrMissingRequiredField is returned when a required field is missing.
	ErrMissingRequiredField = errors.New("missing required field")
	// ErrInvalidFieldValue is returned when a field has an invalid value.
	ErrInvalidFieldValue = errors.New("invalid field value")
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
		readBody, readErr := io.ReadAll(resp.Response.Body)
		closeErr := resp.Response.Body.Close()
		body = readBody

		switch {
		case readErr != nil && closeErr != nil:
			return errors.Join(
				fmt.Errorf("%s: read response body: %w", message, readErr),
				fmt.Errorf("%s: close response body: %w", message, closeErr),
			)
		case readErr != nil:
			return fmt.Errorf("%s: read response body: %w", message, readErr)
		case closeErr != nil:
			return fmt.Errorf("%s: close response body: %w", message, closeErr)
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
	case http.StatusNoContent:
		return fmt.Errorf("%s: %w", message, errors.Join(ErrNoContent, apiErr))
	case http.StatusTemporaryRedirect:
		return fmt.Errorf("%s: %w", message, errors.Join(ErrTemporaryRedirect, apiErr))
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %w", message, errors.Join(ErrUnauthorized, apiErr))
	case http.StatusForbidden:
		return fmt.Errorf("%s: %w", message, errors.Join(ErrForbidden, apiErr))
	case http.StatusNotFound:
		return fmt.Errorf("%s: %w", message, errors.Join(ErrNotFound, apiErr))
	case http.StatusUnprocessableEntity:
		return fmt.Errorf("%s: %w", message, errors.Join(ErrValidationFailed, apiErr))
	default:
		return fmt.Errorf("%s: %w", message, apiErr)
	}
}
