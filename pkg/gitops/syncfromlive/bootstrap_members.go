package syncfromlive

import (
	"fmt"
	"slices"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func bootstrapOrganizationMembers(
	actualMembers []state.OrganizationMember,
	teamMembers []state.TeamMember,
) ([]config.OrganizationMemberSpec, error) {
	memberSpecs := make([]config.OrganizationMemberSpec, 0, len(actualMembers))
	memberIndex := make(map[string]struct{}, len(actualMembers))

	for _, actualMember := range actualMembers {
		username := strings.TrimSpace(actualMember.Username)
		if username == "" {
			return nil, fmt.Errorf("bootstrap organization member username is required: %w", githubpkg.ErrInvalidFieldValue)
		}

		role := strings.TrimSpace(actualMember.Role)
		switch role {
		case "admin", "member":
		default:
			return nil, fmt.Errorf("bootstrap organization member %q has unsupported role %q: %w", username, role, githubpkg.ErrInvalidFieldValue)
		}

		usernameKey := strings.ToLower(username)
		if _, ok := memberIndex[usernameKey]; ok {
			return nil, fmt.Errorf("bootstrap organization member %q is duplicated: %w", username, githubpkg.ErrInvalidFieldValue)
		}
		memberIndex[usernameKey] = struct{}{}

		memberSpecs = append(memberSpecs, config.OrganizationMemberSpec{
			Username: username,
			Role:     role,
		})
	}

	for _, teamMember := range teamMembers {
		username := strings.TrimSpace(teamMember.Username)
		if username == "" {
			continue
		}
		if _, ok := memberIndex[strings.ToLower(username)]; !ok {
			return nil, fmt.Errorf("bootstrap team member %q is missing from organization members: %w", username, githubpkg.ErrInvalidFieldValue)
		}
	}

	slices.SortFunc(memberSpecs, func(a, b config.OrganizationMemberSpec) int {
		aKey := strings.ToLower(a.Username)
		bKey := strings.ToLower(b.Username)
		switch {
		case aKey < bKey:
			return -1
		case aKey > bKey:
			return 1
		case a.Username < b.Username:
			return -1
		case a.Username > b.Username:
			return 1
		default:
			return compareBootstrapMemberRoles(a.Role, b.Role)
		}
	})

	return memberSpecs, nil
}

func compareBootstrapMemberRoles(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
