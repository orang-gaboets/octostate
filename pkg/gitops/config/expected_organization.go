package config

import (
	"strings"
)

// CheckExpectedOrganization verifies that the configured target matches an
// independently supplied GitHub organization login. The expected value must
// come from the caller, not from the desired configuration. It returns an
// *ExpectedOrganizationError on failure; inspect its Kind for a stable reason.
func CheckExpectedOrganization(configured, expected string) error {
	configured = strings.TrimSpace(configured)
	expected = strings.TrimSpace(expected)
	switch {
	case expected == "":
		return &ExpectedOrganizationError{Kind: ExpectedOrganizationErrorMissingExpected}
	case !isValidGitHubUsername(expected):
		return &ExpectedOrganizationError{Kind: ExpectedOrganizationErrorInvalidExpected}
	case configured == "":
		return &ExpectedOrganizationError{Kind: ExpectedOrganizationErrorMissingConfigured}
	case !isValidGitHubUsername(configured):
		return &ExpectedOrganizationError{Kind: ExpectedOrganizationErrorInvalidConfigured}
	case !strings.EqualFold(configured, expected):
		return &ExpectedOrganizationError{
			Kind:       ExpectedOrganizationErrorMismatch,
			configured: configured,
			expected:   expected,
		}
	default:
		return nil
	}
}
