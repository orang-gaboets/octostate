package config

// ValidationIssueCode identifies the type of validation issue detected.
type ValidationIssueCode string

const (
	// ValidationIssueCodeLoadError indicates validation could not run because
	// configuration loading failed.
	ValidationIssueCodeLoadError ValidationIssueCode = "load_error"
	// ValidationIssueCodeMissingRequiredField indicates a required field was empty.
	ValidationIssueCodeMissingRequiredField ValidationIssueCode = "missing_required_field"
	// ValidationIssueCodeDuplicateRepository indicates a repository identity was repeated.
	ValidationIssueCodeDuplicateRepository ValidationIssueCode = "duplicate_repository"
	// ValidationIssueCodeDuplicateOrganizationMember indicates a top-level organization member was repeated.
	ValidationIssueCodeDuplicateOrganizationMember ValidationIssueCode = "duplicate_organization_member"
	// ValidationIssueCodeDuplicateTeamSlug indicates a team slug was repeated.
	ValidationIssueCodeDuplicateTeamSlug ValidationIssueCode = "duplicate_team_slug"
	// ValidationIssueCodeInvalidInviteIdentity indicates an invite identity declaration is invalid.
	ValidationIssueCodeInvalidInviteIdentity ValidationIssueCode = "invalid_invite_identity"
	// ValidationIssueCodeUnknownInviteTeamSlug indicates an invite referenced an unknown team slug.
	ValidationIssueCodeUnknownInviteTeamSlug ValidationIssueCode = "unknown_invite_team_slug"
	// ValidationIssueCodeUnknownOrganizationMember indicates a reference to an undeclared top-level organization member.
	ValidationIssueCodeUnknownOrganizationMember ValidationIssueCode = "unknown_organization_member"
	// ValidationIssueCodeUnknownTeamParentSlug indicates a team parent_slug reference could not be resolved.
	ValidationIssueCodeUnknownTeamParentSlug ValidationIssueCode = "unknown_team_parent_slug"
	// ValidationIssueCodeTeamParentCycle indicates the team parent graph contains a cycle.
	ValidationIssueCodeTeamParentCycle ValidationIssueCode = "team_parent_cycle"
	// ValidationIssueCodeDuplicateTeamMember indicates a team member was repeated in one team.
	ValidationIssueCodeDuplicateTeamMember ValidationIssueCode = "duplicate_team_member"
	// ValidationIssueCodeDuplicateTeamRepository indicates a team repository permission entry was repeated.
	ValidationIssueCodeDuplicateTeamRepository ValidationIssueCode = "duplicate_team_repository"
	// ValidationIssueCodeDuplicateOrganizationMemberInvite indicates a username is declared in both members and invites.
	ValidationIssueCodeDuplicateOrganizationMemberInvite ValidationIssueCode = "duplicate_organization_member_invite"
	// ValidationIssueCodeInvalidEnum indicates a field has an unsupported enum value.
	ValidationIssueCodeInvalidEnum ValidationIssueCode = "invalid_enum"
	// ValidationIssueCodeInvalidFieldValue indicates a field value is invalid for its schema.
	ValidationIssueCodeInvalidFieldValue ValidationIssueCode = "invalid_field_value"
	// ValidationIssueCodeSlugNameMismatch indicates a team slug does not match its normalized name.
	ValidationIssueCodeSlugNameMismatch ValidationIssueCode = "slug_name_mismatch"
)

// ValidationReport is the structured output produced by semantic validation.
type ValidationReport struct {
	Valid    bool              `json:"valid"`
	Summary  ValidationSummary `json:"summary"`
	Errors   []ValidationIssue `json:"errors"`
	Warnings []ValidationIssue `json:"warnings"`
}

// ValidationSummary contains the high-level object counts and issue counts.
type ValidationSummary struct {
	Repositories int `json:"repositories"`
	Members      int `json:"members"`
	Teams        int `json:"teams"`
	Invites      int `json:"invites"`
	Errors       int `json:"errors"`
	Warnings     int `json:"warnings"`
}

// ValidationIssue describes a single semantic validation failure or warning.
type ValidationIssue struct {
	Path    string              `json:"path,omitempty"`
	Code    ValidationIssueCode `json:"code"`
	Message string              `json:"message"`
}
