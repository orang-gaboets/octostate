package github

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
)

func TestWrapError(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		sentinel error
	}{
		{"unauthorized", http.StatusUnauthorized, ErrUnauthorized},
		{"not_found", http.StatusNotFound, ErrNotFound},
		{"validation_failed", http.StatusUnprocessableEntity, ErrValidationFailed},
		{"internal_error", http.StatusInternalServerError, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &gh.ErrorResponse{
				Response: &http.Response{
					StatusCode: tt.status,
					Body:       io.NopCloser(strings.NewReader("body")),
				},
				Message:          "msg",
				DocumentationURL: "docs",
			}

			err := WrapError(resp, "test error")
			if tt.sentinel != nil {
				if !errors.Is(err, tt.sentinel) {
					t.Fatalf("expected errors.Is to match sentinel %v", tt.sentinel)
				}
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected APIError in error chain")
			}
			if apiErr.StatusCode != tt.status {
				t.Errorf("status code: expected %d, got %d", tt.status, apiErr.StatusCode)
			}
			if apiErr.Message != "msg" {
				t.Errorf("message mismatch: %s", apiErr.Message)
			}
			if apiErr.DocumentationURL != "docs" {
				t.Errorf("docs URL mismatch: %s", apiErr.DocumentationURL)
			}
			if string(apiErr.Body) != "body" {
				t.Errorf("body mismatch: %s", string(apiErr.Body))
			}
		})
	}
}
