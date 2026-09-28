package config

import (
	"fmt"
	"strings"
)

// CheckExpectedOrganization verifies that the configured target matches an
// independently supplied GitHub organization login. The expected value must
// come from the caller, not from the desired configuration.
func CheckExpectedOrganization(configured, expected string) error {
	configured = strings.TrimSpace(configured)
	expected = strings.TrimSpace(expected)
	switch {
	case expected == "":
		return fmt.Errorf("expected organization is required")
	case configured == "":
		return fmt.Errorf("configured organization is required")
	case !strings.EqualFold(configured, expected):
		return fmt.Errorf("configured organization %q does not match expected organization %q", configured, expected)
	default:
		return nil
	}
}
