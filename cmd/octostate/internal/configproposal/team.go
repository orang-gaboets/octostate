package configproposal

import (
	"strings"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// FindTeamIndex returns the index of a team matching slug.
// Team identity is case-insensitive after trimming the value.
func FindTeamIndex(cfg *gitopsconfig.OrganizationConfig, slug string) (int, bool) {
	if cfg == nil {
		return -1, false
	}

	wantSlug := strings.TrimSpace(slug)
	for index, team := range cfg.Teams {
		if strings.EqualFold(strings.TrimSpace(team.Slug), wantSlug) {
			return index, true
		}
	}

	return -1, false
}
