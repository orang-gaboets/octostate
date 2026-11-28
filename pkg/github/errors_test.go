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

func TestAPIErrorStringShowsOnlyReviewedDetails(t *testing.T) {
	const marker = "SYNTHETIC_SENSITIVE_VALUE"
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "message_marker",
			body: `{"message":"` + marker + `","documentation_url":"https://docs.github.com/rest"}`,
			want: "status 422 Unprocessable Entity",
		},
		{
			name: "docs_url_marker",
			body: `{"message":"Validation Failed","documentation_url":"https://example.test/` + marker + `"}`,
			want: "status 422 Unprocessable Entity",
		},
		{
			name: "field_message_marker",
			body: `{"message":"Validation Failed","errors":[{"resource":"Team","field":"name","code":"custom","message":"` + marker + `"}]}`,
			want: "status 422 Unprocessable Entity; Team.name: custom",
		},
		{
			name: "resource_marker",
			body: `{"message":"Validation Failed","errors":[{"resource":"` + marker + `","field":"name","code":"invalid"}]}`,
			want: "status 422 Unprocessable Entity; 1 unreviewed validation detail omitted",
		},
		{
			name: "field_marker",
			body: `{"message":"Validation Failed","errors":[{"resource":"Team","field":"` + marker + `","code":"invalid"}]}`,
			want: "status 422 Unprocessable Entity; 1 unreviewed validation detail omitted",
		},
		{
			name: "code_marker",
			body: `{"message":"Validation Failed","errors":[{"resource":"Team","field":"name","code":"` + marker + `"}]}`,
			want: "status 422 Unprocessable Entity; 1 unreviewed validation detail omitted",
		},
		{
			name: "mixed_safe_and_unsafe",
			body: `{"message":"Validation Failed","errors":[{"resource":"Team","field":"name","code":"already_exists"},{"resource":"` + marker + `","field":"name","code":"invalid"},{"resource":"Repository","field":"topics","code":"invalid"},{"resource":"Team","field":"name","code":"already_exists"},{"resource":"Team","field":"x","code":"` + marker + `"}]}`,
			want: "status 422 Unprocessable Entity; Team.name: already_exists; Repository.topics: invalid; 2 unreviewed validation details omitted",
		},
		{
			name: "non_json_body",
			body: "<html>" + marker + "</html>",
			want: "status 422 Unprocessable Entity",
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
			if got, want := err.Error(), "create team: validation failed\n"+tt.want; got != want {
				t.Fatalf("error text:\n got %q\nwant %q", got, want)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || !strings.Contains(string(apiErr.Body), marker) {
				t.Fatalf("APIError.Body must still carry the raw body for explicit inspection")
			}
			var ghErr *gh.ErrorResponse
			if !errors.As(err, &ghErr) {
				t.Fatalf("the go-github ErrorResponse must stay reachable through Unwrap")
			}
		})
	}
}

func TestAPIErrorStringIsLinearInValidationErrors(t *testing.T) {
	errs := make([]gh.Error, 10000)
	for i := range errs {
		errs[i] = gh.Error{Resource: "Team", Field: "name", Code: "invalid"}
	}
	e := &APIError{StatusCode: http.StatusUnprocessableEntity, err: &gh.ErrorResponse{Errors: errs}}
	allocs := testing.AllocsPerRun(5, func() { _ = e.Error() })
	if allocs > 50 {
		t.Fatalf("Error() made %.0f allocations for 10,000 validation errors", allocs)
	}
	if got, want := e.Error(), "status 422 Unprocessable Entity; Team.name: invalid"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
