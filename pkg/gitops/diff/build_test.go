package diff

import (
	"errors"
	"reflect"
	"testing"
	"time"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/internal/testconfig"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/snapshot"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
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
		PendingInvitations: []state.PendingInvitation{
			{ID: 10, Username: "ZOE", Role: "admin", TeamSlugs: []string{}},
		},
		Repositories: []state.Repository{
			{
				Owner:        "orang-gaboets",
				Name:         "repo-builder",
				Visibility:   "private",
				Description:  "CLI",
				Homepage:     "https://example.com/repo-builder",
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
			{TeamSlug: "platform", Owner: "orang-gaboets", Name: "repo-builder", Permission: "push"},
		},
	}

	snap := snapshot.NewActualSnapshot(pulledAt, &actual)
	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{Username: presentString("zoe"), Role: "direct_member", TeamSlugs: []string{"platform"}},
			},
			Repositories: []config.RepositorySpec{
				{
					Owner:        "orang-gaboets",
					Name:         "repo-builder",
					Visibility:   "private",
					Description:  "CLI",
					Homepage:     "https://example.com/repo-builder",
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
						{Owner: "orang-gaboets", Name: "repo-builder", Permission: "push"},
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
			{TeamSlug: "platform", Owner: "orang-gaboets", Name: "repo-builder", Permission: "push"},
		},
	}

	snap := snapshot.NewActualSnapshot(pulledAt, &actual)
	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
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
					Homepage:     "https://example.com/repo-builder",
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
						{Owner: "orang-gaboets", Name: "repo-builder", Permission: "admin"},
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
			Actions:              16,
			ExecutableActions:    10,
			NonExecutableActions: 6,
			CreateActions:        7,
			UpdateActions:        4,
			DeleteActions:        2,
			RemoveActions:        3,
		},
		Actions: []Action{
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationCreate, ResourceID: "orang-gaboets/new-repo", Executable: false, Message: "repository orang-gaboets/new-repo cannot be created because template configuration is missing", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationUpdate, ResourceID: "orang-gaboets/existing-repo", Executable: true, Message: "update repository orang-gaboets/existing-repo", Changes: []FieldChange{{Field: "archived", From: false, To: true}, {Field: "description", From: "Old desc", To: "New desc"}, {Field: "homepage", From: "", To: "https://example.com/repo-builder"}, {Field: "is_template", From: false, To: true}, {Field: "topics", From: []string{"gitops"}, To: []string{"gitops", "go"}}, {Field: "visibility", From: "public", To: "private"}}},
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationDelete, ResourceID: "orang-gaboets/orphan-repo", Executable: false, Message: "repository orang-gaboets/orphan-repo exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationCreate, ResourceID: "fresh", Executable: true, Message: "create team fresh", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationUpdate, ResourceID: "platform", Executable: true, Message: "update team platform", Changes: []FieldChange{{Field: "description", From: "Old desc", To: "New desc"}, {Field: "name", From: "Platform Old", To: "Platform New"}, {Field: "privacy", From: "closed", To: "secret"}}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationDelete, ResourceID: "legacy", Executable: false, Message: "team legacy exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "email:invite@example.com", Executable: true, Message: "create organization invite email:invite@example.com", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "user_id:42", Executable: true, Message: "create organization invite user_id:42", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "username:invite-user", Executable: true, Message: "create organization invite username:invite-user", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationRemove, ResourceID: "email:orphan@example.com", Executable: false, Message: "pending invitation email:orphan@example.com exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationCreate, ResourceID: "platform/charlie", Executable: true, Message: "add team membership platform/charlie", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationUpdate, ResourceID: "platform/alice", Executable: true, Message: "update team membership platform/alice", Changes: []FieldChange{{Field: "role", From: "member", To: "maintainer"}}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationRemove, ResourceID: "platform/bob", Executable: false, Message: "team membership platform/bob exists in snapshot state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationCreate, ResourceID: "platform/orang-gaboets/repo-extra", Executable: true, Message: "create team repository permission platform/orang-gaboets/repo-extra", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationUpdate, ResourceID: "platform/orang-gaboets/repo-builder", Executable: true, Message: "update team repository permission platform/orang-gaboets/repo-builder", Changes: []FieldChange{{Field: "permission", From: "push", To: "admin"}}},
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
			{ID: 99, Username: "octocat", Email: "octocat@example.com"},
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
	if len(report.Actions) != 0 {
		t.Fatalf("expected no actions, got %#v", report.Actions)
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
				Name:         "repo-builder",
				Visibility:   "private",
				Description:  "CLI",
				Homepage:     "https://example.com/repo-builder",
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
  - name: repo-builder
    visibility: private
    description: "CLI"
    homepage: "https://example.com/repo-builder"
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
				Name:         "repo-builder",
				Visibility:   "public",
				Description:  "CLI",
				Homepage:     "https://example.com/repo-builder",
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
  - name: repo-builder
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
				Name:         "repo-builder",
				Visibility:   "public",
				Description:  "CLI",
				Homepage:     "https://example.com/repo-builder",
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
  - name: repo-builder
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
		ResourceID:   "orang-gaboets/repo-builder",
		Executable:   true,
		Message:      "update repository orang-gaboets/repo-builder",
		Changes: []FieldChange{
			{Field: "allow_forking", From: true, To: false},
			{Field: "archived", From: true, To: false},
			{Field: "description", From: "CLI", To: ""},
			{Field: "homepage", From: "https://example.com/repo-builder", To: ""},
			{Field: "is_template", From: true, To: false},
		},
	}}
	if !reflect.DeepEqual(report.Actions, want) {
		t.Fatalf("unexpected actions:\n got %#v\nwant %#v", report.Actions, want)
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
