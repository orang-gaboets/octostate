package config

import (
	"strings"
	"testing"
)

func TestCheckExpectedOrganization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		configured string
		expected   string
		wantError  string
	}{
		{name: "exact match", configured: "org-a", expected: "org-a"},
		{name: "normalized match", configured: " Org-A ", expected: " org-a "},
		{name: "mismatch", configured: "org-b", expected: "org-a", wantError: `configured organization "org-b" does not match expected organization "org-a"`},
		{name: "Unicode fold in expected value", configured: "kube", expected: "Kube", wantError: "expected organization must contain only ASCII characters"},
		{name: "Unicode configured value", configured: "Kube", expected: "kube", wantError: "configured organization must contain only ASCII characters"},
		{name: "missing expected", configured: "org-a", expected: "", wantError: "expected organization is required"},
		{name: "whitespace expected", configured: "org-a", expected: "  ", wantError: "expected organization is required"},
		{name: "missing configured", configured: "  ", expected: "org-a", wantError: "configured organization is required"},
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
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("expected error containing %q, got %v", tt.wantError, err)
			}
		})
	}
}
