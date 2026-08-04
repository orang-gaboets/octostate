package configproposal

import (
	"testing"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

func TestFindTeamIndex(t *testing.T) {
	cfg := &gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams: []gitopsconfig.TeamSpec{
			{Slug: "platform", Name: "Platform"},
			{Slug: "core-infra", Name: "Core Infra"},
		},
	}

	tests := []struct {
		name      string
		slug      string
		wantIndex int
		wantFound bool
	}{
		{name: "exact", slug: "platform", wantIndex: 0, wantFound: true},
		{name: "case insensitive", slug: "PLATFORM", wantIndex: 0, wantFound: true},
		{name: "trims values", slug: " core-infra ", wantIndex: 1, wantFound: true},
		{name: "missing", slug: "missing", wantIndex: -1, wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, gotFound := FindTeamIndex(cfg, tt.slug)
			if gotIndex != tt.wantIndex || gotFound != tt.wantFound {
				t.Fatalf("FindTeamIndex() = (%d, %t), want (%d, %t)", gotIndex, gotFound, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestFindTeamIndexNilConfig(t *testing.T) {
	if index, found := FindTeamIndex(nil, "platform"); index != -1 || found {
		t.Fatalf("FindTeamIndex(nil) = (%d, %t), want (-1, false)", index, found)
	}
}
