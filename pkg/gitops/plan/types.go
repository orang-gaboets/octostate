package plan

import (
	"fmt"
	"slices"
	"strings"
)

// ActionResourceType identifies the GitOps resource category affected by one
// plan action.
type ActionResourceType string

const (
	// ActionResourceTypeRepository identifies a repository resource action.
	ActionResourceTypeRepository ActionResourceType = "repository"
	// ActionResourceTypeTeam identifies a team resource action.
	ActionResourceTypeTeam ActionResourceType = "team"
	// ActionResourceTypeOrganizationMember identifies an organization membership action.
	ActionResourceTypeOrganizationMember ActionResourceType = "organization_member"
	// ActionResourceTypeInvite identifies an organization invitation action.
	ActionResourceTypeInvite ActionResourceType = "invite"
	// ActionResourceTypeTeamMember identifies a team membership action.
	ActionResourceTypeTeamMember ActionResourceType = "team_member"
	// ActionResourceTypeTeamRepositoryPermission identifies a team repository permission action.
	ActionResourceTypeTeamRepositoryPermission ActionResourceType = "team_repo_permission"
)

// ActionOperation identifies the reconciliation operation represented by one
// plan action.
type ActionOperation string

const (
	// ActionOperationCreate indicates the planner would create a missing resource.
	ActionOperationCreate ActionOperation = "create"
	// ActionOperationUpdate indicates the planner would update an existing resource.
	ActionOperationUpdate ActionOperation = "update"
	// ActionOperationDelete indicates unsupported extra live state that would need deletion.
	ActionOperationDelete ActionOperation = "delete"
	// ActionOperationRemove indicates unsupported extra live relationship state that would need removal.
	ActionOperationRemove ActionOperation = "remove"
)

// Report is the machine-readable output produced by GitOps planning.
type Report struct {
	Organization string   `json:"organization"`
	Summary      Summary  `json:"summary"`
	Actions      []Action `json:"actions"`
}

// Summary contains the high-level counts derived from the action list.
type Summary struct {
	HasChanges           bool `json:"has_changes"`
	Actions              int  `json:"actions"`
	ExecutableActions    int  `json:"executable_actions"`
	NonExecutableActions int  `json:"non_executable_actions"`
	CreateActions        int  `json:"create_actions"`
	UpdateActions        int  `json:"update_actions"`
	DeleteActions        int  `json:"delete_actions"`
	RemoveActions        int  `json:"remove_actions"`
}

// Action describes one deterministic reconciliation step or unsupported
// drift entry.
type Action struct {
	ResourceType ActionResourceType `json:"resource_type"`
	Operation    ActionOperation    `json:"operation"`
	ResourceID   string             `json:"resource_id"`
	Executable   bool               `json:"executable"`
	Message      string             `json:"message"`
	Changes      []FieldChange      `json:"changes,omitempty"`
}

// FieldChange describes one field-level difference for an update action.
type FieldChange struct {
	Field string `json:"field"`
	From  any    `json:"from,omitempty"`
	To    any    `json:"to,omitempty"`
}

// Normalize initializes nil slices, sorts actions deterministically, sorts
// field changes within each action, and recomputes summary counts.
func (r *Report) Normalize() {
	if r == nil {
		return
	}

	if r.Actions == nil {
		r.Actions = []Action{}
	}

	for i := range r.Actions {
		r.Actions[i].Normalize()
	}

	slices.SortFunc(r.Actions, comparePlanActions)
	r.Summary = summarizeActions(r.Actions)
}

// Normalize initializes nil field-change slices and sorts them deterministically.
func (a *Action) Normalize() {
	if a == nil {
		return
	}

	if a.Changes == nil {
		a.Changes = []FieldChange{}
	}

	slices.SortFunc(a.Changes, comparePlanFieldChanges)
}

func summarizeActions(actions []Action) Summary {
	summary := Summary{
		HasChanges: len(actions) > 0,
		Actions:    len(actions),
	}

	for _, action := range actions {
		if action.Executable {
			summary.ExecutableActions++
		} else {
			summary.NonExecutableActions++
		}

		switch action.Operation {
		case ActionOperationCreate:
			summary.CreateActions++
		case ActionOperationUpdate:
			summary.UpdateActions++
		case ActionOperationDelete:
			summary.DeleteActions++
		case ActionOperationRemove:
			summary.RemoveActions++
		}
	}

	return summary
}

func comparePlanActions(a, b Action) int {
	if diff := compareActionResourceTypes(a.ResourceType, b.ResourceType); diff != 0 {
		return diff
	}
	if diff := compareActionOperations(a.Operation, b.Operation); diff != 0 {
		return diff
	}
	if diff := compareStrings(a.ResourceID, b.ResourceID); diff != 0 {
		return diff
	}
	if a.Executable != b.Executable {
		if a.Executable {
			return -1
		}
		return 1
	}
	return compareStrings(a.Message, b.Message)
}

func comparePlanFieldChanges(a, b FieldChange) int {
	if diff := compareStrings(a.Field, b.Field); diff != 0 {
		return diff
	}
	if diff := compareStrings(fmt.Sprint(a.From), fmt.Sprint(b.From)); diff != 0 {
		return diff
	}
	return compareStrings(fmt.Sprint(a.To), fmt.Sprint(b.To))
}

func compareActionResourceTypes(a, b ActionResourceType) int {
	return compareOrderedStrings(string(a), string(b), []string{
		string(ActionResourceTypeRepository),
		string(ActionResourceTypeTeam),
		string(ActionResourceTypeOrganizationMember),
		string(ActionResourceTypeInvite),
		string(ActionResourceTypeTeamMember),
		string(ActionResourceTypeTeamRepositoryPermission),
	})
}

func compareActionOperations(a, b ActionOperation) int {
	return compareOrderedStrings(string(a), string(b), []string{
		string(ActionOperationCreate),
		string(ActionOperationUpdate),
		string(ActionOperationRemove),
		string(ActionOperationDelete),
	})
}

func compareOrderedStrings(a, b string, order []string) int {
	aRank := orderedStringRank(a, order)
	bRank := orderedStringRank(b, order)
	if aRank < bRank {
		return -1
	}
	if aRank > bRank {
		return 1
	}
	return compareStrings(a, b)
}

func orderedStringRank(value string, order []string) int {
	for i, candidate := range order {
		if value == candidate {
			return i
		}
	}
	return len(order)
}

func compareStrings(a, b string) int {
	aKey := strings.ToLower(a)
	bKey := strings.ToLower(b)
	if aKey < bKey {
		return -1
	}
	if aKey > bKey {
		return 1
	}
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
