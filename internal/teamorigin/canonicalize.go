// Package teamorigin canonicalizes team membership origin rows across API and
// GitOps state boundaries.
package teamorigin

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/github"
)

// Membership is the common representation used to canonicalize team-member
// rows across the GitHub adapter and GitOps state boundaries.
type Membership struct {
	TeamSlug string
	Username string
	Role     string
	Origin   github.TeamMembershipOrigin
}

type membershipKey struct {
	teamSlug string
	username string
}

// Canonicalize trims membership keys, resolves case-insensitive duplicate
// identities by origin precedence, validates roles, and returns rows in
// deterministic order. Empty origins represent legacy zero-value rows and
// are normalized to unknown.
func Canonicalize(values []Membership) ([]Membership, error) {
	byKey := make(map[membershipKey][]Membership, len(values))
	for _, value := range values {
		value.TeamSlug = strings.TrimSpace(value.TeamSlug)
		value.Username = strings.TrimSpace(value.Username)
		value.Role = strings.TrimSpace(value.Role)
		if value.Origin == "" {
			value.Origin = github.TeamMembershipOriginUnknown
		}
		if !value.Origin.IsValid() {
			return nil, fmt.Errorf("team membership %q/%q has invalid origin %q: %w", value.TeamSlug, value.Username, value.Origin, github.ErrValidationFailed)
		}
		if value.TeamSlug == "" || value.Username == "" {
			return nil, fmt.Errorf("team membership requires a team slug and username: %w", github.ErrValidationFailed)
		}
		if value.Role != "member" && value.Role != "maintainer" {
			return nil, fmt.Errorf("team membership %q/%q has invalid role %q: %w", value.TeamSlug, value.Username, value.Role, github.ErrValidationFailed)
		}

		key := membershipKey{
			teamSlug: strings.ToLower(value.TeamSlug),
			username: strings.ToLower(value.Username),
		}
		byKey[key] = append(byKey[key], value)
	}

	canonical := make([]Membership, 0, len(byKey))
	keys := make([]membershipKey, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b membershipKey) int {
		if diff := cmp.Compare(a.teamSlug, b.teamSlug); diff != 0 {
			return diff
		}
		return cmp.Compare(a.username, b.username)
	})
	for _, key := range keys {
		duplicates := byKey[key]
		winningOrigin := duplicates[0].Origin
		for _, duplicate := range duplicates[1:] {
			if originPrecedence(duplicate.Origin) > originPrecedence(winningOrigin) {
				winningOrigin = duplicate.Origin
			}
		}

		var winner Membership
		winnerSet := false
		for _, duplicate := range duplicates {
			if duplicate.Origin != winningOrigin {
				continue
			}
			if winnerSet && duplicate.Role != winner.Role {
				return nil, fmt.Errorf("team membership %q/%q has conflicting roles for origin %q: %w", key.teamSlug, key.username, winningOrigin, github.ErrValidationFailed)
			}
			if !winnerSet || membershipSpellingLess(duplicate, winner) {
				winner = duplicate
				winnerSet = true
			}
		}
		canonical = append(canonical, winner)
	}

	return canonical, nil
}

func originPrecedence(origin github.TeamMembershipOrigin) int {
	switch origin {
	case github.TeamMembershipOriginDirect:
		return 3
	case github.TeamMembershipOriginUnknown:
		return 2
	case github.TeamMembershipOriginInherited:
		return 1
	default:
		return 0
	}
}

func membershipSpellingLess(a, b Membership) bool {
	if a.TeamSlug != b.TeamSlug {
		return cmp.Compare(a.TeamSlug, b.TeamSlug) < 0
	}
	return cmp.Compare(a.Username, b.Username) < 0
}
