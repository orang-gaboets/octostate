package config

import "strings"

// ValidationError carries a full semantic validation report.
type ValidationError struct {
	Report ValidationReport
}

// ValidateAndError validates cfg and returns a typed error when invalid.
func ValidateAndError(cfg OrganizationConfig) error {
	report := Validate(cfg)
	if report.Valid {
		return nil
	}
	return &ValidationError{Report: report}
}

// Error formats validation issues deterministically in report order.
func (e *ValidationError) Error() string {
	lines := []string{"organization config validation failed:"}
	for _, issue := range e.Report.Errors {
		line := "- "
		if issue.Path != "" {
			line += issue.Path + ": "
		}
		line += string(issue.Code) + ": " + issue.Message
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
