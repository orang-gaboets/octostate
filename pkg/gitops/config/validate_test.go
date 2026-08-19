package config

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

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

func TestRepositoryOwnerMatchesOrganization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		owner        string
		organization string
		want         bool
	}{
		{name: "exact", owner: "orang-gaboets", organization: "orang-gaboets", want: true},
		{name: "trimmed and case insensitive", owner: " ORANG-GABOETS ", organization: " orang-gaboets ", want: true},
		{name: "blank owner", owner: " ", organization: "orang-gaboets", want: false},
		{name: "blank organization", owner: "orang-gaboets", organization: " ", want: false},
		{name: "different owner", owner: "shared-platform", organization: "orang-gaboets", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := RepositoryOwnerMatchesOrganization(tt.owner, tt.organization); got != tt.want {
				t.Fatalf("RepositoryOwnerMatchesOrganization(%q, %q) = %t, want %t", tt.owner, tt.organization, got, tt.want)
			}
		})
	}
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

func TestValidateRepositoryOwnerScope(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		cfg       OrganizationConfig
		wantValid bool
		wantPaths []string
	}{
		{
			name: "omitted owners default to organization",
			cfg: OrganizationConfig{
				Organization: " Orang-Gaboets ",
				Repositories: []RepositorySpec{{
					Name:       "octostate",
					Visibility: "private",
				}},
				Teams: []TeamSpec{{
					Slug:    "platform",
					Name:    "Platform",
					Privacy: "closed",
					Repositories: []TeamRepositorySpec{{
						Name:       "octostate-infra",
						Permission: "push",
					}},
				}},
			},
			wantValid: true,
		},
		{
			name: "same organization owners with whitespace and case are valid",
			cfg: OrganizationConfig{
				Organization: " orang-gaboets ",
				Repositories: []RepositorySpec{{
					Owner:      " ORANG-GABOETS ",
					Name:       "octostate",
					Visibility: "private",
				}},
				Teams: []TeamSpec{{
					Slug:    "platform",
					Name:    "Platform",
					Privacy: "closed",
					Repositories: []TeamRepositorySpec{{
						Owner:      " Orang-Gaboets ",
						Name:       "octostate-infra",
						Permission: "push",
					}},
				}},
			},
			wantValid: true,
		},
		{
			name: "top level repository owner outside organization is invalid",
			cfg: OrganizationConfig{
				Organization: "orang-gaboets",
				Repositories: []RepositorySpec{{
					Owner:      "shared-platform",
					Name:       "octostate",
					Visibility: "private",
				}},
			},
			wantPaths: []string{"repositories[0].owner"},
		},
		{
			name: "team repository owner outside organization is invalid",
			cfg: OrganizationConfig{
				Organization: "orang-gaboets",
				Teams: []TeamSpec{{
					Slug:    "platform",
					Name:    "Platform",
					Privacy: "closed",
					Repositories: []TeamRepositorySpec{{
						Owner:      "shared-platform",
						Name:       "octostate",
						Permission: "push",
					}},
				}},
			},
			wantPaths: []string{"teams[0].repositories[0].owner"},
		},
		{
			name: "multiple mismatches are deterministic",
			cfg: OrganizationConfig{
				Organization: "orang-gaboets",
				Repositories: []RepositorySpec{{
					Owner:      "shared-platform",
					Name:       "octostate",
					Visibility: "private",
				}},
				Teams: []TeamSpec{{
					Slug:    "platform",
					Name:    "Platform",
					Privacy: "closed",
					Repositories: []TeamRepositorySpec{{
						Owner:      "other-org",
						Name:       "octostate-infra",
						Permission: "push",
					}},
				}},
			},
			wantPaths: []string{
				"repositories[0].owner",
				"teams[0].repositories[0].owner",
			},
		},
		{
			name: "blank organization suppresses owner scope issues",
			cfg: OrganizationConfig{
				Organization: " ",
				Repositories: []RepositorySpec{{
					Owner:      "shared-platform",
					Name:       "octostate",
					Visibility: "private",
				}},
				Teams: []TeamSpec{{
					Slug:    "platform",
					Name:    "Platform",
					Privacy: "closed",
					Repositories: []TeamRepositorySpec{{
						Owner:      "other-org",
						Name:       "octostate-infra",
						Permission: "push",
					}},
				}},
			},
			wantPaths: []string{"organization"},
		},
		{
			name: "external template owner is exempt",
			cfg: OrganizationConfig{
				Organization: "orang-gaboets",
				Repositories: []RepositorySpec{{
					Name:       "octostate",
					Visibility: "private",
					Template: TemplateSpec{
						Owner: "shared-platform",
						Name:  "repo-template",
					},
				}},
			},
			wantValid: true,
		},
		{
			name: "same organization team repository need not exist top level",
			cfg: OrganizationConfig{
				Organization: "orang-gaboets",
				Teams: []TeamSpec{{
					Slug:    "platform",
					Name:    "Platform",
					Privacy: "closed",
					Repositories: []TeamRepositorySpec{{
						Owner:      "orang-gaboets",
						Name:       "octostate-infra",
						Permission: "push",
					}},
				}},
			},
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := Validate(tt.cfg)
			if report.Valid != tt.wantValid {
				t.Fatalf("Validate valid = %t, want %t; errors: %#v", report.Valid, tt.wantValid, report.Errors)
			}

			if len(tt.wantPaths) == 0 {
				assertNotHasIssueCode(t, report, ValidationIssueCodeRepositoryOwnerScope)
				return
			}

			gotPaths := make([]string, 0, len(report.Errors))
			for _, issue := range report.Errors {
				if issue.Code == ValidationIssueCodeRepositoryOwnerScope || issue.Path == "organization" {
					gotPaths = append(gotPaths, issue.Path)
				}
			}
			if !slices.Equal(gotPaths, tt.wantPaths) {
				t.Fatalf("owner-scope paths = %#v, want %#v; full errors: %#v", gotPaths, tt.wantPaths, report.Errors)
			}
		})
	}
}

func TestValidateAndErrorFormatsReportInOrder(t *testing.T) {
	t.Parallel()

	cfg := OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []RepositorySpec{{
			Owner:      "shared-platform",
			Name:       "octostate",
			Visibility: "private",
		}},
		Teams: []TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Repositories: []TeamRepositorySpec{{
				Owner:      "other-org",
				Name:       "octostate-infra",
				Permission: "push",
			}},
		}},
	}

	err := ValidateAndError(cfg)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *ValidationError, got %T", err)
	}

	want := strings.Join([]string{
		"organization config validation failed:",
		"- repositories[0].owner: repository_owner_scope: repository owner \"shared-platform\" must match organization \"orang-gaboets\"",
		"- teams[0].repositories[0].owner: repository_owner_scope: repository owner \"other-org\" must match organization \"orang-gaboets\"",
	}, "\n")
	if err.Error() != want {
		t.Fatalf("ValidationError.Error() = %q, want %q", err.Error(), want)
	}
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

func TestValidateRepositoryTopicsFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		topic    string
		wantCode ValidationIssueCode
	}{
		{name: "letters", topic: "go"},
		{name: "hyphen", topic: "go-lang"},
		{name: "digits", topic: "abc123"},
		{name: "leading and trailing hyphen", topic: "-go-"},
		{name: "surrounding whitespace", topic: " go "},
		{name: "whitespace only", topic: "   ", wantCode: ValidationIssueCodeMissingRequiredField},
		{name: "empty", topic: "", wantCode: ValidationIssueCodeMissingRequiredField},
		{name: "embedded whitespace", topic: "go lang", wantCode: ValidationIssueCodeInvalidRepositoryTopic},
		{name: "uppercase", topic: "Go", wantCode: ValidationIssueCodeInvalidRepositoryTopic},
		{name: "underscore", topic: "go_lang", wantCode: ValidationIssueCodeInvalidRepositoryTopic},
		{name: "slash", topic: "go/lang", wantCode: ValidationIssueCodeInvalidRepositoryTopic},
		{name: "non ASCII under 50 characters", topic: strings.Repeat("é", 26), wantCode: ValidationIssueCodeInvalidRepositoryTopic},
		{name: "50 characters", topic: strings.Repeat("a", 50)},
		{name: "51 characters", topic: strings.Repeat("a", 51), wantCode: ValidationIssueCodeInvalidRepositoryTopic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := validOrganizationConfig()
			cfg.Repositories[0].Topics = []string{tt.topic}
			originalTopics := append([]string(nil), cfg.Repositories[0].Topics...)

			report := Validate(cfg)
			if tt.wantCode == "" {
				if !report.Valid || len(report.Errors) != 0 {
					t.Fatalf("expected valid report, got %#v", report.Errors)
				}
			} else {
				assertHasIssueAtPathAndCode(t, report, "repositories[0].topics[0]", tt.wantCode)
			}
			if !slices.Equal(cfg.Repositories[0].Topics, originalTopics) {
				t.Fatalf("validation mutated topics: got %#v, want %#v", cfg.Repositories[0].Topics, originalTopics)
			}
		})
	}
}

func TestValidateRepositoryTopicsOmittedAndEmptyAreValid(t *testing.T) {
	t.Parallel()

	for _, topics := range [][]string{nil, {}} {
		cfg := validOrganizationConfig()
		cfg.Repositories[0].Topics = topics

		report := Validate(cfg)
		if !report.Valid {
			t.Fatalf("expected topics %#v to be valid, got %#v", topics, report.Errors)
		}
	}
}

func TestValidateRepositoryTopicsLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		topics    []string
		wantValid bool
		wantPath  string
		wantCode  ValidationIssueCode
	}{
		{name: "20 distinct", topics: repositoryTopics(20), wantValid: true},
		{name: "21 distinct", topics: repositoryTopics(21), wantPath: "repositories[0].topics", wantCode: ValidationIssueCodeRepositoryTopicLimit},
		{name: "20 distinct plus duplicate", topics: append(repositoryTopics(20), " topic-a "), wantValid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := validOrganizationConfig()
			cfg.Repositories[0].Topics = tt.topics

			report := Validate(cfg)
			if report.Valid != tt.wantValid {
				t.Fatalf("Validate valid = %t, want %t; errors: %#v", report.Valid, tt.wantValid, report.Errors)
			}
			if tt.wantCode != "" {
				assertHasIssueAtPathAndCode(t, report, tt.wantPath, tt.wantCode)
			}
		})
	}
}

func TestValidateRepositoryTopicsLimitCountsInvalidTopics(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Repositories[0].Topics = append(repositoryTopics(20), "Go")

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "repositories[0].topics[20]", ValidationIssueCodeInvalidRepositoryTopic)
	assertHasIssueAtPathAndCode(t, report, "repositories[0].topics", ValidationIssueCodeRepositoryTopicLimit)
}

func repositoryTopics(count int) []string {
	topics := make([]string, count)
	for i := range topics {
		topics[i] = "topic-" + strings.Repeat("a", i)
	}
	return topics
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

func TestValidateRepositoryOptionalNullFields(t *testing.T) {
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
	for _, issue := range report.Errors {
		if issue.Path == "repositories[0].is_template" {
			t.Fatalf("expected repository is_template null to be accepted, got %#v", issue)
		}
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

func TestNormalizeTeamNameExported(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
	}{
		{name: " Platform Team ", want: "platform-team"},
		{name: "Core_Infra  Ops", want: "core-infra-ops"},
		{name: "***", want: ""},
	}
	for _, tt := range tests {
		if got := NormalizeTeamName(tt.name); got != tt.want {
			t.Fatalf("NormalizeTeamName(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestNormalizeDesiredStateResolvesRepositoryIdentitiesWithoutMutating(t *testing.T) {
	original := OrganizationConfig{
		Organization: " org-a ",
		Repositories: []RepositorySpec{{Owner: "", Name: " service ", Template: TemplateSpec{Owner: " external ", Name: " base "}}},
		Teams:        []TeamSpec{{Repositories: []TeamRepositorySpec{{Owner: " ORG-A ", Name: " service ", Permission: "push"}}}},
	}

	normalized := NormalizeDesiredState(original)
	if normalized.Organization != "org-a" {
		t.Fatalf("normalized organization = %q, want %q", normalized.Organization, "org-a")
	}
	if normalized.Repositories[0].Owner != "org-a" {
		t.Fatalf("normalized repository owner = %q, want %q", normalized.Repositories[0].Owner, "org-a")
	}
	if normalized.Repositories[0].Name != "service" || normalized.Repositories[0].Template.Owner != "external" || normalized.Repositories[0].Template.Name != "base" {
		t.Fatalf("normalized repository identity = %#v", normalized.Repositories[0])
	}
	if normalized.Teams[0].Repositories[0].Owner != "org-a" {
		t.Fatalf("normalized team repository owner = %q, want %q", normalized.Teams[0].Repositories[0].Owner, "org-a")
	}
	if normalized.Teams[0].Repositories[0].Name != "service" {
		t.Fatalf("normalized team repository name = %q, want %q", normalized.Teams[0].Repositories[0].Name, "service")
	}

	if original.Organization != " org-a " || original.Repositories[0].Owner != "" || original.Repositories[0].Name != " service " || original.Repositories[0].Template.Owner != " external " || original.Teams[0].Repositories[0].Owner != " ORG-A " || original.Teams[0].Repositories[0].Name != " service " {
		t.Fatalf("normalization mutated the original config: %#v", original)
	}
}

func inviteConfigWith(invites ...InviteSpec) OrganizationConfig {
	cfg := validOrganizationConfig()
	cfg.Invites = invites
	return cfg
}

func TestValidateDuplicateInviteUsernames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		invites []InviteSpec
	}{
		{
			name: "identical",
			invites: []InviteSpec{
				{Username: optionalString("octocat"), Role: "direct_member"},
				{Username: optionalString("octocat"), Role: "direct_member"},
			},
		},
		{
			name: "case insensitive",
			invites: []InviteSpec{
				{Username: optionalString("octocat"), Role: "direct_member"},
				{Username: optionalString("Octocat"), Role: "admin"},
			},
		},
		{
			name: "surrounding whitespace",
			invites: []InviteSpec{
				{Username: optionalString("octocat"), Role: "direct_member"},
				{Username: optionalString(" octocat "), Role: "direct_member"},
			},
		},
		{
			name: "conflicting team slugs",
			invites: []InviteSpec{
				{Username: optionalString("octocat"), Role: "direct_member", TeamSlugs: []string{"platform"}},
				{Username: optionalString("octocat"), Role: "direct_member"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := Validate(inviteConfigWith(tt.invites...))
			assertHasIssueAtPathAndCode(t, report, "invites[1].username", ValidationIssueCodeDuplicateInvite)
		})
	}
}

func TestValidateDuplicateInviteEmailsAreCaseInsensitive(t *testing.T) {
	t.Parallel()

	report := Validate(inviteConfigWith(
		InviteSpec{Email: optionalString("dev@example.com"), Role: "direct_member"},
		InviteSpec{Email: optionalString("DEV@Example.COM"), Role: "admin"},
	))
	assertHasIssueAtPathAndCode(t, report, "invites[1].email", ValidationIssueCodeDuplicateInvite)
}

func TestValidateDuplicateInviteUserIDs(t *testing.T) {
	t.Parallel()

	report := Validate(inviteConfigWith(
		InviteSpec{UserID: optionalInt64(42), Role: "direct_member"},
		InviteSpec{UserID: optionalInt64(42), Role: "billing_manager"},
	))
	assertHasIssueAtPathAndCode(t, report, "invites[1].user_id", ValidationIssueCodeDuplicateInvite)
}

func TestValidateDistinctInvitesRemainValid(t *testing.T) {
	t.Parallel()

	report := Validate(inviteConfigWith(
		InviteSpec{Username: optionalString("octocat"), Role: "direct_member"},
		InviteSpec{Username: optionalString("hubber"), Role: "direct_member"},
		InviteSpec{Email: optionalString("dev@example.com"), Role: "direct_member"},
		InviteSpec{Email: optionalString("ops@example.com"), Role: "direct_member"},
		InviteSpec{UserID: optionalInt64(1), Role: "direct_member"},
		InviteSpec{UserID: optionalInt64(2), Role: "direct_member"},
	))
	if !report.Valid {
		t.Fatalf("expected distinct invites to be valid, got %#v", report.Errors)
	}
}

func TestValidateInviteIdentityKindsAreIndependent(t *testing.T) {
	t.Parallel()

	// A username, an email, and a user_id that a human might consider the same
	// person must not be treated as duplicates: resolving that would require a
	// live identity lookup the offline validator must not perform.
	report := Validate(inviteConfigWith(
		InviteSpec{Username: optionalString("octocat"), Role: "direct_member"},
		InviteSpec{Email: optionalString("octocat@example.com"), Role: "direct_member"},
		InviteSpec{UserID: optionalInt64(42), Role: "direct_member"},
	))
	if !report.Valid {
		t.Fatalf("expected different identity kinds to be independent, got %#v", report.Errors)
	}
}

func TestValidateDuplicateInvitesReferenceFirstDeclaration(t *testing.T) {
	t.Parallel()

	report := Validate(inviteConfigWith(
		InviteSpec{Username: optionalString("octocat"), Role: "direct_member"},
		InviteSpec{Username: optionalString("OCTOCAT"), Role: "direct_member"},
		InviteSpec{Username: optionalString("octocat"), Role: "admin"},
	))

	var messages []string
	for _, issue := range report.Errors {
		if issue.Code == ValidationIssueCodeDuplicateInvite {
			messages = append(messages, issue.Path+": "+issue.Message)
		}
	}
	if len(messages) != 2 {
		t.Fatalf("expected two duplicate errors, got %#v", messages)
	}
	for _, message := range messages {
		if !strings.Contains(message, "duplicates invites[0]") {
			t.Fatalf("expected every duplicate to reference the first declaration, got %q", message)
		}
	}
	if !strings.HasPrefix(messages[0], "invites[1].username") || !strings.HasPrefix(messages[1], "invites[2].username") {
		t.Fatalf("expected deterministic ordering by invite index, got %#v", messages)
	}
}

func TestValidateDuplicateInviteDoesNotMaskMemberConflict(t *testing.T) {
	t.Parallel()

	cfg := validOrganizationConfig()
	cfg.Members = []OrganizationMemberSpec{{Username: "alice", Role: "member"}}
	cfg.Invites = []InviteSpec{
		{Username: optionalString("alice"), Role: "direct_member"},
		{Username: optionalString("alice"), Role: "direct_member"},
	}
	cfg.Teams = nil

	report := Validate(cfg)
	assertHasIssueAtPathAndCode(t, report, "invites[0].username", ValidationIssueCodeDuplicateOrganizationMemberInvite)
	assertHasIssueAtPathAndCode(t, report, "invites[1].username", ValidationIssueCodeDuplicateInvite)
}

func TestValidateInvalidInviteIdentityIsNotReportedAsDuplicate(t *testing.T) {
	t.Parallel()

	report := Validate(inviteConfigWith(
		InviteSpec{Role: "direct_member"},
		InviteSpec{Role: "direct_member"},
	))
	for _, issue := range report.Errors {
		if issue.Code == ValidationIssueCodeDuplicateInvite {
			t.Fatalf("invites without an identity must not be reported as duplicates: %#v", issue)
		}
	}
	assertHasIssueAtPathAndCode(t, report, "invites[0]", ValidationIssueCodeInvalidInviteIdentity)
	assertHasIssueAtPathAndCode(t, report, "invites[1]", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateMalformedMultiIdentityInviteIsExcludedFromDuplicateDetection(t *testing.T) {
	t.Parallel()

	malformedUsernameAndEmail := InviteSpec{
		Username: optionalString("octocat"),
		Email:    optionalString("dup@example.com"),
		Role:     "direct_member",
	}
	validUsername := InviteSpec{Username: optionalString("octocat"), Role: "direct_member"}
	validEmail := InviteSpec{Email: optionalString("dup@example.com"), Role: "direct_member"}

	tests := []struct {
		name    string
		invites []InviteSpec
	}{
		{name: "malformed before username", invites: []InviteSpec{malformedUsernameAndEmail, validUsername}},
		{name: "malformed after username", invites: []InviteSpec{validUsername, malformedUsernameAndEmail}},
		{name: "malformed before email", invites: []InviteSpec{malformedUsernameAndEmail, validEmail}},
		{name: "malformed after email", invites: []InviteSpec{validEmail, malformedUsernameAndEmail}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := Validate(inviteConfigWith(tt.invites...))

			// The malformed invite must be reported for its identity declaration
			// only. It must never seed or match the uniqueness index, in either
			// position and regardless of which identity field it happens to
			// declare first.
			assertHasIssueCode(t, report, ValidationIssueCodeInvalidInviteIdentity)
			for _, issue := range report.Errors {
				if issue.Code == ValidationIssueCodeDuplicateInvite {
					t.Fatalf("malformed multi-identity invite participated in duplicate detection: %#v", issue)
				}
			}
		})
	}
}

func TestValidateDuplicateDetectionSurvivesMalformedNeighbour(t *testing.T) {
	t.Parallel()

	// A malformed invite between two genuine duplicates must not shift which
	// declaration the duplicate is reported against.
	report := Validate(inviteConfigWith(
		InviteSpec{Username: optionalString("octocat"), Role: "direct_member"},
		InviteSpec{Username: optionalString("hubber"), Email: optionalString("x@example.com"), Role: "direct_member"},
		InviteSpec{Username: optionalString("octocat"), Role: "direct_member"},
	))
	assertHasIssueAtPathAndCode(t, report, "invites[2].username", ValidationIssueCodeDuplicateInvite)

	for _, issue := range report.Errors {
		if issue.Code == ValidationIssueCodeDuplicateInvite && !strings.Contains(issue.Message, "duplicates invites[0]") {
			t.Fatalf("expected duplicate to reference invites[0], got %q", issue.Message)
		}
	}
}
