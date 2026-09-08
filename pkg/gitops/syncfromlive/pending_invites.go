package syncfromlive

import (
	"fmt"
	"slices"
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
	indexByIdentity := make(map[string]int, len(invitations))
	for _, invitation := range invitations {
		invite, err := pendingInvitationToInviteSpec(invitation)
		if err != nil {
			return nil, err
		}
		if pendingUsernameConflictsMember(invite, members) {
			continue
		}
		keys := pendingInvitationIdentityKeys(invitation)
		index := -1
		matchingIndexes := pendingInvitationIndexes(indexByIdentity, keys)
		if len(matchingIndexes) == 0 {
			index = len(invites)
			invites = append(invites, invite)
		} else {
			index = canonicalInviteIndex(matchingIndexes)
			if len(matchingIndexes) > 1 {
				invites[index] = invite
				removeIndexes := inviteIndexesExcept(matchingIndexes, index)
				invites, _ = removeInviteIndexes(invites, nil, removeIndexes)
				indexByIdentity = inviteIndexesByIdentity(invites)
			} else if !invites[index].Username.Present && invite.Username.Present {
				invites[index] = invite
			}
		}
		for _, key := range keys {
			indexByIdentity[key] = index
		}
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
	pendingIndexes := make(map[int]struct{}, len(invitations))
	for i, invite := range merged {
		indexByIdentity[inviteIdentityKey(invite)] = i
	}

	for _, invitation := range invitations {
		invite, err := pendingInvitationToInviteSpec(invitation)
		if err != nil {
			return nil, err
		}
		if pendingUsernameConflictsMember(invite, members) {
			continue
		}
		keys := pendingInvitationIdentityKeys(invitation)
		index := -1
		matchingIndexes := pendingInvitationIndexes(indexByIdentity, keys)
		if len(matchingIndexes) == 0 {
			index = len(merged)
			merged = append(merged, invite)
			pendingIndexes[index] = struct{}{}
		} else {
			index = canonicalInviteIndex(matchingIndexes)
			if len(matchingIndexes) > 1 {
				merged[index] = invite
				removeIndexes := inviteIndexesExcept(matchingIndexes, index)
				merged, pendingIndexes = removeInviteIndexes(merged, pendingIndexes, removeIndexes)
				indexByIdentity = inviteIndexesByIdentity(merged)
			} else if _, fromPending := pendingIndexes[index]; !fromPending ||
				(!merged[index].Username.Present && invite.Username.Present) {
				merged[index] = invite
			}
		}
		for _, key := range keys {
			indexByIdentity[key] = index
		}
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

func pendingInvitationIndexes(indexByIdentity map[string]int, keys []string) []int {
	indexes := make([]int, 0, len(keys))
	for _, key := range keys {
		if index, ok := indexByIdentity[key]; ok && !slices.Contains(indexes, index) {
			indexes = append(indexes, index)
		}
	}
	return indexes
}

func canonicalInviteIndex(indexes []int) int {
	canonical := indexes[0]
	for _, index := range indexes[1:] {
		if index < canonical {
			canonical = index
		}
	}
	return canonical
}

func inviteIndexesExcept(indexes []int, keep int) []int {
	result := make([]int, 0, len(indexes)-1)
	for _, index := range indexes {
		if index != keep {
			result = append(result, index)
		}
	}
	return result
}

func removeInviteIndexes(
	invites []config.InviteSpec,
	pendingIndexes map[int]struct{},
	removeIndexes []int,
) ([]config.InviteSpec, map[int]struct{}) {
	remove := make(map[int]struct{}, len(removeIndexes))
	for _, index := range removeIndexes {
		remove[index] = struct{}{}
	}
	result := make([]config.InviteSpec, 0, len(invites)-len(remove))
	resultPendingIndexes := make(map[int]struct{}, len(pendingIndexes))
	for index, invite := range invites {
		if _, ok := remove[index]; ok {
			continue
		}
		resultIndex := len(result)
		result = append(result, invite)
		if _, ok := pendingIndexes[index]; ok {
			resultPendingIndexes[resultIndex] = struct{}{}
		}
	}
	return result, resultPendingIndexes
}

func inviteIndexesByIdentity(invites []config.InviteSpec) map[string]int {
	indexes := make(map[string]int, len(invites))
	for index, invite := range invites {
		indexes[inviteIdentityKey(invite)] = index
	}
	return indexes
}

func pendingInvitationToInviteSpec(invitation state.PendingInvitation) (config.InviteSpec, error) {
	role := strings.TrimSpace(invitation.Role)
	if role != "" && role != "admin" && role != "direct_member" && role != "billing_manager" {
		return config.InviteSpec{}, fmt.Errorf("pending invitation %d has unsupported role %q: %w", invitation.ID, role, github.ErrInvalidFieldValue)
	}

	invite := config.InviteSpec{
		Role:      role,
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
