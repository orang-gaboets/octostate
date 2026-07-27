package diff

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/internal/testconfig"
	"github.com/orang-gaboets/octostate/pkg/gitops/snapshot"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	validSnapshot := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
	})

	tests := []struct {
		name    string
		options Options
		wantErr error
	}{
		{
			name:    "missing organization",
			options: Options{Snapshot: &validSnapshot},
			wantErr: githubpkg.ErrMissingRequiredField,
		},
		{
			name:    "missing snapshot",
			options: Options{Desired: config.OrganizationConfig{Organization: "orang-gaboets"}},
			wantErr: githubpkg.ErrMissingRequiredField,
		},
		{
			name: "organization mismatch",
			options: Options{
				Desired:  config.OrganizationConfig{Organization: "orang-gaboets"},
				Snapshot: &snapshot.ActualSnapshot{Organization: "other-org"},
			},
			wantErr: githubpkg.ErrInvalidFieldValue,
		},
		{
			name: "valid",
			options: Options{
				Desired:  config.OrganizationConfig{Organization: "orang-gaboets"},
				Snapshot: &validSnapshot,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.options.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error: got %v want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildRejectsConflictingResolvedInviteUserIDsByUsername(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
	})

	_, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
		},
		Snapshot: &snap,
		ResolvedInviteUserIDsByUsername: map[string]int64{
			"octocat": 1,
			"OCTOCAT": 2,
		},
	})
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: got %v want %v", err, githubpkg.ErrInvalidFieldValue)
	}
}

func TestBuildUsesSnapshotResolvedInviteUserIDsByUsernameByDefault(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		PendingInvitations: []state.PendingInvitation{
			{Username: "octocat"},
		},
	})
	snap.ResolvedInviteUserIDsByUsername = map[string]int64{"octocat": 99}

	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if report.Summary.HasChanges {
		t.Fatalf("expected no drift when snapshot mapping satisfies user_id invite, got %#v", report)
	}
}

func TestBuildRejectsInviteThatDuplicatesDesiredMemberByResolvedUserID(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
	})
	snap.ResolvedInviteUserIDsByUsername = map[string]int64{"alice": 99}

	_, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: "alice", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Snapshot: &snap,
	})
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: got %v want %v", err, githubpkg.ErrInvalidFieldValue)
	}
	if err == nil || !strings.Contains(err.Error(), "duplicates a declared top-level member") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestBuildRejectsInviteThatDuplicatesUnnormalizedDesiredMemberByResolvedUserID(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
	})
	snap.ResolvedInviteUserIDsByUsername = map[string]int64{"alice": 99}

	_, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: " alice ", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Snapshot: &snap,
	})
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: got %v want %v", err, githubpkg.ErrInvalidFieldValue)
	}
	if err == nil || !strings.Contains(err.Error(), "duplicates a declared top-level member") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestBuildOptionsResolvedInviteUserIDsRejectConflictingSnapshotValues(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		PendingInvitations: []state.PendingInvitation{
			{Username: "octocat"},
		},
	})
	snap.ResolvedInviteUserIDsByUsername = map[string]int64{"octocat": 1}

	_, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Snapshot:                        &snap,
		ResolvedInviteUserIDsByUsername: map[string]int64{"OCTOCAT": 99},
	})
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: got %v want %v", err, githubpkg.ErrInvalidFieldValue)
	}
}

func TestBuildNoDriftWhenDesiredMatchesSnapshot(t *testing.T) {
	t.Parallel()

	pulledAt := time.Date(2026, 3, 14, 9, 30, 0, 0, time.FixedZone("SGT", 8*60*60))
	actual := state.OrganizationState{
		Organization: "ORANG-GABOETS",
		Members: []state.OrganizationMember{
			{Username: "alice", Role: "member"},
		},
		PendingInvitations: []state.PendingInvitation{
			{ID: 10, Username: "ZOE", Role: "admin", TeamSlugs: []string{}},
		},
		Repositories: []state.Repository{
			{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "private",
				Description:  "CLI",
				Homepage:     "https://example.com/octostate",
				Topics:       []string{"go", "gitops"},
				AllowForking: false,
				Archived:     false,
				IsTemplate:   false,
			},
		},
		Teams: []state.Team{
			{Slug: "platform", Name: "Platform", Description: "Platform engineering", Privacy: "closed"},
		},
		TeamMembers: []state.TeamMember{
			{TeamSlug: "platform", Username: "alice", Role: "member"},
		},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{
			{TeamSlug: "platform", Owner: "orang-gaboets", Name: "octostate", Permission: "push"},
		},
	}

	snap := snapshot.NewActualSnapshot(pulledAt, &actual)
	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: "alice", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{Username: presentString("zoe"), Role: "direct_member", TeamSlugs: []string{"platform"}},
			},
			Repositories: []config.RepositorySpec{
				{
					Owner:        "orang-gaboets",
					Name:         "octostate",
					Visibility:   "private",
					Description:  "CLI",
					Homepage:     "https://example.com/octostate",
					Topics:       []string{"gitops", "go"},
					AllowForking: false,
					Archived:     false,
					IsTemplate:   false,
				},
			},
			Teams: []config.TeamSpec{
				{
					Slug:        "platform",
					Name:        "Platform",
					Description: "Platform engineering",
					Privacy:     "closed",
					Members: []config.TeamMemberSpec{
						{Username: "alice", Role: "member"},
					},
					Repositories: []config.TeamRepositorySpec{
						{Owner: "orang-gaboets", Name: "octostate", Permission: "push"},
					},
				},
			},
		},
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := &Report{
		Organization:     "orang-gaboets",
		SnapshotPulledAt: pulledAt.UTC(),
		Summary: Summary{
			HasChanges:           false,
			Actions:              0,
			ExecutableActions:    0,
			NonExecutableActions: 0,
			CreateActions:        0,
			UpdateActions:        0,
			DeleteActions:        0,
			RemoveActions:        0,
		},
		Actions: []Action{},
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("unexpected report:\n got %#v\nwant %#v", report, want)
	}
}

func TestBuildPlansDeterministicDriftActions(t *testing.T) {
	t.Parallel()

	pulledAt := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	actual := state.OrganizationState{
		Organization: "orang-gaboets",
		Members: []state.OrganizationMember{
			{Username: "alice", Role: "member"},
			{Username: "bob", Role: "member"},
		},
		PendingInvitations: []state.PendingInvitation{
			{ID: 5, Email: "orphan@example.com", Role: "direct_member", TeamSlugs: []string{}},
		},
		Repositories: []state.Repository{
			{Owner: "orang-gaboets", Name: "orphan-repo", Visibility: "private", Topics: []string{}},
			{
				Owner:        "orang-gaboets",
				Name:         "existing-repo",
				Visibility:   "public",
				Description:  "Old desc",
				Homepage:     "",
				Topics:       []string{"gitops"},
				AllowForking: true,
				Archived:     false,
				IsTemplate:   false,
			},
		},
		Teams: []state.Team{
			{Slug: "legacy", Name: "Legacy", Description: "Legacy team", Privacy: "closed"},
			{Slug: "platform", Name: "Platform Old", Description: "Old desc", Privacy: "closed"},
		},
		TeamMembers: []state.TeamMember{
			{TeamSlug: "platform", Username: "bob", Role: "member"},
			{TeamSlug: "platform", Username: "alice", Role: "member"},
		},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{
			{TeamSlug: "platform", Owner: "orang-gaboets", Name: "repo-old", Permission: "pull"},
			{TeamSlug: "platform", Owner: "orang-gaboets", Name: "octostate", Permission: "push"},
		},
	}

	snap := snapshot.NewActualSnapshot(pulledAt, &actual)
	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: "alice", Role: "admin"},
				{Username: "charlie", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{Username: presentString("invite-user")},
				{Email: presentString("invite@example.com")},
				{UserID: presentInt64(42)},
			},
			Repositories: []config.RepositorySpec{
				{
					Owner:        "orang-gaboets",
					Name:         "new-repo",
					Visibility:   "private",
					Description:  "",
					Homepage:     "",
					Topics:       []string{},
					AllowForking: false,
					Archived:     false,
					IsTemplate:   false,
				},
				{
					Owner:        "orang-gaboets",
					Name:         "existing-repo",
					Visibility:   "private",
					Description:  "New desc",
					Homepage:     "https://example.com/octostate",
					Topics:       []string{"go", "gitops"},
					AllowForking: false,
					Archived:     true,
					IsTemplate:   true,
				},
			},
			Teams: []config.TeamSpec{
				{
					Slug:        "platform",
					Name:        "Platform New",
					Description: "New desc",
					Privacy:     "secret",
					Members: []config.TeamMemberSpec{
						{Username: "alice", Role: "maintainer"},
						{Username: "charlie", Role: "member"},
					},
					Repositories: []config.TeamRepositorySpec{
						{Owner: "orang-gaboets", Name: "octostate", Permission: "admin"},
						{Owner: "orang-gaboets", Name: "repo-extra", Permission: "push"},
					},
				},
				{
					Slug:         "fresh",
					Name:         "Fresh",
					Description:  "",
					Privacy:      "closed",
					Members:      []config.TeamMemberSpec{},
					Repositories: []config.TeamRepositorySpec{},
				},
			},
		},
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := &Report{
		Organization:     "orang-gaboets",
		SnapshotPulledAt: pulledAt.UTC(),
		Summary: Summary{
			HasChanges:           true,
			Actions:              19,
			ExecutableActions:    9,
			NonExecutableActions: 10,
			CreateActions:        8,
			UpdateActions:        5,
			DeleteActions:        3,
			RemoveActions:        3,
		},
		Actions: []Action{
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationUpdate, ResourceID: "orang-gaboets/existing-repo", Executable: true, Message: "update repository orang-gaboets/existing-repo", Changes: []FieldChange{{Field: "archived", From: false, To: true}, {Field: "description", From: "Old desc", To: "New desc"}, {Field: "homepage", From: "", To: "https://example.com/octostate"}, {Field: "is_template", From: false, To: true}, {Field: "topics", From: []string{"gitops"}, To: []string{"gitops", "go"}}, {Field: "visibility", From: "public", To: "private"}}},
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationCreate, ResourceID: "orang-gaboets/new-repo", Executable: false, Message: "repository orang-gaboets/new-repo cannot be created because template configuration is missing", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationDelete, ResourceID: "orang-gaboets/orphan-repo", Executable: false, Message: "repository orang-gaboets/orphan-repo exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationCreate, ResourceID: "fresh", Executable: true, Message: "create team fresh", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationUpdate, ResourceID: "platform", Executable: true, Message: "update team platform", Changes: []FieldChange{{Field: "description", From: "Old desc", To: "New desc"}, {Field: "name", From: "Platform Old", To: "Platform New"}, {Field: "privacy", From: "closed", To: "secret"}}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationDelete, ResourceID: "legacy", Executable: false, Message: "team legacy exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeOrganizationMember, Operation: ActionOperationCreate, ResourceID: "charlie", Executable: true, Message: "create organization member charlie", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeOrganizationMember, Operation: ActionOperationUpdate, ResourceID: "alice", Executable: true, Message: "update organization member alice", Changes: []FieldChange{{Field: "role", From: "member", To: "admin"}}},
			{ResourceType: ActionResourceTypeOrganizationMember, Operation: ActionOperationDelete, ResourceID: "bob", Executable: false, Message: "organization member bob exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "email:invite@example.com", Executable: true, Message: "create organization invite email:invite@example.com", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "user_id:42", Executable: true, Message: "create organization invite user_id:42", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "username:invite-user", Executable: true, Message: "create organization invite username:invite-user", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationRemove, ResourceID: "email:orphan@example.com", Executable: false, Message: "pending invitation email:orphan@example.com exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationCreate, ResourceID: "platform/charlie", Executable: false, Message: "team membership platform/charlie requires organization member charlie to exist first", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationUpdate, ResourceID: "platform/alice", Executable: true, Message: "update team membership platform/alice", Changes: []FieldChange{{Field: "role", From: "member", To: "maintainer"}}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationRemove, ResourceID: "platform/bob", Executable: false, Message: "team membership platform/bob exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationCreate, ResourceID: "platform/orang-gaboets/repo-extra", Executable: false, Message: "team repository permission platform/orang-gaboets/repo-extra requires repository orang-gaboets/repo-extra to exist or be created earlier in the same plan", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationUpdate, ResourceID: "platform/orang-gaboets/octostate", Executable: false, Message: "team repository permission platform/orang-gaboets/octostate requires repository orang-gaboets/octostate to exist or be created earlier in the same plan", Changes: []FieldChange{{Field: "permission", From: "push", To: "admin"}}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationRemove, ResourceID: "platform/orang-gaboets/repo-old", Executable: false, Message: "team repository permission platform/orang-gaboets/repo-old exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
		},
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("unexpected report:\n got %#v\nwant %#v", report, want)
	}
}

func TestBuildInviteUserIDSatisfiedByPendingInviteUsesResolvedUserIDMap(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 10, 30, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		PendingInvitations: []state.PendingInvitation{
			{ID: 10, Username: "octocat", Role: "direct_member"},
		},
	})

	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Snapshot:                        &snap,
		ResolvedInviteUserIDsByUsername: map[string]int64{"octocat": 99},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(report.Actions) != 0 {
		t.Fatalf("expected no actions, got %#v", report.Actions)
	}
}

func TestBuildInvitesSatisfiedByExistingMembers(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 11, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		Members: []state.OrganizationMember{
			{ID: 99, Username: "octocat", Role: "member", Email: "octocat@example.com"},
		},
	})

	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{Username: presentString("octocat")},
				{Email: presentString("octocat@example.com")},
				{UserID: presentInt64(99)},
			},
		},
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	want := []Action{{
		ResourceType: ActionResourceTypeOrganizationMember,
		Operation:    ActionOperationDelete,
		ResourceID:   "octocat",
		Executable:   false,
		Message:      "organization member octocat exists in snapshot state but is not declared in desired config",
		Changes:      []FieldChange{},
	}}
	if !reflect.DeepEqual(report.Actions, want) {
		t.Fatalf("unexpected actions:\n got %#v\nwant %#v", report.Actions, want)
	}
}

func TestBuildInviteUserIDPendingInviteWithoutResolvedUserIDMappingCreatesDrift(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 10, 45, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		PendingInvitations: []state.PendingInvitation{
			{ID: 10, Username: "octocat", Role: "direct_member"},
		},
	})

	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	want := &Report{
		Organization:     "orang-gaboets",
		SnapshotPulledAt: snap.PulledAt.UTC(),
		Summary: Summary{
			HasChanges:           true,
			Actions:              2,
			ExecutableActions:    1,
			NonExecutableActions: 1,
			CreateActions:        1,
			UpdateActions:        0,
			DeleteActions:        0,
			RemoveActions:        1,
		},
		Actions: []Action{
			{
				ResourceType: ActionResourceTypeInvite,
				Operation:    ActionOperationCreate,
				ResourceID:   "user_id:99",
				Executable:   true,
				Message:      "create organization invite user_id:99",
				Changes:      []FieldChange{},
			},
			{
				ResourceType: ActionResourceTypeInvite,
				Operation:    ActionOperationRemove,
				ResourceID:   "username:octocat",
				Executable:   false,
				Message:      "pending invitation username:octocat exists in snapshot state but is not declared in desired config",
				Changes:      []FieldChange{},
			},
		},
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("unexpected report:\n got %#v\nwant %#v", report, want)
	}
}

func TestBuildPrivateRepositoryIgnoresAllowForkingDrift(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 11, 15, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{
			{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "private",
				Description:  "CLI",
				Homepage:     "https://example.com/octostate",
				Topics:       []string{"gitops"},
				AllowForking: true,
				Archived:     false,
				IsTemplate:   false,
			},
		},
	})

	desired := testconfig.LoadDesiredConfig(t, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: private
    description: "CLI"
    homepage: "https://example.com/octostate"
    topics: [gitops]
    allow_forking: false
    archived: false
    is_template: false
teams: []
invites: []
`)

	report, err := Build(Options{
		Desired:  desired,
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(report.Actions) != 0 {
		t.Fatalf("expected no actions, got %#v", report.Actions)
	}
}

func TestBuildRepositoryOmittedOptionalFieldsProduceNoDiff(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 11, 20, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{
			{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "public",
				Description:  "CLI",
				Homepage:     "https://example.com/octostate",
				Topics:       []string{"gitops"},
				AllowForking: true,
				Archived:     true,
				IsTemplate:   true,
			},
		},
	})

	desired := testconfig.LoadDesiredConfig(t, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: public
    topics: [gitops]
teams: []
invites: []
`)

	report, err := Build(Options{
		Desired:  desired,
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(report.Actions) != 0 {
		t.Fatalf("expected omitted optional repository fields to produce no diff, got %#v", report.Actions)
	}
}

func TestBuildRepositoryExplicitOptionalZeroValuesProduceDiff(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 11, 25, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{
			{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "public",
				Description:  "CLI",
				Homepage:     "https://example.com/octostate",
				Topics:       []string{"gitops"},
				AllowForking: true,
				Archived:     true,
				IsTemplate:   true,
			},
		},
	})

	desired := testconfig.LoadDesiredConfig(t, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: public
    description: ""
    homepage: ""
    topics: [gitops]
    allow_forking: false
    archived: false
    is_template: false
teams: []
invites: []
`)

	report, err := Build(Options{
		Desired:  desired,
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := []Action{{
		ResourceType: ActionResourceTypeRepository,
		Operation:    ActionOperationUpdate,
		ResourceID:   "orang-gaboets/octostate",
		Executable:   true,
		Message:      "update repository orang-gaboets/octostate",
		Changes: []FieldChange{
			{Field: "allow_forking", From: true, To: false},
			{Field: "archived", From: true, To: false},
			{Field: "description", From: "CLI", To: ""},
			{Field: "homepage", From: "https://example.com/octostate", To: ""},
			{Field: "is_template", From: true, To: false},
		},
	}}
	if !reflect.DeepEqual(report.Actions, want) {
		t.Fatalf("unexpected actions:\n got %#v\nwant %#v", report.Actions, want)
	}
}

func TestBuildOrdersRepositoryUpdateBeforeCreateWhenTemplateStateChangesInSameSnapshotPlan(t *testing.T) {
	t.Parallel()

	templateRepo := config.RepositorySpec{
		Owner:      "orang-gaboets",
		Name:       "zzz-template",
		Visibility: "private",
	}
	templateRepo.SetManagedIsTemplate(true)

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 11, 25, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{
			{
				Owner:      "orang-gaboets",
				Name:       "zzz-template",
				Visibility: "private",
				IsTemplate: false,
			},
		},
	})

	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []config.RepositorySpec{
				{
					Owner:      "orang-gaboets",
					Name:       "aaa-app",
					Visibility: "private",
					Template: config.TemplateSpec{
						Owner: "orang-gaboets",
						Name:  "zzz-template",
					},
				},
				templateRepo,
			},
		},
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := []Action{
		{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationUpdate,
			ResourceID:   "orang-gaboets/zzz-template",
			Executable:   true,
			Message:      "update repository orang-gaboets/zzz-template",
			Changes: []FieldChange{
				{Field: "is_template", From: false, To: true},
			},
		},
		{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationCreate,
			ResourceID:   "orang-gaboets/aaa-app",
			Executable:   true,
			Message:      "create repository orang-gaboets/aaa-app",
			Changes:      []FieldChange{},
		},
	}
	if !reflect.DeepEqual(report.Actions, want) {
		t.Fatalf("unexpected actions:\n got %#v\nwant %#v", report.Actions, want)
	}
}

func TestBuildActionsKeepsFixedPhaseOrder(t *testing.T) {
	t.Parallel()

	const waitTimeout = 5 * time.Second

	phaseDefinitions := []struct {
		resourceType ActionResourceType
		resourceID   string
	}{
		{resourceType: ActionResourceTypeRepository, resourceID: "repositories"},
		{resourceType: ActionResourceTypeTeam, resourceID: "teams"},
		{resourceType: ActionResourceTypeOrganizationMember, resourceID: "organization-members"},
		{resourceType: ActionResourceTypeInvite, resourceID: "invites"},
		{resourceType: ActionResourceTypeTeamMember, resourceID: "team-members"},
		{resourceType: ActionResourceTypeTeamRepositoryPermission, resourceID: "team-repository-permissions"},
	}

	releases := make([]chan struct{}, len(phaseDefinitions))
	for i := range releases {
		releases[i] = make(chan struct{})
	}

	started := make(chan int, len(releases))
	buildPhase := func(index int, resourceType ActionResourceType, resourceID string) actionPhase {
		return func(ctx context.Context) ([]Action, error) {
			started <- index
			select {
			case <-releases[index]:
				return []Action{{
					ResourceType: resourceType,
					Operation:    ActionOperationCreate,
					ResourceID:   resourceID,
				}}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	phases := make([]actionPhase, 0, len(phaseDefinitions))
	for index, phaseDefinition := range phaseDefinitions {
		phases = append(phases, buildPhase(index, phaseDefinition.resourceType, phaseDefinition.resourceID))
	}
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultCh := make(chan [][]Action, 1)
	errCh := make(chan error, 1)
	go func() {
		phaseResults, err := runPhases(runCtx, len(phases), phases)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- phaseResults
	}()

	startTimeout := time.NewTimer(waitTimeout)
	defer startTimeout.Stop()
	seen := make(map[int]struct{}, len(releases))
	for len(seen) < len(releases) {
		select {
		case index := <-started:
			seen[index] = struct{}{}
		case <-startTimeout.C:
			cancel()
			t.Fatal("timed out waiting for all phase goroutines to start")
		}
	}
	for _, index := range []int{1, 3, 5, 4, 2, 0} {
		close(releases[index])
	}

	var (
		phaseResults [][]Action
		err          error
	)
	resultTimeout := time.NewTimer(waitTimeout)
	defer resultTimeout.Stop()
	select {
	case phaseResults = <-resultCh:
	case err = <-errCh:
	case <-resultTimeout.C:
		cancel()
		t.Fatal("timed out waiting for phase results")
	}
	if err != nil {
		t.Fatalf("runPhases returned error: %v", err)
	}

	actions := flattenPhaseResults(phaseResults)
	wantResourceTypes := []ActionResourceType{
		ActionResourceTypeRepository,
		ActionResourceTypeTeam,
		ActionResourceTypeOrganizationMember,
		ActionResourceTypeInvite,
		ActionResourceTypeTeamMember,
		ActionResourceTypeTeamRepositoryPermission,
	}
	if len(actions) != len(wantResourceTypes) {
		t.Fatalf("unexpected action count: got %d want %d", len(actions), len(wantResourceTypes))
	}

	gotResourceTypes := make([]ActionResourceType, 0, len(actions))
	for _, action := range actions {
		gotResourceTypes = append(gotResourceTypes, action.ResourceType)
	}
	if !reflect.DeepEqual(gotResourceTypes, wantResourceTypes) {
		t.Fatalf("unexpected action order: got %#v want %#v", gotResourceTypes, wantResourceTypes)
	}
}

func TestBuildActionsConcurrentMatchesSequential(t *testing.T) {
	t.Parallel()

	builder := largeFixtureBuilder()

	want, err := builder.buildActionsWithLimit(1)
	if err != nil {
		t.Fatalf("buildActionsWithLimit returned error: %v", err)
	}

	got, err := builder.buildActions()
	if err != nil {
		t.Fatalf("buildActions returned error: %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected action set:\n got %#v\nwant %#v", got, want)
	}
}

func TestBuildActionsInviteErrorMatchesSequential(t *testing.T) {
	t.Parallel()

	builder := builder{
		desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: "alice", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		actual: state.OrganizationState{
			Organization: "orang-gaboets",
		},
		resolvedInviteUserIDsByUsername: map[string]int64{
			"alice": 99,
		},
	}

	_, sequentialErr := builder.buildActionsWithLimit(1)
	if sequentialErr == nil {
		t.Fatal("expected sequential buildActionsWithLimit to fail")
	}

	_, concurrentErr := builder.buildActions()
	if concurrentErr == nil {
		t.Fatal("expected concurrent buildActions to fail")
	}

	if !errors.Is(concurrentErr, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected concurrent error: got %v want %v", concurrentErr, githubpkg.ErrInvalidFieldValue)
	}
	if concurrentErr.Error() != sequentialErr.Error() {
		t.Fatalf("unexpected concurrent error text: got %q want %q", concurrentErr.Error(), sequentialErr.Error())
	}
}

func presentString(value string) config.OptionalString {
	return config.OptionalString{
		Present: true,
		Value:   value,
	}
}

func presentInt64(value int64) config.OptionalInt64 {
	return config.OptionalInt64{
		Present: true,
		Value:   value,
	}
}

func TestBuildTeamRepositoryPermissionCreateIsExecutableWhenRepositoryCanBeCreated(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 11, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		Teams: []state.Team{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
		}},
	})

	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []config.RepositorySpec{{
				Owner:      "ORANG-GABOETS",
				Name:       "OctoState",
				Visibility: "private",
				Template: config.TemplateSpec{
					Owner: "ORANG-GABOETS",
					Name:  "Repo-Template",
				},
			}},
			Teams: []config.TeamSpec{{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
				Repositories: []config.TeamRepositorySpec{{
					Owner:      "orang-gaboets",
					Name:       "octostate",
					Permission: "push",
				}},
			}},
		},
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := []Action{
		{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationCreate,
			ResourceID:   "ORANG-GABOETS/OctoState",
			Executable:   true,
			Message:      "create repository ORANG-GABOETS/OctoState",
			Changes:      []FieldChange{},
		},
		{
			ResourceType: ActionResourceTypeTeamRepositoryPermission,
			Operation:    ActionOperationCreate,
			ResourceID:   "platform/orang-gaboets/octostate",
			Executable:   true,
			Message:      "create team repository permission platform/orang-gaboets/octostate",
			Changes:      []FieldChange{},
		},
	}
	if !reflect.DeepEqual(report.Actions, want) {
		t.Fatalf("unexpected actions:\n got %#v\nwant %#v", report.Actions, want)
	}
	if report.Summary.ExecutableActions != 2 || report.Summary.NonExecutableActions != 0 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
}

func TestBuildTeamRepositoryPermissionCreateIsExecutableWhenRepositoryExistsInSnapshot(t *testing.T) {
	t.Parallel()

	snap := snapshot.NewActualSnapshot(time.Date(2026, 3, 14, 11, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{{
			Owner:      "orang-gaboets",
			Name:       "octostate",
			Visibility: "private",
		}},
		Teams: []state.Team{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
		}},
	})

	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []config.RepositorySpec{{
				Owner:      "orang-gaboets",
				Name:       "octostate",
				Visibility: "private",
			}},
			Teams: []config.TeamSpec{{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
				Repositories: []config.TeamRepositorySpec{{
					Owner:      "orang-gaboets",
					Name:       "octostate",
					Permission: "push",
				}},
			}},
		},
		Snapshot: &snap,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := []Action{
		{
			ResourceType: ActionResourceTypeTeamRepositoryPermission,
			Operation:    ActionOperationCreate,
			ResourceID:   "platform/orang-gaboets/octostate",
			Executable:   true,
			Message:      "create team repository permission platform/orang-gaboets/octostate",
			Changes:      []FieldChange{},
		},
	}
	if !reflect.DeepEqual(report.Actions, want) {
		t.Fatalf("unexpected actions:\n got %#v\nwant %#v", report.Actions, want)
	}
}
