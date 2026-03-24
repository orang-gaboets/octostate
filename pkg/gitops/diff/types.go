package diff

import (
	"strings"
	"time"

	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
)

type (
	// ActionResourceType identifies the GitOps resource category affected by one
	// drift action.
	ActionResourceType = gitopsplan.ActionResourceType
	// ActionOperation identifies the drift operation represented by one action.
	ActionOperation = gitopsplan.ActionOperation
	// Summary contains the high-level counts derived from the action list.
	Summary = gitopsplan.Summary
	// Action describes one deterministic drift entry.
	Action = gitopsplan.Action
	// FieldChange describes one field-level difference for an update action.
	FieldChange = gitopsplan.FieldChange
)

// Shared action resource and operation constants mirror the GitOps planner so
// audit diff can expose the same machine-readable schema.
const (
	ActionResourceTypeRepository               = gitopsplan.ActionResourceTypeRepository
	ActionResourceTypeTeam                     = gitopsplan.ActionResourceTypeTeam
	ActionResourceTypeOrganizationMember       = gitopsplan.ActionResourceTypeOrganizationMember
	ActionResourceTypeInvite                   = gitopsplan.ActionResourceTypeInvite
	ActionResourceTypeTeamMember               = gitopsplan.ActionResourceTypeTeamMember
	ActionResourceTypeTeamRepositoryPermission = gitopsplan.ActionResourceTypeTeamRepositoryPermission

	ActionOperationCreate = gitopsplan.ActionOperationCreate
	ActionOperationUpdate = gitopsplan.ActionOperationUpdate
	ActionOperationDelete = gitopsplan.ActionOperationDelete
	ActionOperationRemove = gitopsplan.ActionOperationRemove
)

// Report is the machine-readable output produced by offline GitOps drift
// detection against a stored actual-state snapshot.
type Report struct {
	Organization     string    `json:"organization"`
	SnapshotPulledAt time.Time `json:"snapshot_pulled_at"`
	Summary          Summary   `json:"summary"`
	Actions          []Action  `json:"actions"`
}

// Normalize initializes nil slices, sorts actions deterministically, and
// recomputes summary counts using the shared planner ordering rules.
func (r *Report) Normalize() {
	if r == nil {
		return
	}

	base := gitopsplan.Report{
		Organization: strings.TrimSpace(r.Organization),
		Summary:      r.Summary,
		Actions:      append([]Action(nil), r.Actions...),
	}
	base.Normalize()

	r.Organization = base.Organization
	r.Summary = base.Summary
	r.Actions = base.Actions
	if !r.SnapshotPulledAt.IsZero() {
		r.SnapshotPulledAt = r.SnapshotPulledAt.UTC()
	}
}
