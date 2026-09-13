package plan

import (
	"fmt"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func (p planner) computeOrganizationMemberPlan() organizationMemberPlan {
	actions := make([]Action, 0)
	availability := make(map[string]organizationMemberAvailability, len(p.actual.Members)+len(p.desired.Members))

	// A member already present live is available to dependents regardless of
	// what this plan does.
	for _, member := range p.actual.Members {
		availability[organizationMemberKey(member.Username)] = organizationMemberAvailability{executable: true}
	}

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
			action := Action{
				ResourceType: ActionResourceTypeOrganizationMember,
				Operation:    ActionOperationCreate,
				ResourceID:   organizationMemberID(member.Username),
				Executable:   true,
				Message:      fmt.Sprintf("create organization member %s", organizationMemberID(member.Username)),
			}
			actions = append(actions, action)
			// A supported create makes the member available to dependents that
			// are ordered after it.
			availability[key] = organizationMemberAvailability{
				executable: action.Executable,
				diagnostic: fmt.Sprintf(
					"organization member %s cannot be created by this plan",
					organizationMemberID(member.Username),
				),
			}
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

	return organizationMemberPlan{actions: actions, availability: availability}
}
