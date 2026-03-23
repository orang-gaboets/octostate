package syncfromlive

import (
	"fmt"
	"slices"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func ensureNoUnsupportedDirectMembers(
	members []state.OrganizationMember,
	teamMembers []state.TeamMember,
) error {
	if len(members) == 0 {
		return nil
	}

	teamMemberUsernames := make(map[string]struct{}, len(teamMembers))
	for _, teamMember := range teamMembers {
		if username := normalizedMemberKey(teamMember.Username); username != "" {
			teamMemberUsernames[username] = struct{}{}
		}
	}

	unsupported := make([]string, 0)
	seen := make(map[string]struct{}, len(members))
	for _, member := range members {
		if username := normalizedMemberKey(member.Username); username != "" {
			if _, ok := teamMemberUsernames[username]; ok {
				continue
			}
		}

		label := bootstrapMemberLabel(member)
		labelKey := strings.ToLower(label)
		if _, ok := seen[labelKey]; ok {
			continue
		}
		seen[labelKey] = struct{}{}
		unsupported = append(unsupported, label)
	}

	if len(unsupported) == 0 {
		return nil
	}

	slices.SortFunc(unsupported, compareBootstrapLabels)
	return fmt.Errorf(
		"bootstrap does not support direct organization members outside teams: %s: %w",
		strings.Join(unsupported, ", "),
		githubpkg.ErrInvalidFieldValue,
	)
}

func normalizedMemberKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func bootstrapMemberLabel(member state.OrganizationMember) string {
	switch {
	case strings.TrimSpace(member.Username) != "":
		return strings.TrimSpace(member.Username)
	case strings.TrimSpace(member.Email) != "":
		return strings.TrimSpace(member.Email)
	case member.ID != 0:
		return fmt.Sprintf("id:%d", member.ID)
	case strings.TrimSpace(member.Name) != "":
		return strings.TrimSpace(member.Name)
	default:
		return "<unknown member>"
	}
}

func compareBootstrapLabels(a, b string) int {
	aKey := strings.ToLower(a)
	bKey := strings.ToLower(b)
	switch {
	case aKey < bKey:
		return -1
	case aKey > bKey:
		return 1
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
