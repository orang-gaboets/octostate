package configproposal

import (
	"strings"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

// FindOrganizationMemberIndex returns the index of a top-level organization
// member matching username. Member identity is case-insensitive after trimming
// the value.
func FindOrganizationMemberIndex(cfg *gitopsconfig.OrganizationConfig, username string) (int, bool) {
	if cfg == nil {
		return -1, false
	}

	wantUsername := strings.TrimSpace(username)
	for index, member := range cfg.Members {
		if strings.EqualFold(strings.TrimSpace(member.Username), wantUsername) {
			return index, true
		}
	}

	return -1, false
}

// FindTeamMemberIndex returns the index of a team member matching username
// within team. Member identity is case-insensitive after trimming the value.
func FindTeamMemberIndex(team *gitopsconfig.TeamSpec, username string) (int, bool) {
	if team == nil {
		return -1, false
	}

	wantUsername := strings.TrimSpace(username)
	for index, member := range team.Members {
		if strings.EqualFold(strings.TrimSpace(member.Username), wantUsername) {
			return index, true
		}
	}

	return -1, false
}
