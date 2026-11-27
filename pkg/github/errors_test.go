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

func TestAPIErrorStringOmitsResponseBody(t *testing.T) {
	const secret = "SYNTHETIC_SENSITIVE_VALUE"
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "json_validation_errors",
			body: `{"message":"Validation Failed","errors":[{"resource":"Team","field":"name","code":"custom","message":"` + secret + `"}],"documentation_url":"https://docs.github.com/rest","status":"422"}`,
			want: []string{"status 422: Validation Failed", "(docs: https://docs.github.com/rest)", "Team.name: custom"},
		},
		{
			name: "non_json_body",
			body: "<html>" + secret + "</html>",
			want: []string{"status 422"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: http.StatusUnprocessableEntity,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
				Header:     http.Header{},
			}
			err := WrapError(gh.CheckResponse(resp), "create team")
			if !errors.Is(err, ErrValidationFailed) {
				t.Fatalf("expected ErrValidationFailed, got %v", err)
			}
			got := err.Error()
			if strings.Contains(got, secret) {
				t.Fatalf("error text exposes response body: %q", got)
			}
			if strings.Contains(got, "(docs: )") {
				t.Fatalf("error text has an empty docs URL: %q", got)
			}
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("error text %q missing %q", got, want)
				}
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || !strings.Contains(string(apiErr.Body), secret) {
				t.Fatalf("APIError.Body must still carry the raw body for explicit inspection")
			}
		})
	}
}
