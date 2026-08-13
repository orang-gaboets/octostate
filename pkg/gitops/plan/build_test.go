package plan

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/internal/testconfig"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
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
		Actual: &state.OrganizationState{
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

func TestBuildResolvesUnnormalizedRepositoryOwners(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: " org-a ",
		Repositories: []config.RepositorySpec{{
			Name:       "service",
			Visibility: "private",
		}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Repositories: []config.TeamRepositorySpec{{
				Owner:      " ORG-A ",
				Name:       "service",
				Permission: "push",
			}},
		}},
	}

	report, err := Build(context.Background(), Options{
		Desired: desired,
		Actual: &state.OrganizationState{
			Organization: "org-a",
			Repositories: []state.Repository{{Owner: "org-a", Name: "service", Visibility: "private"}},
			Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
			TeamRepositoryPermissions: []state.TeamRepositoryPermission{{
				TeamSlug: "platform", Owner: "org-a", Name: "service", Permission: "push",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if report.Summary.HasChanges {
		t.Fatalf("expected no drift for effective repository owners, got %#v", report.Actions)
	}
	if desired.Repositories[0].Owner != "" || desired.Teams[0].Repositories[0].Owner != " ORG-A " {
		t.Fatalf("Build mutated the desired config: %#v", desired)
	}
}

func TestBuildRejectsInvalidDesiredConfig(t *testing.T) {
	t.Parallel()

	_, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []config.RepositorySpec{{
				Owner:      "shared-platform",
				Name:       "octostate",
				Visibility: "private",
			}},
			Teams: []config.TeamSpec{{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
				Repositories: []config.TeamRepositorySpec{{
					Owner:      "other-org",
					Name:       "octostate-infra",
					Permission: "push",
				}},
			}},
		},
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
		},
	})

	assertValidationErrorHasIssue(t, err, "repositories[0].owner", config.ValidationIssueCodeRepositoryOwnerScope)
	assertValidationErrorHasIssue(t, err, "teams[0].repositories[0].owner", config.ValidationIssueCodeRepositoryOwnerScope)
}

func TestBuildPlansDeterministicReconciliationActions(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
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
					Name:        "Platform",
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
		Actual: &state.OrganizationState{
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
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := &Report{
		Organization: "orang-gaboets",
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
			{ResourceType: ActionResourceTypeRepository, Operation: ActionOperationDelete, ResourceID: "orang-gaboets/orphan-repo", Executable: false, Message: "repository orang-gaboets/orphan-repo exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationCreate, ResourceID: "fresh", Executable: true, Message: "create team fresh", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationUpdate, ResourceID: "platform", Executable: true, Message: "update team platform", Changes: []FieldChange{{Field: "description", From: "Old desc", To: "New desc"}, {Field: "name", From: "Platform Old", To: "Platform"}, {Field: "privacy", From: "closed", To: "secret"}}},
			{ResourceType: ActionResourceTypeTeam, Operation: ActionOperationDelete, ResourceID: "legacy", Executable: false, Message: "team legacy exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeOrganizationMember, Operation: ActionOperationCreate, ResourceID: "charlie", Executable: true, Message: "create organization member charlie", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeOrganizationMember, Operation: ActionOperationUpdate, ResourceID: "alice", Executable: true, Message: "update organization member alice", Changes: []FieldChange{{Field: "role", From: "member", To: "admin"}}},
			{ResourceType: ActionResourceTypeOrganizationMember, Operation: ActionOperationDelete, ResourceID: "bob", Executable: false, Message: "organization member bob exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "email:invite@example.com", Executable: true, Message: "create organization invite email:invite@example.com", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "user_id:42", Executable: true, Message: "create organization invite user_id:42", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationCreate, ResourceID: "username:invite-user", Executable: true, Message: "create organization invite username:invite-user", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeInvite, Operation: ActionOperationRemove, ResourceID: "email:orphan@example.com", Executable: false, Message: "pending invitation email:orphan@example.com exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationCreate, ResourceID: "platform/charlie", Executable: false, Message: "team membership platform/charlie requires organization member charlie to exist first", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationUpdate, ResourceID: "platform/alice", Executable: true, Message: "update team membership platform/alice", Changes: []FieldChange{{Field: "role", From: "member", To: "maintainer"}}},
			{ResourceType: ActionResourceTypeTeamMember, Operation: ActionOperationRemove, ResourceID: "platform/bob", Executable: false, Message: "team membership platform/bob exists in live state but is not declared in desired config", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationCreate, ResourceID: "platform/orang-gaboets/repo-extra", Executable: false, Message: "team repository permission platform/orang-gaboets/repo-extra requires repository orang-gaboets/repo-extra to be available: repository orang-gaboets/repo-extra is absent from live and desired state", Changes: []FieldChange{}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationUpdate, ResourceID: "platform/orang-gaboets/octostate", Executable: false, Message: "team repository permission platform/orang-gaboets/octostate requires repository orang-gaboets/octostate to be available: repository orang-gaboets/octostate is absent from live and desired state", Changes: []FieldChange{{Field: "permission", From: "push", To: "admin"}}},
			{ResourceType: ActionResourceTypeTeamRepositoryPermission, Operation: ActionOperationRemove, ResourceID: "platform/orang-gaboets/repo-old", Executable: false, Message: "team repository permission platform/orang-gaboets/repo-old exists in live state but is not declared in desired config", Changes: []FieldChange{}},
		},
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("unexpected report:\n got %#v\nwant %#v", report, want)
	}
}

func TestBuildRejectsInviteThatDuplicatesDesiredMemberByUserID(t *testing.T) {
	t.Parallel()

	userService := &userServiceStub{
		getByIDFunc: func(_ context.Context, id int64) (*gh.User, *gh.Response, error) {
			if id != 99 {
				t.Fatalf("unexpected user id lookup: got %d want 99", id)
			}
			return &gh.User{Login: githubpkg.Ptr("alice")}, &gh.Response{}, nil
		},
	}

	_, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: "alice", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Actual:      &state.OrganizationState{Organization: "orang-gaboets"},
		UserService: userService,
	})
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: got %v want %v", err, githubpkg.ErrInvalidFieldValue)
	}
	if err == nil || !strings.Contains(err.Error(), "duplicates a declared top-level member") {
		t.Fatalf("unexpected error text: %v", err)
	}
	if !reflect.DeepEqual(userService.calls, []int64{99}) {
		t.Fatalf("unexpected user lookups: got %#v want %#v", userService.calls, []int64{99})
	}
}

func TestBuildRejectsInviteThatDuplicatesUnnormalizedDesiredMemberByUserID(t *testing.T) {
	t.Parallel()

	userService := &userServiceStub{
		getByIDFunc: func(_ context.Context, id int64) (*gh.User, *gh.Response, error) {
			if id != 99 {
				t.Fatalf("unexpected user lookup id %d", id)
			}
			return &gh.User{Login: githubpkg.Ptr("alice")}, &gh.Response{}, nil
		},
	}

	_, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: " alice ", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Actual:      &state.OrganizationState{Organization: "orang-gaboets"},
		UserService: userService,
	})
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: got %v want %v", err, githubpkg.ErrInvalidFieldValue)
	}
	if err == nil || !strings.Contains(err.Error(), "duplicates a declared top-level member") {
		t.Fatalf("unexpected error text: %v", err)
	}
	if !reflect.DeepEqual(userService.calls, []int64{99}) {
		t.Fatalf("unexpected user lookups: got %#v want %#v", userService.calls, []int64{99})
	}
}

func TestBuildInviteUserIDWithoutDesiredMembersDoesNotLookupUser(t *testing.T) {
	t.Parallel()

	userService := &userServiceStub{
		getByIDFunc: func(_ context.Context, id int64) (*gh.User, *gh.Response, error) {
			t.Fatalf("unexpected user lookup id %d", id)
			return nil, nil, nil
		},
	}

	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Actual:      &state.OrganizationState{Organization: "orang-gaboets"},
		UserService: userService,
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(userService.calls) != 0 {
		t.Fatalf("unexpected user lookups: got %#v want none", userService.calls)
	}
	if len(report.Actions) != 1 || report.Actions[0].ResourceID != "user_id:99" {
		t.Fatalf("unexpected actions: %#v", report.Actions)
	}
}

func TestBuildRejectsInviteThatDuplicatesDesiredMemberByActualMemberIDWithoutLookup(t *testing.T) {
	t.Parallel()

	userService := &userServiceStub{
		getByIDFunc: func(_ context.Context, id int64) (*gh.User, *gh.Response, error) {
			t.Fatalf("unexpected user lookup id %d", id)
			return nil, nil, nil
		},
	}

	_, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: "alice", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{UserID: presentInt64(99)},
			},
		},
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Members: []state.OrganizationMember{
				{ID: 99, Username: "alice", Role: "member"},
			},
		},
		UserService: userService,
	})
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: got %v want %v", err, githubpkg.ErrInvalidFieldValue)
	}
	if err == nil || !strings.Contains(err.Error(), "duplicates a declared top-level member") {
		t.Fatalf("unexpected error text: %v", err)
	}
	if len(userService.calls) != 0 {
		t.Fatalf("unexpected user lookups: got %#v want none", userService.calls)
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

func TestBuildSkipsAllowForkingDiffForPrivateRepository(t *testing.T) {
	t.Parallel()

	desired := testconfig.LoadDesiredConfig(t, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: private
    allow_forking: false
teams: []
invites: []
`)

	report, err := Build(context.Background(), Options{
		Desired: desired,
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Repositories: []state.Repository{{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "private",
				AllowForking: true,
			}},
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if len(report.Actions) != 0 {
		t.Fatalf("expected no actions for private allow_forking drift, got %#v", report.Actions)
	}
}

func TestBuildRepositoryOmittedOptionalFieldsProduceNoDiff(t *testing.T) {
	t.Parallel()

	desired := testconfig.LoadDesiredConfig(t, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: public
    topics: [gitops]
teams: []
invites: []
`)

	report, err := Build(context.Background(), Options{
		Desired: desired,
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Repositories: []state.Repository{{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "public",
				Description:  "CLI",
				Homepage:     "https://example.com/octostate",
				Topics:       []string{"gitops"},
				AllowForking: true,
				Archived:     true,
				IsTemplate:   true,
			}},
		},
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

	report, err := Build(context.Background(), Options{
		Desired: desired,
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Repositories: []state.Repository{{
				Owner:        "orang-gaboets",
				Name:         "octostate",
				Visibility:   "public",
				Description:  "CLI",
				Homepage:     "https://example.com/octostate",
				Topics:       []string{"gitops"},
				AllowForking: true,
				Archived:     true,
				IsTemplate:   true,
			}},
		},
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

func TestBuildOrdersRepositoryUpdateBeforeCreateWhenTemplateStateChangesInSamePlan(t *testing.T) {
	t.Parallel()

	templateRepo := config.RepositorySpec{
		Owner:      "orang-gaboets",
		Name:       "zzz-template",
		Visibility: "private",
	}
	templateRepo.SetManagedIsTemplate(true)

	report, err := Build(context.Background(), Options{
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
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Repositories: []state.Repository{{
				Owner:      "orang-gaboets",
				Name:       "zzz-template",
				Visibility: "private",
				IsTemplate: false,
			}},
		},
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

func TestBuildRepositoryCreateWithoutTemplateIsNonExecutable(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []config.RepositorySpec{{
				Owner:      "orang-gaboets",
				Name:       "new-repo",
				Visibility: "private",
			}},
		},
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := Action{
		ResourceType: ActionResourceTypeRepository,
		Operation:    ActionOperationCreate,
		ResourceID:   "orang-gaboets/new-repo",
		Executable:   false,
		Message:      "repository orang-gaboets/new-repo cannot be created because template configuration is missing",
		Changes:      []FieldChange{},
	}
	if len(report.Actions) != 1 {
		t.Fatalf("expected one action, got %#v", report.Actions)
	}
	if !reflect.DeepEqual(report.Actions[0], want) {
		t.Fatalf("unexpected action:\n got %#v\nwant %#v", report.Actions[0], want)
	}
	if report.Summary.ExecutableActions != 0 || report.Summary.NonExecutableActions != 1 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
}

func TestBuildTeamRepositoryPermissionCreateIsExecutableWhenRepositoryIsCreatedInSamePlan(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
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
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Teams: []state.Team{{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := []Action{
		{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationCreate,
			ResourceID:   "orang-gaboets/OctoState",
			Executable:   true,
			Message:      "create repository orang-gaboets/OctoState",
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

func TestBuildTeamRepositoryPermissionCreateIsNonExecutableWhenRepositoryCannotBeCreated(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
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
		Actual: &state.OrganizationState{
			Organization: "orang-gaboets",
			Teams: []state.Team{{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := []Action{
		{
			ResourceType: ActionResourceTypeRepository,
			Operation:    ActionOperationCreate,
			ResourceID:   "orang-gaboets/octostate",
			Executable:   false,
			Message:      "repository orang-gaboets/octostate cannot be created because template configuration is missing",
			Changes:      []FieldChange{},
		},
		{
			ResourceType: ActionResourceTypeTeamRepositoryPermission,
			Operation:    ActionOperationCreate,
			ResourceID:   "platform/orang-gaboets/octostate",
			Executable:   false,
			Message:      "team repository permission platform/orang-gaboets/octostate requires repository orang-gaboets/octostate to be available: template configuration is missing",
			Changes:      []FieldChange{},
		},
	}
	if !reflect.DeepEqual(report.Actions, want) {
		t.Fatalf("unexpected actions:\n got %#v\nwant %#v", report.Actions, want)
	}
	if report.Summary.ExecutableActions != 0 || report.Summary.NonExecutableActions != 2 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
}

func TestBuildTeamRepositoryPermissionUpdateIsExecutableWhenRepositoryExistsLive(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
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
					Permission: "admin",
				}},
			}},
		},
		Actual: &state.OrganizationState{
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
			TeamRepositoryPermissions: []state.TeamRepositoryPermission{{
				TeamSlug:   "platform",
				Owner:      "orang-gaboets",
				Name:       "octostate",
				Permission: "push",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	want := []Action{
		{
			ResourceType: ActionResourceTypeTeamRepositoryPermission,
			Operation:    ActionOperationUpdate,
			ResourceID:   "platform/orang-gaboets/octostate",
			Executable:   true,
			Message:      "update team repository permission platform/orang-gaboets/octostate",
			Changes:      []FieldChange{{Field: "permission", From: "push", To: "admin"}},
		},
	}
	if !reflect.DeepEqual(report.Actions, want) {
		t.Fatalf("unexpected actions:\n got %#v\nwant %#v", report.Actions, want)
	}
	if report.Summary.ExecutableActions != 1 || report.Summary.NonExecutableActions != 0 {
		t.Fatalf("unexpected summary: %#v", report.Summary)
	}
}

func TestBuildRepositoryTopicsTreatDuplicatesAsSetEquivalent(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []config.RepositorySpec{
				{
					Owner:        "orang-gaboets",
					Name:         "octostate",
					Visibility:   "private",
					Description:  "CLI",
					Homepage:     "https://example.com/octostate",
					Topics:       []string{"gitops", "go", "gitops"},
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
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if report.Summary.Actions != 0 || len(report.Actions) != 0 {
		t.Fatalf("expected duplicate desired topics to be treated as set-equivalent, got %#v", report)
	}
}

func TestBuildActionsKeepsFixedPhaseOrder(t *testing.T) {
	t.Parallel()

	actions, err := (planner{
		ctx: context.Background(),
		desired: config.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []config.OrganizationMemberSpec{
				{Username: "alice", Role: "member"},
			},
			Invites: []config.InviteSpec{
				{Username: presentString("invite-user")},
			},
			Repositories: []config.RepositorySpec{
				{
					Owner:      "orang-gaboets",
					Name:       "octostate",
					Visibility: "private",
				},
			},
			Teams: []config.TeamSpec{
				{
					Slug:    "platform",
					Name:    "Platform",
					Privacy: "closed",
					Members: []config.TeamMemberSpec{
						{Username: "alice", Role: "member"},
					},
					Repositories: []config.TeamRepositorySpec{
						{Owner: "orang-gaboets", Name: "octostate", Permission: "push"},
					},
				},
			},
		},
		actual:         &state.OrganizationState{Organization: "orang-gaboets"},
		userLoginsByID: map[int64]string{},
	}).buildActions()
	if err != nil {
		t.Fatalf("buildActions returned error: %v", err)
	}

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
			member: state.OrganizationMember{ID: 1, Username: "OctoCat", Role: "member"},
		},
		{
			name:   "email",
			invite: config.InviteSpec{Email: presentString("octocat@example.com")},
			member: state.OrganizationMember{ID: 1, Username: "octocat", Role: "member", Email: "OctoCat@example.com"},
		},
		{
			name:   "user id",
			invite: config.InviteSpec{UserID: presentInt64(99)},
			member: state.OrganizationMember{ID: 99, Username: "octocat", Role: "member"},
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
			want := []Action{{
				ResourceType: ActionResourceTypeOrganizationMember,
				Operation:    ActionOperationDelete,
				ResourceID:   tt.member.Username,
				Executable:   false,
				Message:      "organization member " + tt.member.Username + " exists in live state but is not declared in desired config",
				Changes:      []FieldChange{},
			}}
			if !reflect.DeepEqual(report.Actions, want) {
				t.Fatalf("unexpected plan actions:\n got %#v\nwant %#v", report.Actions, want)
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
					Name:         "octostate",
					Template:     config.TemplateSpec{Owner: "templates", Name: "new-template", IncludeAllBranches: true},
					Visibility:   "private",
					Description:  "CLI",
					Homepage:     "https://example.com/octostate",
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
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if report.Summary.Actions != 0 || len(report.Actions) != 0 {
		t.Fatalf("expected template-only diff to be ignored, got %#v", report)
	}
}

func TestBuildDoesNotOrderExistingConsumerAfterReferencedTemplate(t *testing.T) {
	t.Parallel()

	template := config.RepositorySpec{Owner: "orang-gaboets", Name: "z-template", Visibility: "private", Template: config.TemplateSpec{Owner: "external", Name: "base"}}
	template.SetManagedIsTemplate(true)
	consumer := config.RepositorySpec{Owner: "orang-gaboets", Name: "a-consumer", Visibility: "private", Description: "new", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "z-template"}}
	consumer.SetManagedDescription("new")

	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{Organization: "orang-gaboets", Repositories: []config.RepositorySpec{consumer, template}},
		Actual: &state.OrganizationState{Organization: "orang-gaboets", Repositories: []state.Repository{{
			Owner: "orang-gaboets", Name: "a-consumer", Visibility: "private", Description: "old",
		}}},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	wantOrder := []string{"orang-gaboets/a-consumer", "orang-gaboets/z-template"}
	gotOrder := make([]string, len(report.Actions))
	for i, action := range report.Actions {
		gotOrder[i] = action.ResourceID
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("existing consumer should not depend on desired template: got %#v want %#v", gotOrder, wantOrder)
	}
	if report.Actions[0].Operation != ActionOperationUpdate {
		t.Fatalf("expected existing consumer update first, got %#v", report.Actions[0])
	}
}

func TestBuildReadyRepositoryActionsIgnoreNoOpTemplateSources(t *testing.T) {
	t.Parallel()

	source := config.RepositorySpec{Owner: "orang-gaboets", Name: "z-template", Visibility: "private", Template: config.TemplateSpec{Owner: "external", Name: "base"}}
	source.SetManagedIsTemplate(true)
	consumer := config.RepositorySpec{Owner: "orang-gaboets", Name: "a-consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "z-template"}}
	independent := config.RepositorySpec{Owner: "orang-gaboets", Name: "b-independent", Visibility: "private", Template: config.TemplateSpec{Owner: "external", Name: "base"}}

	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{Organization: "orang-gaboets", Repositories: []config.RepositorySpec{consumer, independent, source}},
		Actual: &state.OrganizationState{Organization: "orang-gaboets", Repositories: []state.Repository{{
			Owner: "orang-gaboets", Name: "z-template", Visibility: "private", IsTemplate: true,
		}}},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	want := []string{"orang-gaboets/a-consumer", "orang-gaboets/b-independent"}
	got := make([]string, len(report.Actions))
	for i, action := range report.Actions {
		got[i] = action.ResourceID
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("no-op template source should not affect ready ordering: got %#v want %#v", got, want)
	}
}

func TestBuildReadyRepositoryActionsIgnoreUnrelatedTemplateSourceUpdates(t *testing.T) {
	t.Parallel()

	source := config.RepositorySpec{
		Owner:       "orang-gaboets",
		Name:        "z-template",
		Visibility:  "private",
		Description: "new description",
		Template:    config.TemplateSpec{Owner: "external", Name: "base"},
	}
	source.SetManagedDescription("new description")
	source.SetManagedIsTemplate(true)
	consumer := config.RepositorySpec{Owner: "orang-gaboets", Name: "a-consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "z-template"}}
	independent := config.RepositorySpec{Owner: "orang-gaboets", Name: "b-independent", Visibility: "private", Template: config.TemplateSpec{Owner: "external", Name: "base"}}

	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{Organization: "orang-gaboets", Repositories: []config.RepositorySpec{consumer, independent, source}},
		Actual: &state.OrganizationState{Organization: "orang-gaboets", Repositories: []state.Repository{
			{Owner: "orang-gaboets", Name: "z-template", Visibility: "private", Description: "old description", IsTemplate: true},
		}},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	want := []string{"orang-gaboets/a-consumer", "orang-gaboets/b-independent", "orang-gaboets/z-template"}
	got := make([]string, len(report.Actions))
	for i, action := range report.Actions {
		got[i] = action.ResourceID
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unrelated template source update should not gate consumers: got %#v want %#v", got, want)
	}
}

func TestBuildOrdersManagedTemplateBeforeItsConsumer(t *testing.T) {
	t.Parallel()

	template := config.RepositorySpec{Owner: "orang-gaboets", Name: "template", Visibility: "private", Template: config.TemplateSpec{Owner: "external", Name: "base"}}
	template.SetManagedIsTemplate(true)
	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{Organization: "orang-gaboets", Repositories: []config.RepositorySpec{
			{Owner: "orang-gaboets", Name: "consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "template"}},
			template,
		}},
		Actual: &state.OrganizationState{Organization: "orang-gaboets"},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if got, want := report.Actions[0].ResourceID, "orang-gaboets/template"; got != want {
		t.Fatalf("template action order: got %q want %q; actions=%#v", got, want, report.Actions)
	}
	if got, want := report.Actions[1].ResourceID, "orang-gaboets/consumer"; got != want {
		t.Fatalf("consumer action order: got %q want %q; actions=%#v", got, want, report.Actions)
	}
}

func TestBuildDoesNotTreatExternalTemplateAsManagedDependency(t *testing.T) {
	t.Parallel()

	template := config.RepositorySpec{Owner: "orang-gaboets", Name: "template", Visibility: "private", Template: config.TemplateSpec{Owner: "external", Name: "base"}}
	template.SetManagedIsTemplate(true)
	report, err := Build(context.Background(), Options{
		Desired: config.OrganizationConfig{Organization: "orang-gaboets", Repositories: []config.RepositorySpec{
			{Owner: "orang-gaboets", Name: "consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "aaa-external", Name: "template"}},
			template,
		}},
		Actual: &state.OrganizationState{Organization: "orang-gaboets"},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	if got, want := report.Actions[0].ResourceID, "orang-gaboets/consumer"; got != want {
		t.Fatalf("external template consumer should retain normalized independent ordering: got %q want %q; actions=%#v", got, want, report.Actions)
	}
	if !report.Actions[0].Executable {
		t.Fatalf("external consumer should not inherit managed source availability: %#v", report.Actions[0])
	}
}

func TestBuildUsesFinalManagedTemplateState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		live        *bool
		declaration string
		executable  bool
	}{
		{"existing template omitted", boolPtr(true), "", true},
		{"existing template null", boolPtr(true), "is_template: null", true},
		{"existing template false", boolPtr(true), "is_template: false", false},
		{"existing template true", boolPtr(true), "is_template: true", true},
		{"existing non-template omitted", boolPtr(false), "", false},
		{"existing non-template null", boolPtr(false), "is_template: null", false},
		{"existing non-template false", boolPtr(false), "is_template: false", false},
		{"existing non-template true", boolPtr(false), "is_template: true", true},
		{"new template omitted", nil, "", false},
		{"new template null", nil, "is_template: null", false},
		{"new template false", nil, "is_template: false", false},
		{"new template true", nil, "is_template: true", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			declaration := ""
			if tt.declaration != "" {
				declaration = "\n    " + tt.declaration
			}
			desired := testconfig.LoadDesiredConfig(t, fmt.Sprintf("organization: orang-gaboets\nrepositories:\n  - name: source\n    visibility: private\n    template: {owner: external, name: base}%s\n  - name: consumer\n    visibility: private\n    template: {owner: orang-gaboets, name: source}\nteams: []\ninvites: []\n", declaration))
			actual := &state.OrganizationState{Organization: "orang-gaboets"}
			if tt.live != nil {
				actual.Repositories = []state.Repository{{Owner: "orang-gaboets", Name: "source", Visibility: "private", IsTemplate: *tt.live}}
			}
			report, err := Build(context.Background(), Options{Desired: desired, Actual: actual})
			if err != nil {
				t.Fatalf("Build returned error: %v", err)
			}
			found := false
			for _, action := range report.Actions {
				if action.ResourceID == "orang-gaboets/consumer" && action.Executable != tt.executable {
					t.Fatalf("consumer executable=%v want %v: %#v", action.Executable, tt.executable, report.Actions)
				}
				if action.ResourceID == "orang-gaboets/consumer" {
					found = true
				}
			}
			if !found {
				t.Fatalf("consumer action not found: %#v", report.Actions)
			}
		})
	}
}

func TestBuildManagedRepositoryDependencyGraph(t *testing.T) {
	t.Parallel()

	managedTemplate := func(owner, name, templateOwner, templateName string) config.RepositorySpec {
		repository := config.RepositorySpec{Owner: owner, Name: name, Visibility: "private", Template: config.TemplateSpec{Owner: templateOwner, Name: templateName}}
		repository.SetManagedIsTemplate(true)
		return repository
	}
	team := config.TeamSpec{Slug: "platform", Name: "Platform", Privacy: "closed", Repositories: []config.TeamRepositorySpec{{Owner: "orang-gaboets", Name: "consumer", Permission: "push"}}}
	liveTeam := state.Team{Slug: "platform", Name: "Platform", Privacy: "closed"}

	tests := []struct {
		name           string
		repositories   []config.RepositorySpec
		teams          []config.TeamSpec
		actualTeams    []state.Team
		wantOrder      []string
		wantExecutable map[string]bool
		wantMessage    map[string][]string
	}{
		{
			name: "three repository chain",
			repositories: []config.RepositorySpec{
				managedTemplate("orang-gaboets", "source", "external", "base"),
				managedTemplate("orang-gaboets", "middle", "orang-gaboets", "source"),
				{Owner: "orang-gaboets", Name: "consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "middle"}},
			},
			wantOrder: []string{"orang-gaboets/source", "orang-gaboets/middle", "orang-gaboets/consumer"},
			wantExecutable: map[string]bool{
				"orang-gaboets/source": true, "orang-gaboets/middle": true, "orang-gaboets/consumer": true,
			},
		},
		{
			name: "three repository chain in reversed yaml order",
			repositories: []config.RepositorySpec{
				{Owner: "orang-gaboets", Name: "consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "middle"}},
				managedTemplate("orang-gaboets", "middle", "orang-gaboets", "source"),
				managedTemplate("orang-gaboets", "source", "external", "base"),
			},
			wantOrder: []string{"orang-gaboets/source", "orang-gaboets/middle", "orang-gaboets/consumer"},
			wantExecutable: map[string]bool{
				"orang-gaboets/source": true, "orang-gaboets/middle": true, "orang-gaboets/consumer": true,
			},
		},
		{
			name: "normalized tie-break among ready repositories",
			repositories: []config.RepositorySpec{
				{Owner: "orang-gaboets", Name: "a-consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "z-template"}},
				managedTemplate("orang-gaboets", "z-template", "external", "base"),
				managedTemplate("orang-gaboets", "b-independent", "external", "base"),
			},
			wantOrder: []string{"orang-gaboets/b-independent", "orang-gaboets/z-template", "orang-gaboets/a-consumer"},
			wantExecutable: map[string]bool{
				"orang-gaboets/a-consumer": true, "orang-gaboets/z-template": true, "orang-gaboets/b-independent": true,
			},
		},
		{
			name: "normalized repository identities",
			repositories: []config.RepositorySpec{
				{Owner: "orang-gaboets", Name: "consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "ORANG-GABOETS", Name: "z-template"}},
				managedTemplate("ORANG-GABOETS", "Z-Template", "external", "base"),
			},
			wantOrder: []string{"orang-gaboets/Z-Template", "orang-gaboets/consumer"},
			wantExecutable: map[string]bool{
				"orang-gaboets/Z-Template": true, "orang-gaboets/consumer": true,
			},
		},
		{
			name: "self reference",
			repositories: []config.RepositorySpec{
				managedTemplate("orang-gaboets", "self", "orang-gaboets", "self"),
			},
			wantOrder:      []string{"orang-gaboets/self"},
			wantExecutable: map[string]bool{"orang-gaboets/self": false},
			wantMessage:    map[string][]string{"orang-gaboets/self": {"template dependency cycle", "orang-gaboets/self -> orang-gaboets/self"}},
		},
		{
			name: "cycle",
			repositories: []config.RepositorySpec{
				managedTemplate("orang-gaboets", "a", "orang-gaboets", "b"),
				managedTemplate("orang-gaboets", "b", "orang-gaboets", "a"),
			},
			wantOrder: []string{"orang-gaboets/a", "orang-gaboets/b"},
			wantExecutable: map[string]bool{
				"orang-gaboets/a": false, "orang-gaboets/b": false,
			},
			wantMessage: map[string][]string{
				"orang-gaboets/a": {"template dependency cycle", "orang-gaboets/a -> orang-gaboets/b -> orang-gaboets/a"},
				"orang-gaboets/b": {"template dependency cycle", "orang-gaboets/a -> orang-gaboets/b -> orang-gaboets/a"},
			},
		},
		{
			name: "cycle precedes lexically earlier downstream consumer",
			repositories: []config.RepositorySpec{
				managedTemplate("orang-gaboets", "z-a", "orang-gaboets", "z-b"),
				managedTemplate("orang-gaboets", "z-b", "orang-gaboets", "z-a"),
				{Owner: "orang-gaboets", Name: "a-consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "z-a"}},
				{Owner: "orang-gaboets", Name: "z-consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "z-a"}},
				{Owner: "orang-gaboets", Name: "a-grandchild", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "z-consumer"}},
			},
			wantOrder: []string{"orang-gaboets/z-a", "orang-gaboets/z-b", "orang-gaboets/a-consumer", "orang-gaboets/z-consumer", "orang-gaboets/a-grandchild"},
			wantExecutable: map[string]bool{
				"orang-gaboets/z-a": false, "orang-gaboets/z-b": false, "orang-gaboets/a-consumer": false,
				"orang-gaboets/z-consumer": false, "orang-gaboets/a-grandchild": false,
			},
			wantMessage: map[string][]string{
				"orang-gaboets/a-consumer":   {"required template orang-gaboets/z-a is unavailable", "orang-gaboets/z-a -> orang-gaboets/z-b -> orang-gaboets/z-a"},
				"orang-gaboets/a-grandchild": {"required template orang-gaboets/z-consumer is unavailable"},
			},
		},
		{
			name: "transitive cycle consumer and dependent team permission",
			repositories: []config.RepositorySpec{
				managedTemplate("orang-gaboets", "a", "orang-gaboets", "b"),
				managedTemplate("orang-gaboets", "b", "orang-gaboets", "a"),
				{Owner: "orang-gaboets", Name: "consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "a"}},
			},
			teams:       []config.TeamSpec{team},
			actualTeams: []state.Team{liveTeam},
			wantOrder:   []string{"orang-gaboets/a", "orang-gaboets/b", "orang-gaboets/consumer", "platform/orang-gaboets/consumer"},
			wantExecutable: map[string]bool{
				"orang-gaboets/a": false, "orang-gaboets/b": false, "orang-gaboets/consumer": false, "platform/orang-gaboets/consumer": false,
			},
			wantMessage: map[string][]string{
				"orang-gaboets/consumer":          {"required template orang-gaboets/a is unavailable", "orang-gaboets/a -> orang-gaboets/b -> orang-gaboets/a"},
				"platform/orang-gaboets/consumer": {"required template orang-gaboets/a is unavailable", "orang-gaboets/a -> orang-gaboets/b -> orang-gaboets/a"},
			},
		},
		{
			name: "unsatisfied source and dependent team permission",
			repositories: []config.RepositorySpec{
				{Owner: "orang-gaboets", Name: "consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "source"}},
				{Owner: "orang-gaboets", Name: "source", Visibility: "private", Template: config.TemplateSpec{Owner: "external", Name: "base"}},
			},
			teams:       []config.TeamSpec{team},
			actualTeams: []state.Team{liveTeam},
			wantOrder:   []string{"orang-gaboets/source", "orang-gaboets/consumer", "platform/orang-gaboets/consumer"},
			wantExecutable: map[string]bool{
				"orang-gaboets/source": true, "orang-gaboets/consumer": false, "platform/orang-gaboets/consumer": false,
			},
			wantMessage: map[string][]string{
				"orang-gaboets/consumer":          {"required template orang-gaboets/source is unavailable", "will not be a template"},
				"platform/orang-gaboets/consumer": {"required template orang-gaboets/source is unavailable", "will not be a template"},
			},
		},
		{
			name: "cross organization template remains reference only",
			repositories: []config.RepositorySpec{
				{Owner: "orang-gaboets", Name: "a-consumer", Visibility: "private", Template: config.TemplateSpec{Owner: "other-org", Name: "z-template"}},
				managedTemplate("orang-gaboets", "z-template", "external", "base"),
			},
			wantOrder: []string{"orang-gaboets/a-consumer", "orang-gaboets/z-template"},
			wantExecutable: map[string]bool{
				"orang-gaboets/a-consumer": true, "orang-gaboets/z-template": true,
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report, err := Build(context.Background(), Options{
				Desired: config.OrganizationConfig{Organization: "orang-gaboets", Repositories: tt.repositories, Teams: tt.teams},
				Actual:  &state.OrganizationState{Organization: "orang-gaboets", Teams: tt.actualTeams},
			})
			if err != nil {
				t.Fatalf("Build returned error: %v", err)
			}
			gotOrder := make([]string, len(report.Actions))
			for i, action := range report.Actions {
				gotOrder[i] = action.ResourceID
				if want, ok := tt.wantExecutable[action.ResourceID]; !ok {
					t.Fatalf("unexpected action %#v", action)
				} else if action.Executable != want {
					t.Fatalf("action %s executable=%v want %v: %#v", action.ResourceID, action.Executable, want, report.Actions)
				}
				for _, fragment := range tt.wantMessage[action.ResourceID] {
					if !strings.Contains(action.Message, fragment) {
						t.Fatalf("action %s message %q does not contain %q", action.ResourceID, action.Message, fragment)
					}
				}
			}
			if !reflect.DeepEqual(gotOrder, tt.wantOrder) {
				t.Fatalf("unexpected action order: got %#v want %#v", gotOrder, tt.wantOrder)
			}
		})
	}
}

func TestBuildIssue210Reproducer(t *testing.T) {
	t.Parallel()

	desired := testconfig.LoadDesiredConfig(t, `
organization: acme
repositories:
  - owner: acme
    name: a-consumer
    visibility: private
    template: {owner: acme, name: z-template}
  - owner: acme
    name: z-template
    visibility: private
    is_template: true
    template: {owner: shared-templates, name: base}
teams:
  - slug: platform
    name: Platform
    privacy: closed
    repositories:
      - owner: acme
        name: a-consumer
        permission: push
invites: []
`)
	report, err := Build(context.Background(), Options{
		Desired: desired,
		Actual:  &state.OrganizationState{Organization: "acme", Teams: []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}}},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	wantOrder := []string{"acme/z-template", "acme/a-consumer", "platform/acme/a-consumer"}
	gotOrder := make([]string, len(report.Actions))
	for i, action := range report.Actions {
		gotOrder[i] = action.ResourceID
		if !action.Executable {
			t.Fatalf("action should be executable: %#v", action)
		}
	}
	if !reflect.DeepEqual(gotOrder, wantOrder) {
		t.Fatalf("unexpected action order: got %#v want %#v", gotOrder, wantOrder)
	}
}

func TestBuildLeavesLiveOnlyTemplateReferencesToApplyPreflight(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Owner: "orang-gaboets", Name: "consumer", Visibility: "private",
			Template: config.TemplateSpec{Owner: "orang-gaboets", Name: "live-source"},
		}},
		Teams: []config.TeamSpec{{
			Slug: "platform", Name: "Platform", Privacy: "closed",
			Repositories: []config.TeamRepositorySpec{{Owner: "orang-gaboets", Name: "consumer", Permission: "push"}},
		}},
	}
	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{{Owner: "orang-gaboets", Name: "live-source", IsTemplate: false}},
		Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
	}

	report, err := Build(context.Background(), Options{Desired: desired, Actual: actual})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	for _, action := range report.Actions {
		switch action.ResourceID {
		case "orang-gaboets/consumer", "platform/orang-gaboets/consumer":
			if !action.Executable {
				t.Fatalf("reference-only live source should not gate planning: %#v", action)
			}
		}
	}
}

func TestBuildTeamRepositoryPermissionUsesRepositoryAvailability(t *testing.T) {
	t.Parallel()

	executableRepository := config.RepositorySpec{Owner: "orang-gaboets", Name: "target", Visibility: "private", Template: config.TemplateSpec{Owner: "external", Name: "base"}}
	tests := []struct {
		name         string
		owner        string
		desiredRepos []config.RepositorySpec
		actualRepos  []state.Repository
		executable   bool
	}{
		{name: "live-only same organization", owner: "orang-gaboets", actualRepos: []state.Repository{{Owner: "orang-gaboets", Name: "target"}}, executable: true},
		{name: "missing same organization", owner: "orang-gaboets", executable: false},
		{name: "executable same-plan create", owner: "orang-gaboets", desiredRepos: []config.RepositorySpec{executableRepository}, executable: true},
		{name: "non-executable same-plan create", owner: "orang-gaboets", desiredRepos: []config.RepositorySpec{{Owner: "orang-gaboets", Name: "target", Visibility: "private"}}, executable: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			report, err := Build(context.Background(), Options{
				Desired: config.OrganizationConfig{
					Organization: "orang-gaboets",
					Repositories: tt.desiredRepos,
					Teams: []config.TeamSpec{{
						Slug: "platform", Name: "Platform", Privacy: "closed",
						Repositories: []config.TeamRepositorySpec{{Owner: tt.owner, Name: "target", Permission: "push"}},
					}},
				},
				Actual: &state.OrganizationState{
					Organization: "orang-gaboets",
					Repositories: tt.actualRepos,
					Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
				},
			})
			if err != nil {
				t.Fatalf("Build returned error: %v", err)
			}
			permissionID := "platform/" + tt.owner + "/target"
			for _, action := range report.Actions {
				if action.ResourceID != permissionID {
					continue
				}
				if action.Executable != tt.executable {
					t.Fatalf("permission executable=%v want %v: %#v", action.Executable, tt.executable, report.Actions)
				}
				return
			}
			t.Fatalf("permission action %s not found: %#v", permissionID, report.Actions)
		})
	}
}

func boolPtr(value bool) *bool { return &value }

func presentString(value string) config.OptionalString {
	return config.OptionalString{Present: true, Value: value}
}

func presentInt64(value int64) config.OptionalInt64 {
	return config.OptionalInt64{Present: true, Value: value}
}

func assertValidationErrorHasIssue(t *testing.T, err error, wantPath string, wantCode config.ValidationIssueCode) {
	t.Helper()

	var validationErr *config.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *config.ValidationError, got %T (%v)", err, err)
	}

	for _, issue := range validationErr.Report.Errors {
		if issue.Path == wantPath && issue.Code == wantCode {
			return
		}
	}

	t.Fatalf("expected validation issue path=%q code=%q, got %#v", wantPath, wantCode, validationErr.Report.Errors)
}
