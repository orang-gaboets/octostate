package syncfromlive

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func bootstrapTeamMembers(
	teams []state.Team,
	members []state.TeamMember,
) (map[string][]config.TeamMemberSpec, error) {
	knownTeams := make(map[string]struct{}, len(teams))
	for _, team := range teams {
		knownTeams[strings.ToLower(strings.TrimSpace(team.Slug))] = struct{}{}
	}

	result := make(map[string][]config.TeamMemberSpec, len(teams))
	for _, member := range members {
		teamKey := strings.ToLower(strings.TrimSpace(member.TeamSlug))
		if _, ok := knownTeams[teamKey]; !ok {
			return nil, fmt.Errorf("team membership references unknown team %q: %w", member.TeamSlug, githubpkg.ErrInvalidFieldValue)
		}

		result[teamKey] = append(result[teamKey], config.TeamMemberSpec{
			Username: strings.TrimSpace(member.Username),
			Role:     strings.TrimSpace(member.Role),
		})
	}

	return result, nil
}

func bootstrapTeamRepositoryPermissions(
	organization string,
	teams []state.Team,
	permissions []state.TeamRepositoryPermission,
) (map[string][]config.TeamRepositorySpec, error) {
	knownTeams := make(map[string]struct{}, len(teams))
	for _, team := range teams {
		knownTeams[strings.ToLower(strings.TrimSpace(team.Slug))] = struct{}{}
	}

	result := make(map[string][]config.TeamRepositorySpec, len(teams))
	for _, permission := range permissions {
		teamKey := strings.ToLower(strings.TrimSpace(permission.TeamSlug))
		if _, ok := knownTeams[teamKey]; !ok {
			return nil, fmt.Errorf("team repository permission references unknown team %q: %w", permission.TeamSlug, githubpkg.ErrInvalidFieldValue)
		}

		result[teamKey] = append(result[teamKey], config.TeamRepositorySpec{
			Owner:      bootstrapOwner(organization, permission.Owner),
			Name:       strings.TrimSpace(permission.Name),
			Permission: strings.TrimSpace(permission.Permission),
		})
	}

	return result, nil
}

func bootstrapTeams(
	teams []state.Team,
	membersByTeam map[string][]config.TeamMemberSpec,
	permissionsByTeam map[string][]config.TeamRepositorySpec,
) []config.TeamSpec {
	desired := make([]config.TeamSpec, 0, len(teams))

	for _, team := range teams {
		teamKey := strings.ToLower(strings.TrimSpace(team.Slug))
		members := append([]config.TeamMemberSpec{}, membersByTeam[teamKey]...)
		repositories := append([]config.TeamRepositorySpec{}, permissionsByTeam[teamKey]...)

		desired = append(desired, config.TeamSpec{
			Slug:         strings.TrimSpace(team.Slug),
			Name:         strings.TrimSpace(team.Name),
			Description:  strings.TrimSpace(team.Description),
			Privacy:      strings.TrimSpace(team.Privacy),
			ParentSlug:   strings.TrimSpace(team.ParentSlug),
			Members:      members,
			Repositories: repositories,
		})
	}

	return desired
}
