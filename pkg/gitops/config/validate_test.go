package config

import "testing"

func TestValidateValidConfig(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()

	report := Validate(cfg)
	if !report.Valid {
		t.Fatalf("expected valid report, got %#v", report)
	}
	if report.Summary.Repositories != 1 || report.Summary.Teams != 1 || report.Summary.Invites != 1 {
		t.Fatalf("unexpected summary counts: %#v", report.Summary)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("expected no errors, got %#v", report.Errors)
	}
	if report.Errors == nil || report.Warnings == nil {
		t.Fatalf("expected non-nil errors/warnings slices")
	}
}

func TestValidateMissingOrganization(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Organization = ""

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeMissingRequiredField)
}

func TestValidateDuplicateRepositories(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Repositories = append(cfg.Repositories, cfg.Repositories[0])

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateRepository)
}

func TestValidateDuplicateRepositoriesCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Repositories = append(cfg.Repositories, RepositorySpec{
		Owner:      "ORANG-GABOETS",
		Name:       "REPO-BUILDER",
		Visibility: "private",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateRepository)
}

func TestValidateDuplicateTeamSlugs(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams = append(cfg.Teams, TeamSpec{
		Slug: "platform",
		Name: "Platform Clone",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateTeamSlug)
}

func TestValidateDuplicateTeamSlugsCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams = append(cfg.Teams, TeamSpec{
		Slug: "PLATFORM",
		Name: "platform",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateTeamSlug)
}

func TestValidateInvalidInviteIdentityNone(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{Role: "direct_member"}

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteIdentityMultiple(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: "octocat",
		Email:    "octocat@example.com",
	}

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteIdentityMultipleAndNegativeUserIDReportsBoth(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: "octocat",
		UserID:   -1,
	}

	report := Validate(cfg)

	matches := 0
	for _, issue := range report.Errors {
		if issue.Code == ValidationIssueCodeInvalidInviteIdentity {
			matches++
		}
	}
	if matches < 2 {
		t.Fatalf("expected at least two invalid_invite_identity issues, got %#v", report.Errors)
	}
}

func TestValidateUnknownInviteTeamReference(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0].TeamSlugs = []string{"unknown-team"}

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeUnknownInviteTeamSlug)
}

func TestValidateInviteEmptyTeamSlug(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0].TeamSlugs = []string{"  "}

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "invites[0].team_slugs[0]", ValidationIssueCodeMissingRequiredField)
	assertNotHasIssueCode(t, report, ValidationIssueCodeUnknownInviteTeamSlug)
}

func TestValidateInviteTeamReferenceCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0].TeamSlugs = []string{"PLATFORM"}

	report := Validate(cfg)
	assertNotHasIssueCode(t, report, ValidationIssueCodeUnknownInviteTeamSlug)
}

func TestValidateUnknownTeamParentReference(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams[0].ParentSlug = "missing-parent"

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeUnknownTeamParentSlug)
}

func TestValidateTeamParentReferenceCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams = append(cfg.Teams, TeamSpec{
		Slug:       "security",
		Name:       "Security",
		ParentSlug: "PLATFORM",
	})

	report := Validate(cfg)
	assertNotHasIssueCode(t, report, ValidationIssueCodeUnknownTeamParentSlug)
}

func TestValidateTeamParentCycle(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams = []TeamSpec{
		{
			Slug:       "platform",
			Name:       "Platform",
			ParentSlug: "security",
		},
		{
			Slug:       "security",
			Name:       "Security",
			ParentSlug: "platform",
		},
	}

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeTeamParentCycle)
}

func TestValidateDuplicateTeamMembers(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams[0].Members = append(cfg.Teams[0].Members, TeamMemberSpec{
		Username: "alice",
		Role:     "member",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateTeamMember)
}

func TestValidateDuplicateTeamMembersCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams[0].Members = append(cfg.Teams[0].Members, TeamMemberSpec{
		Username: "ALICE",
		Role:     "member",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateTeamMember)
}

func TestValidateDuplicateTeamRepositories(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams[0].Repositories = append(cfg.Teams[0].Repositories, TeamRepositorySpec{
		Name:       "repo-builder",
		Permission: "pull",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateTeamRepository)
}

func TestValidateDuplicateTeamRepositoriesCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams[0].Repositories = append(cfg.Teams[0].Repositories, TeamRepositorySpec{
		Owner:      "ORANG-GABOETS",
		Name:       "REPO-BUILDER",
		Permission: "pull",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateTeamRepository)
}

func TestValidateInvalidEnums(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0].Role = "owner"
	cfg.Repositories[0].Visibility = "internal"
	cfg.Teams[0].Privacy = "visible"
	cfg.Teams[0].Members[0].Role = "owner"
	cfg.Teams[0].Repositories[0].Permission = "write"

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeInvalidEnum)
	if report.Summary.Errors < 5 {
		t.Fatalf("expected multiple invalid enum errors, got %#v", report.Errors)
	}
}

func TestValidateSlugNameMismatch(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams[0].Slug = "wrong-slug"

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeSlugNameMismatch)
}

func validOrganizationConfig() OrganizationConfig {
	return OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []InviteSpec{{
			Username:  "octocat",
			Role:      "direct_member",
			TeamSlugs: []string{"platform"},
		}},
		Repositories: []RepositorySpec{{
			Owner:       "orang-gaboets",
			Name:        "repo-builder",
			Template:    TemplateSpec{Owner: "orang-gaboets", Name: "repo-template"},
			Visibility:  "private",
			Description: "GitHub organization operations CLI",
			Homepage:    "https://github.com/orang-gaboets/repo-builder",
			Topics:      []string{"go", "gitops"},
		}},
		Teams: []TeamSpec{{
			Slug:        "platform",
			Name:        "Platform",
			Description: "Platform engineering",
			Privacy:     "closed",
			Members: []TeamMemberSpec{{
				Username: "alice",
				Role:     "maintainer",
			}},
			Repositories: []TeamRepositorySpec{{
				Name:       "repo-builder",
				Permission: "push",
			}},
		}},
	}
}

func assertHasIssueCode(t *testing.T, report ValidationReport, want ValidationIssueCode) {
	t.Helper()

	for _, issue := range report.Errors {
		if issue.Code == want {
			return
		}
	}

	t.Fatalf("expected issue code %q in %#v", want, report.Errors)
}

func assertNotHasIssueCode(t *testing.T, report ValidationReport, want ValidationIssueCode) {
	t.Helper()

	for _, issue := range report.Errors {
		if issue.Code == want {
			t.Fatalf("did not expect issue code %q in %#v", want, report.Errors)
		}
	}
}

func assertHasIssueAtPathAndCode(t *testing.T, report ValidationReport, wantPath string, wantCode ValidationIssueCode) {
	t.Helper()

	for _, issue := range report.Errors {
		if issue.Path == wantPath && issue.Code == wantCode {
			return
		}
	}

	t.Fatalf("expected issue path=%q code=%q in %#v", wantPath, wantCode, report.Errors)
}
