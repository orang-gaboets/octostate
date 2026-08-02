package configproposal

import (
	"testing"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

func TestFindRepositoryIndex(t *testing.T) {
	cfg := &gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []gitopsconfig.RepositorySpec{
			{Owner: "orang-gaboets", Name: "octostate"},
			{Name: "implicit-owner"},
			{Owner: "other-org", Name: "other-repo"},
		},
	}

	tests := []struct {
		name      string
		owner     string
		repo      string
		wantIndex int
		wantFound bool
	}{
		{name: "case insensitive", owner: "ORANG-GABOETS", repo: "OCTOSTATE", wantIndex: 0, wantFound: true},
		{name: "trims values", owner: " orang-gaboets ", repo: " octostate ", wantIndex: 0, wantFound: true},
		{name: "missing", owner: "orang-gaboets", repo: "missing", wantIndex: -1, wantFound: false},
		{name: "implicit owner", owner: "orang-gaboets", repo: "implicit-owner", wantIndex: 1, wantFound: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotIndex, gotFound := FindRepositoryIndex(cfg, tt.owner, tt.repo)
			if gotIndex != tt.wantIndex || gotFound != tt.wantFound {
				t.Fatalf("FindRepositoryIndex() = (%d, %t), want (%d, %t)", gotIndex, gotFound, tt.wantIndex, tt.wantFound)
			}
		})
	}
}

func TestFindRepositoryIndexNilConfig(t *testing.T) {
	if index, found := FindRepositoryIndex(nil, "org", "repo"); index != -1 || found {
		t.Fatalf("FindRepositoryIndex(nil) = (%d, %t), want (-1, false)", index, found)
	}
}

func TestFindRepositoryIndexUsesConfigOrganizationForEmptyOwner(t *testing.T) {
	cfg := &gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []gitopsconfig.RepositorySpec{{Name: "octostate"}},
	}
	if index, found := FindRepositoryIndex(cfg, "", "octostate"); index != 0 || !found {
		t.Fatalf("FindRepositoryIndex() = (%d, %t), want (0, true)", index, found)
	}
}
