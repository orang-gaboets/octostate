package plan

import (
	"fmt"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func (p planner) planOrganizationMembers() []Action {
	actions := make([]Action, 0)

	actualMembers := make(map[string]state.OrganizationMember, len(p.actual.Members))
	for _, member := range p.actual.Members {
		actualMembers[organizationMemberKey(member.Username)] = member
	}

	desiredMembers := make(map[string]config.OrganizationMemberSpec, len(p.desired.Members))
	for _, member := range p.desired.Members {
		key := organizationMemberKey(member.Username)
		desiredMembers[key] = member

		actualMember, ok := actualMembers[key]
		if !ok {
			actions = append(actions, Action{
				ResourceType: ActionResourceTypeOrganizationMember,
				Operation:    ActionOperationCreate,
				ResourceID:   organizationMemberID(member.Username),
				Executable:   true,
				Message:      fmt.Sprintf("create organization member %s", organizationMemberID(member.Username)),
			})
			continue
		}

		if actualMember.Role == member.Role {
			continue
		}

		actions = append(actions, Action{
			ResourceType: ActionResourceTypeOrganizationMember,
			Operation:    ActionOperationUpdate,
			ResourceID:   organizationMemberID(member.Username),
			Executable:   true,
			Message:      fmt.Sprintf("update organization member %s", organizationMemberID(member.Username)),
			Changes: []FieldChange{{
				Field: "role",
				From:  actualMember.Role,
				To:    member.Role,
			}},
		})
	}

	for _, member := range p.actual.Members {
		key := organizationMemberKey(member.Username)
		if _, ok := desiredMembers[key]; ok {
			continue
		}

		actions = append(actions, Action{
			ResourceType: ActionResourceTypeOrganizationMember,
			Operation:    ActionOperationDelete,
			ResourceID:   organizationMemberID(member.Username),
			Executable:   false,
			Message:      fmt.Sprintf("organization member %s exists in live state but is not declared in desired config", organizationMemberID(member.Username)),
		})
	}

	return actions
}
