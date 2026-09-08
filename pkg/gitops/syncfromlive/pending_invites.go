package syncfromlive

import (
	"fmt"
	"sort"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func bootstrapPendingInvitations(
	invitations []state.PendingInvitation,
	members []config.OrganizationMemberSpec,
) ([]config.InviteSpec, error) {
	invites := make([]config.InviteSpec, 0, len(invitations))
	seen := make(map[string]struct{}, len(invitations))
	for _, invitation := range invitations {
		invite, err := pendingInvitationToInviteSpec(invitation)
		if err != nil {
			return nil, err
		}
		if pendingUsernameConflictsMember(invite, members) {
			continue
		}
		key := inviteIdentityKey(invite)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		invites = append(invites, invite)
	}
	return invites, nil
}

func mergePendingInvitations(
	desired []config.InviteSpec,
	invitations []state.PendingInvitation,
	members []config.OrganizationMemberSpec,
) ([]config.InviteSpec, error) {
	merged := append([]config.InviteSpec{}, desired...)
	indexByIdentity := make(map[string]int, len(merged))
	for i, invite := range merged {
		indexByIdentity[inviteIdentityKey(invite)] = i
	}

	seen := make(map[string]struct{}, len(invitations))
	for _, invitation := range invitations {
		invite, err := pendingInvitationToInviteSpec(invitation)
		if err != nil {
			return nil, err
		}
		if pendingUsernameConflictsMember(invite, members) {
			continue
		}
		keys := pendingInvitationIdentityKeys(invitation)
		if pendingInvitationAlreadySeen(seen, keys) {
			continue
		}
		for _, key := range keys {
			seen[key] = struct{}{}
		}
		matchingIndexes := pendingInvitationDesiredIndexes(indexByIdentity, keys)
		if len(matchingIndexes) > 0 {
			index := matchingIndexes[0]
			merged[index] = invite
			if len(matchingIndexes) > 1 {
				merged = removeInviteIndexes(merged, matchingIndexes[1:])
				indexByIdentity = inviteIndexesByIdentity(merged)
			}
			continue
		}
		indexByIdentity[inviteIdentityKey(invite)] = len(merged)
		merged = append(merged, invite)
	}
	return merged, nil
}

func pendingInvitationIdentityKeys(invitation state.PendingInvitation) []string {
	keys := make([]string, 0, 2)
	if username := strings.TrimSpace(invitation.Username); username != "" {
		keys = append(keys, "username\x00"+strings.ToLower(username))
	}
	if email := strings.TrimSpace(invitation.Email); email != "" {
		keys = append(keys, "email\x00"+strings.ToLower(email))
	}
	return keys
}

func pendingInvitationAlreadySeen(seen map[string]struct{}, keys []string) bool {
	for _, key := range keys {
		if _, ok := seen[key]; ok {
			return true
		}
	}
	return false
}

func pendingInvitationDesiredIndexes(indexByIdentity map[string]int, keys []string) []int {
	indexes := make([]int, 0, len(keys))
	for _, key := range keys {
		if index, ok := indexByIdentity[key]; ok {
			found := false
			for _, existing := range indexes {
				if existing == index {
					found = true
					break
				}
			}
			if !found {
				indexes = append(indexes, index)
			}
		}
	}
	// Preserve existing invite order when both aliases are already declared.
	sort.Ints(indexes)
	return indexes
}

func removeInviteIndexes(invites []config.InviteSpec, indexes []int) []config.InviteSpec {
	remove := make(map[int]struct{}, len(indexes))
	for _, index := range indexes {
		remove[index] = struct{}{}
	}
	result := make([]config.InviteSpec, 0, len(invites)-len(remove))
	for index, invite := range invites {
		if _, ok := remove[index]; !ok {
			result = append(result, invite)
		}
	}
	return result
}

func inviteIndexesByIdentity(invites []config.InviteSpec) map[string]int {
	indexes := make(map[string]int, len(invites))
	for index, invite := range invites {
		indexes[inviteIdentityKey(invite)] = index
	}
	return indexes
}

func pendingInvitationToInviteSpec(invitation state.PendingInvitation) (config.InviteSpec, error) {
	invite := config.InviteSpec{
		Role:      strings.TrimSpace(invitation.Role),
		TeamSlugs: append([]string{}, invitation.TeamSlugs...),
	}

	switch {
	case strings.TrimSpace(invitation.Username) != "":
		invite.Username = config.OptionalString{Present: true, Value: strings.TrimSpace(invitation.Username)}
	case strings.TrimSpace(invitation.Email) != "":
		invite.Email = config.OptionalString{Present: true, Value: strings.TrimSpace(invitation.Email)}
	default:
		return config.InviteSpec{}, fmt.Errorf("pending invitation %d has neither username nor email: %w", invitation.ID, github.ErrInvalidFieldValue)
	}

	return invite, nil
}

func inviteIdentityKey(invite config.InviteSpec) string {
	switch {
	case invite.Username.Present && !invite.Username.Null:
		return "username\x00" + strings.ToLower(strings.TrimSpace(invite.Username.Value))
	case invite.Email.Present && !invite.Email.Null:
		return "email\x00" + strings.ToLower(strings.TrimSpace(invite.Email.Value))
	case invite.UserID.Present && !invite.UserID.Null:
		return fmt.Sprintf("user_id\x00%d", invite.UserID.Value)
	default:
		return "invalid"
	}
}

func pendingUsernameConflictsMember(invite config.InviteSpec, members []config.OrganizationMemberSpec) bool {
	if !invite.Username.Present || invite.Username.Null {
		return false
	}
	username := strings.TrimSpace(invite.Username.Value)
	for _, member := range members {
		if strings.EqualFold(username, strings.TrimSpace(member.Username)) {
			return true
		}
	}
	return false
}
