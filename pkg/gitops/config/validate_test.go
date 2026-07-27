package config

import "testing"

func TestValidateValidConfig(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()

	report := Validate(cfg)
	if !report.Valid {
		t.Fatalf("expected valid report, got %#v", report)
	}
	if report.Summary.Repositories != 1 || report.Summary.Members != 1 || report.Summary.Teams != 1 || report.Summary.Invites != 1 {
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
		Name:       "OCTOSTATE",
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

func TestValidateDuplicateOrganizationMembers(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Members = append(cfg.Members, OrganizationMemberSpec{
		Username: "alice",
		Role:     "admin",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateOrganizationMember)
}

func TestValidateDuplicateOrganizationMembersCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Members = append(cfg.Members, OrganizationMemberSpec{
		Username: "ALICE",
		Role:     "admin",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateOrganizationMember)
}

func TestValidateInvalidOrganizationMemberUsername(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Members[0].Username = "not a valid login"

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "members[0].username", ValidationIssueCodeInvalidFieldValue)
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
		Username: optionalString("octocat"),
		Email:    optionalString("octocat@example.com"),
	}

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteIdentityMultipleAndNegativeUserIDReportsBoth(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: optionalString("octocat"),
		UserID:   optionalInt64(-1),
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

func TestValidateInvalidInviteIdentityMultipleAndZeroUserIDReportsBoth(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: optionalString("octocat"),
		UserID:   optionalInt64(0),
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

func TestValidateInvalidInviteIdentityMultipleAndInvalidEmailReportsBoth(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: optionalString("octocat"),
		Email:    optionalString("not-an-email"),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0]", ValidationIssueCodeInvalidInviteIdentity)
	assertHasIssueAtPathAndCode(t, report, "invites[0].email", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteIdentityZeroUserIDOnly(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		UserID: optionalInt64(0),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0].user_id", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteEmailOnly(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Email: optionalString("not-an-email"),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0].email", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteUsernameOnly(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: optionalString("-octocat"),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0].username", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteUsernameWhitespaceOnly(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: optionalString("   "),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0].username", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteUsernameNullOnly(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: nullOptionalString(),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0].username", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteEmailNullOnly(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Email: nullOptionalString(),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0].email", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteEmailWhitespaceOnly(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Email: optionalString("   "),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0].email", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteUserIDNullOnly(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		UserID: nullOptionalInt64(),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0].user_id", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteIdentityMultipleAndWhitespaceUsernameReportsBoth(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: optionalString("   "),
		Email:    optionalString("octocat@example.com"),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0]", ValidationIssueCodeInvalidInviteIdentity)
	assertHasIssueAtPathAndCode(t, report, "invites[0].username", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateInvalidInviteIdentityMultipleAndNullUserIDReportsBoth(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0] = InviteSpec{
		Username: optionalString("octocat"),
		UserID:   nullOptionalInt64(),
	}

	report := Validate(cfg)

	assertHasIssueAtPathAndCode(t, report, "invites[0]", ValidationIssueCodeInvalidInviteIdentity)
	assertHasIssueAtPathAndCode(t, report, "invites[0].user_id", ValidationIssueCodeInvalidInviteIdentity)
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
		Name:       "octostate",
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
		Name:       "OCTOSTATE",
		Permission: "pull",
	})

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeDuplicateTeamRepository)
}

func TestValidateInvalidEnums(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Members[0].Role = "owner"
	cfg.Invites[0].Role = "owner"
	cfg.Repositories[0].Visibility = "internal"
	cfg.Teams[0].Privacy = "visible"
	cfg.Teams[0].Members[0].Role = "owner"
	cfg.Teams[0].Repositories[0].Permission = "write"

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeInvalidEnum)
	if report.Summary.Errors < 6 {
		t.Fatalf("expected multiple invalid enum errors, got %#v", report.Errors)
	}
}

func TestValidateTeamMembersMustExistInTopLevelMembers(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Members = []OrganizationMemberSpec{}

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "teams[0].members[0].username", ValidationIssueCodeUnknownOrganizationMember)
}

func TestValidateTeamMembersMatchTopLevelMembersCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Members[0].Username = "ALICE"

	report := Validate(cfg)
	assertNotHasIssueCode(t, report, ValidationIssueCodeUnknownOrganizationMember)
}

func TestValidateInviteUsernameMustNotOverlapTopLevelMembers(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites[0].Username = optionalString("alice")

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "invites[0].username", ValidationIssueCodeDuplicateOrganizationMemberInvite)
}

func TestValidateRepositoryOptionalFieldsOmittedAreValid(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Repositories[0] = RepositorySpec{
		Owner:      "orang-gaboets",
		Name:       "octostate",
		Template:   TemplateSpec{Owner: "orang-gaboets", Name: "repo-template"},
		Visibility: "private",
		Topics:     []string{"go", "gitops"},
	}

	report := Validate(cfg)
	if !report.Valid {
		t.Fatalf("expected omitted repository optional fields to be valid, got %#v", report)
	}
}

func TestValidateRepositoryOptionalStringsExplicitEmptyAreValid(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Repositories[0].Description = ""
	cfg.Repositories[0].description = optionalString("")
	cfg.Repositories[0].Homepage = ""
	cfg.Repositories[0].homepage = optionalString("")

	report := Validate(cfg)
	if !report.Valid {
		t.Fatalf("expected explicit empty repository optional strings to be valid, got %#v", report)
	}
}

func TestValidateRepositoryOptionalBooleansExplicitFalseAreValid(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Repositories[0].AllowForking = false
	cfg.Repositories[0].allowForking = optionalBool(false)
	cfg.Repositories[0].Archived = false
	cfg.Repositories[0].archived = optionalBool(false)
	cfg.Repositories[0].IsTemplate = false
	cfg.Repositories[0].isTemplate = optionalBool(false)

	report := Validate(cfg)
	if !report.Valid {
		t.Fatalf("expected explicit false repository optional booleans to be valid, got %#v", report)
	}
}

func TestValidateRepositoryOptionalNullFieldsAreInvalid(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Repositories[0].description = nullOptionalString()
	cfg.Repositories[0].Description = ""
	cfg.Repositories[0].homepage = nullOptionalString()
	cfg.Repositories[0].Homepage = ""
	cfg.Repositories[0].allowForking = nullOptionalBool()
	cfg.Repositories[0].AllowForking = false
	cfg.Repositories[0].archived = nullOptionalBool()
	cfg.Repositories[0].Archived = false
	cfg.Repositories[0].isTemplate = nullOptionalBool()
	cfg.Repositories[0].IsTemplate = false

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "repositories[0].description", ValidationIssueCodeInvalidFieldValue)
	assertHasIssueAtPathAndCode(t, report, "repositories[0].homepage", ValidationIssueCodeInvalidFieldValue)
	assertHasIssueAtPathAndCode(t, report, "repositories[0].allow_forking", ValidationIssueCodeInvalidFieldValue)
	assertHasIssueAtPathAndCode(t, report, "repositories[0].archived", ValidationIssueCodeInvalidFieldValue)
	assertHasIssueAtPathAndCode(t, report, "repositories[0].is_template", ValidationIssueCodeInvalidFieldValue)
}

func TestValidateSlugNameMismatch(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Teams[0].Slug = "wrong-slug"

	report := Validate(cfg)
	assertHasIssueCode(t, report, ValidationIssueCodeSlugNameMismatch)
}

func TestValidateDuplicateInviteUsernamesCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites = append(cfg.Invites, InviteSpec{
		Username: optionalString("OctoCat"),
		Role:     "direct_member",
	})

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "invites[1].username", ValidationIssueCodeDuplicateInvite)
}

func TestValidateDuplicateInviteEmailsCaseInsensitive(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites = []InviteSpec{
		{Email: optionalString("dev@example.com"), Role: "direct_member"},
		{Email: optionalString("DEV@example.com"), Role: "direct_member"},
	}

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "invites[1].email", ValidationIssueCodeDuplicateInvite)
}

func TestValidateDuplicateInviteUserIDs(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites = []InviteSpec{
		{UserID: optionalInt64(42), Role: "direct_member"},
		{UserID: optionalInt64(42), Role: "direct_member"},
	}

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "invites[1].user_id", ValidationIssueCodeDuplicateInvite)
}

func TestValidateDistinctInviteIdentitiesAreValid(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Invites = []InviteSpec{
		{Username: optionalString("octocat"), Role: "direct_member"},
		{Email: optionalString("octocat@example.com"), Role: "direct_member"},
		{UserID: optionalInt64(42), Role: "direct_member"},
	}

	report := Validate(cfg)
	if !report.Valid {
		t.Fatalf("expected distinct invite identities to be valid, got %#v", report)
	}
}

func validOrganizationConfig() OrganizationConfig {
	repo := RepositorySpec{
		Owner:       "orang-gaboets",
		Name:        "octostate",
		Template:    TemplateSpec{Owner: "orang-gaboets", Name: "repo-template"},
		Visibility:  "private",
		Description: "GitHub organization operations CLI",
		Homepage:    "https://github.com/orang-gaboets/octostate",
		Topics:      []string{"go", "gitops"},
	}
	repo.description = optionalString("GitHub organization operations CLI")
	repo.homepage = optionalString("https://github.com/orang-gaboets/octostate")
	repo.allowForking = optionalBool(false)
	repo.archived = optionalBool(false)
	repo.isTemplate = optionalBool(false)

	return OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []OrganizationMemberSpec{{
			Username: "alice",
			Role:     "member",
		}},
		Invites: []InviteSpec{{
			Username:  optionalString("octocat"),
			Role:      "direct_member",
			TeamSlugs: []string{"platform"},
		}},
		Repositories: []RepositorySpec{repo},
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
				Name:       "octostate",
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

func optionalString(v string) OptionalString {
	return OptionalString{
		Present: true,
		Value:   v,
	}
}

func nullOptionalString() OptionalString {
	return OptionalString{
		Present: true,
		Null:    true,
	}
}

func optionalInt64(v int64) OptionalInt64 {
	return OptionalInt64{
		Present: true,
		Value:   v,
	}
}

func nullOptionalInt64() OptionalInt64 {
	return OptionalInt64{
		Present: true,
		Null:    true,
	}
}

func optionalBool(v bool) OptionalBool {
	return OptionalBool{
		Present: true,
		Value:   v,
	}
}

func nullOptionalBool() OptionalBool {
	return OptionalBool{
		Present: true,
		Null:    true,
	}
}
