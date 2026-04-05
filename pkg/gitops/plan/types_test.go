package plan

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestReportNormalizeNilReceiver(t *testing.T) {
	t.Parallel()

	var report *Report
	report.Normalize()
}

func TestActionNormalizeNilReceiver(t *testing.T) {
	t.Parallel()

	var action *Action
	action.Normalize()
}

func TestReportNormalizeInitializesNilSlicesAndEmptySummary(t *testing.T) {
	t.Parallel()

	report := &Report{Organization: "orang-gaboets"}

	report.Normalize()

	if report.Actions == nil {
		t.Fatal("expected actions to be initialized")
	}
	if report.Summary.HasChanges {
		t.Fatalf("expected has_changes false, got %#v", report.Summary)
	}
	if report.Summary.Actions != 0 || report.Summary.ExecutableActions != 0 || report.Summary.NonExecutableActions != 0 {
		t.Fatalf("unexpected summary counts: %#v", report.Summary)
	}
}

func TestReportNormalizeSortsActionsChangesAndSummary(t *testing.T) {
	t.Parallel()

	report := &Report{
		Organization: "orang-gaboets",
		Actions: []Action{
			{
				ResourceType: ActionResourceTypeTeamRepositoryPermission,
				Operation:    ActionOperationRemove,
				ResourceID:   "platform/orang-gaboets/octostate",
				Executable:   false,
				Message:      "remove extra team repo permission",
			},
			{
				ResourceType: ActionResourceTypeRepository,
				Operation:    ActionOperationUpdate,
				ResourceID:   "orang-gaboets/octostate",
				Executable:   true,
				Message:      "update repository settings",
				Changes: []FieldChange{
					{Field: "visibility", From: "public", To: "private"},
					{Field: "allow_forking", From: true, To: false},
				},
			},
			{
				ResourceType: ActionResourceTypeRepository,
				Operation:    ActionOperationCreate,
				ResourceID:   "orang-gaboets/new-repo",
				Executable:   true,
				Message:      "create repository",
			},
			{
				ResourceType: ActionResourceTypeTeam,
				Operation:    ActionOperationDelete,
				ResourceID:   "legacy-team",
				Executable:   false,
				Message:      "delete unsupported extra team",
			},
			{
				ResourceType: ActionResourceTypeOrganizationMember,
				Operation:    ActionOperationUpdate,
				ResourceID:   "alice",
				Executable:   true,
				Message:      "update organization member alice",
				Changes: []FieldChange{
					{Field: "role", From: "member", To: "admin"},
				},
			},
		},
	}

	report.Normalize()

	wantActions := []Action{
		{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationCreate,
			ResourceID:   "orang-gaboets/new-repo",
			Executable:   true,
			Message:      "create repository",
			Changes:      []FieldChange{},
		},
		{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationUpdate,
			ResourceID:   "orang-gaboets/octostate",
			Executable:   true,
			Message:      "update repository settings",
			Changes: []FieldChange{
				{Field: "allow_forking", From: true, To: false},
				{Field: "visibility", From: "public", To: "private"},
			},
		},
		{
			ResourceType: ActionResourceTypeTeam,
			Operation:    ActionOperationDelete,
			ResourceID:   "legacy-team",
			Executable:   false,
			Message:      "delete unsupported extra team",
			Changes:      []FieldChange{},
		},
		{
			ResourceType: ActionResourceTypeOrganizationMember,
			Operation:    ActionOperationUpdate,
			ResourceID:   "alice",
			Executable:   true,
			Message:      "update organization member alice",
			Changes: []FieldChange{
				{Field: "role", From: "member", To: "admin"},
			},
		},
		{
			ResourceType: ActionResourceTypeTeamRepositoryPermission,
			Operation:    ActionOperationRemove,
			ResourceID:   "platform/orang-gaboets/octostate",
			Executable:   false,
			Message:      "remove extra team repo permission",
			Changes:      []FieldChange{},
		},
	}
	if !reflect.DeepEqual(report.Actions, wantActions) {
		t.Fatalf("unexpected actions: got %#v want %#v", report.Actions, wantActions)
	}

	wantSummary := Summary{
		HasChanges:           true,
		Actions:              5,
		ExecutableActions:    3,
		NonExecutableActions: 2,
		CreateActions:        1,
		UpdateActions:        2,
		DeleteActions:        1,
		RemoveActions:        1,
	}
	if !reflect.DeepEqual(report.Summary, wantSummary) {
		t.Fatalf("unexpected summary: got %#v want %#v", report.Summary, wantSummary)
	}
}

func TestReportJSONUsesStableFieldNames(t *testing.T) {
	t.Parallel()

	payload, err := json.Marshal(Report{
		Organization: "orang-gaboets",
		Actions: []Action{
			{
				ResourceType: ActionResourceTypeTeamRepositoryPermission,
				Operation:    ActionOperationCreate,
				ResourceID:   "platform/orang-gaboets/octostate",
				Executable:   true,
				Message:      "create permission",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal plan report: %v", err)
	}

	var got map[string]json.RawMessage
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("unmarshal plan report: %v", err)
	}

	if _, ok := got["organization"]; !ok {
		t.Fatalf("expected organization key in JSON payload: %s", string(payload))
	}
	if _, ok := got["summary"]; !ok {
		t.Fatalf("expected summary key in JSON payload: %s", string(payload))
	}
	if _, ok := got["actions"]; !ok {
		t.Fatalf("expected actions key in JSON payload: %s", string(payload))
	}
}
