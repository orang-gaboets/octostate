package apply

import (
	"reflect"
	"testing"

	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestApplyPlanSummaryCountsActionsWithoutNormalizingPlan(t *testing.T) {
	changes := []gitopsplan.FieldChange{{Field: "z"}, {Field: "a"}}
	plan := &gitopsplan.Report{
		Actions: []gitopsplan.Action{
			{Operation: gitopsplan.ActionOperationCreate, Executable: true},
			{Operation: gitopsplan.ActionOperationUpdate, Changes: changes},
			{Operation: gitopsplan.ActionOperationRemove, Executable: true},
			{Operation: gitopsplan.ActionOperationDelete},
		},
	}
	beforeChanges := append([]gitopsplan.FieldChange(nil), changes...)
	unresolved := []state.TeamMember{{TeamSlug: "platform", Username: "alice"}}

	got := applyPlanSummary(plan, unresolved)
	want := gitopsplan.Summary{
		HasChanges:            true,
		HasUnresolved:         true,
		UnresolvedMemberships: 1,
		Actions:               4,
		ExecutableActions:     2,
		NonExecutableActions:  2,
		CreateActions:         1,
		UpdateActions:         1,
		DeleteActions:         1,
		RemoveActions:         1,
	}
	if got != want {
		t.Fatalf("applyPlanSummary() = %#v, want %#v", got, want)
	}
	if !reflect.DeepEqual(changes, beforeChanges) {
		t.Fatalf("applyPlanSummary() reordered caller-owned field changes: got %#v, want %#v", changes, beforeChanges)
	}
}
