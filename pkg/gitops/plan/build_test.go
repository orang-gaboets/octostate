package plan

import (
	"context"
	"errors"
	"reflect"
	"testing"

	gh "github.com/google/go-github/v55/github"
	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

type userServiceStub struct {
	getByIDFunc func(context.Context, int64) (*gh.User, *gh.Response, error)
	calls       []int64
}

func (s *userServiceStub) Get(_ context.Context, _ string) (*gh.User, *gh.Response, error) {
	return &gh.User{}, &gh.Response{}, nil
}

func (s *userServiceStub) GetByID(ctx context.Context, id int64) (*gh.User, *gh.Response, error) {
	s.calls = append(s.calls, id)
	if s.getByIDFunc != nil {
		return s.getByIDFunc(ctx, id)
	}
	return &gh.User{}, &gh.Response{}, nil
}

func TestOptionsValidate(t *testing.T) {
	t.Parallel()

	valid := Options{
		Desired: config.OrganizationConfig{Organization: "orang-gaboets"},
		Actual:  &state.OrganizationState{Organization: "orang-gaboets"},
	}

	tests := []struct {
		name    string
		options Options
		wantErr error
	}{
		{
			name:    "missing organization",
			options: Options{Actual: &state.OrganizationState{}},
			wantErr: githubpkg.ErrMissingRequiredField,
		},
		{
			name:    "missing actual state",
			options: Options{Desired: config.OrganizationConfig{Organization: "orang-gaboets"}},
			wantErr: githubpkg.ErrMissingRequiredField,
		},
		{
			name: "organization mismatch",
			options: Options{
				Desired: config.OrganizationConfig{Organization: "orang-gaboets"},
				Actual:  &state.OrganizationState{Organization: "other-org"},
			},
			wantErr: githubpkg.ErrInvalidFieldValue,
		},
		{
			name:    "valid",
			options: valid,
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

func TestBuildNoOpWhenDesiredMatchesActual(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
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
		Actual: &state.OrganizationState{
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
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := &Report{
		Organization: "orang-gaboets",
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

func TestBuildPlansDeterministicReconciliationActions(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
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
		Actual: &state.OrganizationState{
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
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := &Report{
		Organization: "orang-gaboets",
		Summary: Summary{
			HasChanges:           true,
			Actions:              16,
			ExecutableActions:    11,
			NonExecutableActions: 5,
			CreateActions:        7,
			UpdateActions:        4,
			DeleteActions:        2,
			RemoveActions:        3,
		},
		Actions: []Action{
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationCreate, ResourceID: "orang-gaboets/new-repo", Executable: true, Message: "create repository orang-gaboets/new-repo", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationUpdate, ResourceID: "orang-gaboets/existing-repo", Executable: true, Message: "update repository orang-gaboets/existing-repo", Changes: []FieldChange{{Field: "allow_forking", From: true, To: false}, {Field: "archived", From: false, To: true}, {Field: "description", From: "Old desc", To: "New desc"}, {Field: "homepage", From: "", To: "https://example.com/repo-builder"}, {Field: "is_template", From: false, To: true}, {Field: "topics", From: []string{"gitops"}, To: []string{"gitops", "go"}}, {Field: "visibility", From: "public", To: "private"}}},
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationDelete, ResourceID: "orang-gaboets/orphan-repo", Executable: false, Message: "repository orang-gaboets/orphan-repo exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationCreate, ResourceID: "fresh", Executable: true, Message: "create team fresh", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationUpdate, ResourceID: "platform", Executable: true, Message: "update team platform", Changes: []FieldChange{{Field: "description", From: "Old desc", To: "New desc"}, {Field: "name", From: "Platform Old", To: "Platform New"}, {Field: "privacy", From: "closed", To: "secret"}}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationDelete, ResourceID: "legacy", Executable: false, Message: "team legacy exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "email:invite@example.com", Executable: true, Message: "create organization invite email:invite@example.com", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "user_id:42", Executable: true, Message: "create organization invite user_id:42", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "username:invite-user", Executable: true, Message: "create organization invite username:invite-user", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationRemove, ResourceID: "email:orphan@example.com", Executable: false, Message: "pending invitation email:orphan@example.com exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationCreate, ResourceID: "platform/charlie", Executable: true, Message: "add team membership platform/charlie", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationUpdate, ResourceID: "platform/alice", Executable: true, Message: "update team membership platform/alice", Changes: []FieldChange{{Field: "role", From: "member", To: "maintainer"}}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationRemove, ResourceID: "platform/bob", Executable: false, Message: "team membership platform/bob exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationCreate, ResourceID: "platform/orang-gaboets/repo-extra", Executable: true, Message: "create team repository permission platform/orang-gaboets/repo-extra", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationUpdate, ResourceID: "platform/orang-gaboets/repo-builder", Executable: true, Message: "update team repository permission platform/orang-gaboets/repo-builder", Changes: []FieldChange{{Field: "permission", From: "push", To: "admin"}}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationRemove, ResourceID: "platform/orang-gaboets/repo-old", Executable: false, Message: "team repository permission platform/orang-gaboets/repo-old exists in live state but is not declared in desired config", Changes: []FieldChange{}},
		},
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("unexpected report:\n got %#v\nwant %#v", report, want)
	}
}

func TestBuildInviteUserIDSatisfiedByPendingInviteUsesLookupCache(t *testing.T) {
	t.Parallel()

	users := &userServiceStub{
		getByIDFunc: func(_ context.Context, id int64) (*gh.User, *gh.Response, error) {
			if id != 99 {
				t.Fatalf("unexpected user lookup id %d", id)
			}
			return &gh.User{Login: githubpkg.Ptr("octocat")}, &gh.Response{}, nil
		},
	}

	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			PendingInvitations: []state.PendingInvitation{
				{ID: 10, Username: "octocat", Role: "direct_member", TeamSlugs: []string{}},
			},
		},
		UserService: users,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(users.calls) != 1 || users.calls[0] != 99 {
		t.Fatalf("expected one cached lookup for user 99, got %#v", users.calls)
	}
	if report.Summary.Actions != 0 || len(report.Actions) != 0 {
		t.Fatalf("expected no plan actions, got %#v", report)
	}
}

func TestBuildInviteLookupFailurePropagates(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("lookup failed")
	_, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			PendingInvitations: []state.PendingInvitation{
				{ID: 10, Username: "octocat", Role: "direct_member", TeamSlugs: []string{}},
			},
		},
		UserService: &userServiceStub{
			getByIDFunc: func(_ context.Context, _ int64) (*gh.User, *gh.Response, error) {
				return nil, nil, wantErr
			},
		},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected lookup failure, got %v", err)
	}
}

func TestBuildInviteSatisfiedByExistingMember(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		invite config.InviteSpec
		member state.OrganizationMember
	}{
		{
			name:   "username",
			invite: config.InviteSpec{Username: presentString("octocat")},
			member: state.OrganizationMember{ID: 1, Username: "OctoCat"},
		},
		{
			name:   "email",
			invite: config.InviteSpec{Email: presentString("octocat@example.com")},
			member: state.OrganizationMember{ID: 1, Username: "octocat", Email: "OctoCat@example.com"},
		},
		{
			name:   "user id",
			invite: config.InviteSpec{UserID: presentInt64(99)},
			member: state.OrganizationMember{ID: 99, Username: "octocat"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report, err := Build(context.Background(), Options{
				Desired: config.OrganizationConfig{
					Organization: "orang-gaboets",
					Invites:      []config.InviteSpec{tt.invite},
				},
				Actual: &state.OrganizationState{
					Organization: "orang-gaboets",
					Members:      []state.OrganizationMember{tt.member},
				},
			})
			if err != nil {
				t.Fatalf("Build returned error: %v", err)
			}
			if report.Summary.Actions != 0 || len(report.Actions) != 0 {
				t.Fatalf("expected no plan actions, got %#v", report)
			}
		})
	}
}

func TestBuildIgnoresTemplateConfigurationForExistingRepository(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []config.RepositorySpec{
				{
					Owner:        "orang-gaboets",
					Name:         "repo-builder",
					Template:     config.TemplateSpec{Owner: "templates", Name: "new-template", IncludeAllBranches: true},
					Visibility:   "private",
					Description:  "CLI",
					Homepage:     "https://example.com/repo-builder",
					Topics:       []string{"gitops", "go"},
					AllowForking: false,
					Archived:     false,
					IsTemplate:   false,
				},
			},
		},
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
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
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if report.Summary.Actions != 0 || len(report.Actions) != 0 {
		t.Fatalf("expected template-only diff to be ignored, got %#v", report)
	}
}

func presentString(value string) config.OptionalString {
	return config.OptionalString{Present: true, Value: value}
}

func presentInt64(value int64) config.OptionalInt64 {
	return config.OptionalInt64{Present: true, Value: value}
}
