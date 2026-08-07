package configproposal

import (
	"testing"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

func inviteConfig() *gitopsconfig.OrganizationConfig {
	return &gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []gitopsconfig.InviteSpec{
			{Username: gitopsconfig.OptionalString{Present: true, Value: "octocat"}, Role: "direct_member"},
			{Email: gitopsconfig.OptionalString{Present: true, Value: "someone@example.com"}, Role: "direct_member"},
			{UserID: gitopsconfig.OptionalInt64{Present: true, Value: 42}, Role: "admin"},
			{Username: gitopsconfig.OptionalString{Present: true, Null: true}, Role: "direct_member"},
		},
	}
}

func TestFindInviteIndexByUsername(t *testing.T) {
	cfg := inviteConfig()

	tests := []struct {
		name      string
		username  string
		wantIndex int
		wantFound bool
	}{
		{name: "exact", username: "octocat", wantIndex: 0, wantFound: true},
		{name: "case insensitive", username: "OCTOCAT", wantIndex: 0, wantFound: true},
		{name: "trims values", username: " octocat ", wantIndex: 0, wantFound: true},
		{name: "missing", username: "hubber", wantIndex: -1, wantFound: false},
		{name: "does not match email invite", username: "someone@example.com", wantIndex: -1, wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, gotFound := FindInviteIndexByUsername(cfg, tt.username)
			if gotIndex != tt.wantIndex || gotFound != tt.wantFound {
				t.Fatalf("FindInviteIndexByUsername() = (%d, %t), want (%d, %t)", gotIndex, gotFound, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestFindInviteIndexByUsernameSkipsNullIdentities(t *testing.T) {
	cfg := inviteConfig()
	if index, found := FindInviteIndexByUsername(cfg, ""); found {
		t.Fatalf("expected null username invite to be skipped, got index %d", index)
	}
}

func TestFindInviteIndexByUserID(t *testing.T) {
	cfg := inviteConfig()

	if index, found := FindInviteIndexByUserID(cfg, 42); index != 2 || !found {
		t.Fatalf("FindInviteIndexByUserID(42) = (%d, %t), want (2, true)", index, found)
	}
	if index, found := FindInviteIndexByUserID(cfg, 99); index != -1 || found {
		t.Fatalf("FindInviteIndexByUserID(99) = (%d, %t), want (-1, false)", index, found)
	}
}

func TestFindInviteIndexNilConfig(t *testing.T) {
	if index, found := FindInviteIndexByUsername(nil, "octocat"); index != -1 || found {
		t.Fatalf("FindInviteIndexByUsername(nil) = (%d, %t), want (-1, false)", index, found)
	}
	if index, found := FindInviteIndexByUserID(nil, 42); index != -1 || found {
		t.Fatalf("FindInviteIndexByUserID(nil) = (%d, %t), want (-1, false)", index, found)
	}
}
