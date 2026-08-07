package configproposal

import (
	"testing"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

func TestFindTeamRepositoryIndex(t *testing.T) {
	team := &gitopsconfig.TeamSpec{
		Slug: "platform",
		Repositories: []gitopsconfig.TeamRepositorySpec{
			{Name: "api", Permission: "push"},
			{Owner: "other-org", Name: "web", Permission: "pull"},
		},
	}

	tests := []struct {
		name      string
		owner     string
		repo      string
		wantIndex int
		wantFound bool
	}{
		{name: "implicit owner both sides", owner: "", repo: "api", wantIndex: 0, wantFound: true},
		{name: "explicit owner matches organization", owner: "orang-gaboets", repo: "api", wantIndex: 0, wantFound: true},
		{name: "case insensitive", owner: "OTHER-ORG", repo: "WEB", wantIndex: 1, wantFound: true},
		{name: "trims values", owner: " other-org ", repo: " web ", wantIndex: 1, wantFound: true},
		{name: "different owner does not match", owner: "another-org", repo: "api", wantIndex: -1, wantFound: false},
		{name: "missing", owner: "", repo: "missing", wantIndex: -1, wantFound: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, gotFound := FindTeamRepositoryIndex(team, "orang-gaboets", tt.owner, tt.repo)
			if gotIndex != tt.wantIndex || gotFound != tt.wantFound {
				t.Fatalf("FindTeamRepositoryIndex() = (%d, %t), want (%d, %t)", gotIndex, gotFound, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestFindTeamRepositoryIndexNilTeam(t *testing.T) {
	if index, found := FindTeamRepositoryIndex(nil, "orang-gaboets", "", "api"); index != -1 || found {
		t.Fatalf("FindTeamRepositoryIndex(nil) = (%d, %t), want (-1, false)", index, found)
	}
}
