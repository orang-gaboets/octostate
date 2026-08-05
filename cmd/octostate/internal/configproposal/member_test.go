package configproposal

import (
	"testing"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

func TestFindOrganizationMemberIndex(t *testing.T) {
	cfg := &gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []gitopsconfig.OrganizationMemberSpec{
			{Username: "alice", Role: "member"},
			{Username: "Bob", Role: "admin"},
		},
	}

	tests := []struct {
		name      string
		username  string
		wantIndex int
		wantFound bool
	}{
		{name: "exact", username: "alice", wantIndex: 0, wantFound: true},
		{name: "case insensitive", username: "BOB", wantIndex: 1, wantFound: true},
		{name: "trims values", username: " alice ", wantIndex: 0, wantFound: true},
		{name: "missing", username: "carol", wantIndex: -1, wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, gotFound := FindOrganizationMemberIndex(cfg, tt.username)
			if gotIndex != tt.wantIndex || gotFound != tt.wantFound {
				t.Fatalf("FindOrganizationMemberIndex() = (%d, %t), want (%d, %t)", gotIndex, gotFound, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestFindOrganizationMemberIndexNilConfig(t *testing.T) {
	if index, found := FindOrganizationMemberIndex(nil, "alice"); index != -1 || found {
		t.Fatalf("FindOrganizationMemberIndex(nil) = (%d, %t), want (-1, false)", index, found)
	}
}

func TestFindTeamMemberIndex(t *testing.T) {
	team := &gitopsconfig.TeamSpec{
		Slug: "platform",
		Members: []gitopsconfig.TeamMemberSpec{
			{Username: "alice", Role: "member"},
			{Username: "Bob", Role: "maintainer"},
		},
	}

	tests := []struct {
		name      string
		username  string
		wantIndex int
		wantFound bool
	}{
		{name: "exact", username: "alice", wantIndex: 0, wantFound: true},
		{name: "case insensitive", username: "BOB", wantIndex: 1, wantFound: true},
		{name: "trims values", username: " Bob ", wantIndex: 1, wantFound: true},
		{name: "missing", username: "carol", wantIndex: -1, wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, gotFound := FindTeamMemberIndex(team, tt.username)
			if gotIndex != tt.wantIndex || gotFound != tt.wantFound {
				t.Fatalf("FindTeamMemberIndex() = (%d, %t), want (%d, %t)", gotIndex, gotFound, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestFindTeamMemberIndexNilTeam(t *testing.T) {
	if index, found := FindTeamMemberIndex(nil, "alice"); index != -1 || found {
		t.Fatalf("FindTeamMemberIndex(nil) = (%d, %t), want (-1, false)", index, found)
	}
}
