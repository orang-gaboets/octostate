package config

import "testing"

// Invite identity validation must read the same effective value load-time
// normalization produces, so a padded identity that LoadFile accepts is not
// rejected when the same config is built in memory.
func TestValidateAcceptsWhitespacePaddedInviteIdentities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		invite InviteSpec
	}{
		{"username", InviteSpec{Username: optionalString(" octocat "), Role: "direct_member"}},
		{"email", InviteSpec{Email: optionalString(" dev@example.com "), Role: "direct_member"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := Validate(inviteConfigWith(tt.invite))
			if !report.Valid {
				t.Fatalf("padded invite %s must validate as its trimmed form: %#v", tt.name, report.Errors)
			}
		})
	}
}

func TestValidateRejectsWhitespaceOnlyInviteIdentities(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		invite InviteSpec
		path   string
	}{
		{"username", InviteSpec{Username: optionalString("   "), Role: "direct_member"}, "invites[0].username"},
		{"email", InviteSpec{Email: optionalString("   "), Role: "direct_member"}, "invites[0].email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := Validate(inviteConfigWith(tt.invite))
			assertHasIssueAtPathAndCode(t, report, tt.path, ValidationIssueCodeInvalidInviteIdentity)
		})
	}
}

func TestValidateRejectsPaddedInviteIdentityThatIsStillMalformed(t *testing.T) {
	t.Parallel()

	report := Validate(inviteConfigWith(InviteSpec{Username: optionalString(" not a username "), Role: "direct_member"}))
	assertHasIssueAtPathAndCode(t, report, "invites[0].username", ValidationIssueCodeInvalidInviteIdentity)
}

func TestValidateDetectsDuplicatesAcrossWhitespacePaddedInviteIdentities(t *testing.T) {
	t.Parallel()

	report := Validate(inviteConfigWith(
		InviteSpec{Username: optionalString("octocat"), Role: "direct_member"},
		InviteSpec{Username: optionalString(" OCTOCAT "), Role: "direct_member"},
	))
	assertHasIssueAtPathAndCode(t, report, "invites[1].username", ValidationIssueCodeDuplicateInvite)
}

func TestValidateDetectsPaddedInviteOverlappingTopLevelMember(t *testing.T) {
	t.Parallel()

	// alice is already declared as a top-level member by the fixture.
	report := Validate(inviteConfigWith(InviteSpec{Username: optionalString(" alice "), Role: "direct_member"}))
	assertHasIssueAtPathAndCode(t, report, "invites[0].username", ValidationIssueCodeDuplicateOrganizationMemberInvite)
}

func TestValidateDoesNotMutateInviteIdentities(t *testing.T) {
	t.Parallel()

	cfg := inviteConfigWith(InviteSpec{Username: optionalString(" octocat "), Role: "direct_member"})
	_ = Validate(cfg)
	if got := cfg.Invites[0].Username.Value; got != " octocat " {
		t.Fatalf("Validate mutated the caller's invite username: %q", got)
	}
}
