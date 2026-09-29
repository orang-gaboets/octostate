package config

import (
	"errors"
	"testing"
)

func TestCheckExpectedOrganization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured string
		expected   string
		wantError  string
		wantKind   ExpectedOrganizationErrorKind
	}{
		{name: "exact match", configured: "org-a", expected: "org-a"},
		{name: "normalized match", configured: " Org-A ", expected: " org-a "},
		{name: "mismatch", configured: "org-b", expected: "org-a", wantError: `configured organization "org-b" does not match expected organization "org-a"`, wantKind: ExpectedOrganizationErrorMismatch},
		{name: "Unicode fold in expected value", configured: "kube", expected: "Kube", wantError: "expected organization must be a valid GitHub login", wantKind: ExpectedOrganizationErrorInvalidExpected},
		{name: "Unicode configured value", configured: "Kube", expected: "kube", wantError: "configured organization must be a valid GitHub login", wantKind: ExpectedOrganizationErrorInvalidConfigured},
		{name: "query delimiter in expected value", configured: "victim", expected: "victim?foo", wantError: "expected organization must be a valid GitHub login", wantKind: ExpectedOrganizationErrorInvalidExpected},
		{name: "fragment delimiter in expected value", configured: "victim", expected: "victim#foo", wantError: "expected organization must be a valid GitHub login", wantKind: ExpectedOrganizationErrorInvalidExpected},
		{name: "slash in expected value", configured: "victim", expected: "victim/foo", wantError: "expected organization must be a valid GitHub login", wantKind: ExpectedOrganizationErrorInvalidExpected},
		{name: "encoded slash in expected value", configured: "victim", expected: "victim%2Ffoo", wantError: "expected organization must be a valid GitHub login", wantKind: ExpectedOrganizationErrorInvalidExpected},
		{name: "query delimiter in configured value", configured: "victim?foo", expected: "victim", wantError: "configured organization must be a valid GitHub login", wantKind: ExpectedOrganizationErrorInvalidConfigured},
		{name: "missing expected", configured: "org-a", expected: "", wantError: "expected organization is required", wantKind: ExpectedOrganizationErrorMissingExpected},
		{name: "whitespace expected", configured: "org-a", expected: "  ", wantError: "expected organization is required", wantKind: ExpectedOrganizationErrorMissingExpected},
		{name: "missing configured", configured: "  ", expected: "org-a", wantError: "configured organization is required", wantKind: ExpectedOrganizationErrorMissingConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := CheckExpectedOrganization(tt.configured, tt.expected)
			if tt.wantError == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantError {
				t.Fatalf("expected error %q, got %v", tt.wantError, err)
			}
			var targetErr *ExpectedOrganizationError
			if !errors.As(err, &targetErr) || targetErr.Kind != tt.wantKind {
				t.Fatalf("expected typed error kind %q, got %#v", tt.wantKind, targetErr)
			}
		})
	}
}
