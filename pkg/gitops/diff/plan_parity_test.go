package diff_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v88/github"
	gitopsgithub "github.com/orang-gaboets/octostate/pkg/github"
	ghusers "github.com/orang-gaboets/octostate/pkg/github/users"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsdiff "github.com/orang-gaboets/octostate/pkg/gitops/diff"
	"github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/snapshot"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

type parityUserService map[int64]string

func (parityUserService) Get(_ context.Context, username string) (*gh.User, *gh.Response, error) {
	return nil, nil, fmt.Errorf("no fixture for username %q", username)
}

func (s parityUserService) GetByID(_ context.Context, id int64) (*gh.User, *gh.Response, error) {
	login, ok := s[id]
	if !ok {
		return nil, nil, fmt.Errorf("no fixture for user ID %d", id)
	}
	return &gh.User{Login: &login}, nil, nil
}

type expectedAction struct {
	resourceType plan.ActionResourceType
	operation    plan.ActionOperation
	resourceID   string
	executable   bool
	changes      []plan.FieldChange
}

type semanticAction struct {
	resourceType string
	operation    string
	resourceID   string
	executable   bool
}

func TestPlanAndDiffParityForTeamMembershipDecisions(t *testing.T) {
	tests := []struct {
		name              string
		desiredMembers    []config.TeamMemberSpec
		actualMemberships []state.TeamMember
		want              []expectedAction
		wantUnresolved    []state.TeamMember
	}{
		{
			name:           "direct create",
			desiredMembers: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeTeamMember,
				operation:    plan.ActionOperationCreate,
				resourceID:   "platform/alice",
				executable:   true,
			}},
		},
		{
			name:           "direct role update",
			desiredMembers: []config.TeamMemberSpec{{Username: "alice", Role: "maintainer"}},
			actualMemberships: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginDirect,
			}},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeTeamMember,
				operation:    plan.ActionOperationUpdate,
				resourceID:   "platform/alice",
				executable:   true,
				changes:      []plan.FieldChange{{Field: "role", From: "member", To: "maintainer"}},
			}},
		},
		{
			name: "direct removal remains non-executable",
			actualMemberships: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginDirect,
			}},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeTeamMember,
				operation:    plan.ActionOperationRemove,
				resourceID:   "platform/alice",
				executable:   false,
			}},
		},
		{
			name:           "direct no-op",
			desiredMembers: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
			actualMemberships: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginDirect,
			}},
		},
		{
			name:           "unknown origin is unresolved when declared",
			desiredMembers: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
			actualMemberships: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginUnknown,
			}},
			wantUnresolved: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginUnknown,
			}},
		},
		{
			name: "unknown origin is unresolved when undeclared",
			actualMemberships: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginUnknown,
			}},
			wantUnresolved: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginUnknown,
			}},
		},
		{
			name:           "unknown origin with role mismatch stays unresolved",
			desiredMembers: []config.TeamMemberSpec{{Username: "alice", Role: "maintainer"}},
			actualMemberships: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginUnknown,
			}},
			wantUnresolved: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginUnknown,
			}},
		},
		{
			name:           "desired inherited-only membership keeps direct create semantics",
			desiredMembers: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
			actualMemberships: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginInherited,
			}},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeTeamMember,
				operation:    plan.ActionOperationCreate,
				resourceID:   "platform/alice",
				executable:   true,
			}},
		},
		{
			name: "undeclared inherited-only membership is not removed",
			actualMemberships: []state.TeamMember{{
				TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginInherited,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desired := teamParityDesired(tt.desiredMembers)
			actual := teamParityActual(tt.actualMemberships)
			assertPlanDiffParity(t, desired, actual, nil, nil, tt.want, tt.wantUnresolved)
		})
	}
}

func TestPlanAndDiffParityWithActionableAndUnresolvedMemberships(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "acme",
		Members: []config.OrganizationMemberSpec{
			{Username: "alice", Role: "member"},
			{Username: "bob", Role: "member"},
			{Username: "charlie", Role: "member"},
		},
		Teams: []config.TeamSpec{{
			Slug: "platform", Name: "Platform", Privacy: "closed",
			Members: []config.TeamMemberSpec{
				{Username: "alice", Role: "maintainer"},
				{Username: "bob", Role: "maintainer"},
				{Username: "charlie", Role: "member"},
			},
		}},
	}
	actual := state.OrganizationState{
		Organization: "acme",
		Members: []state.OrganizationMember{
			{Username: "alice", Role: "member"},
			{Username: "bob", Role: "member"},
			{Username: "charlie", Role: "member"},
		},
		Teams: []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{
			{TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginUnknown},
			{TeamSlug: "platform", Username: "bob", Role: "member", Origin: gitopsgithub.TeamMembershipOriginDirect},
		},
	}
	want := []expectedAction{
		{
			resourceType: plan.ActionResourceTypeTeamMember,
			operation:    plan.ActionOperationUpdate,
			resourceID:   "platform/bob",
			executable:   true,
			changes:      []plan.FieldChange{{Field: "role", From: "member", To: "maintainer"}},
		},
		{
			resourceType: plan.ActionResourceTypeTeamMember,
			operation:    plan.ActionOperationCreate,
			resourceID:   "platform/charlie",
			executable:   true,
		},
	}
	wantUnresolved := []state.TeamMember{{
		TeamSlug: "platform", Username: "alice", Role: "member", Origin: gitopsgithub.TeamMembershipOriginUnknown,
	}}

	assertPlanDiffParity(t, desired, actual, nil, nil, want, wantUnresolved)
}

func TestPlanAndDiffParityForOrganizationMemberDecisions(t *testing.T) {
	tests := []struct {
		name           string
		desired        config.OrganizationConfig
		actual         state.OrganizationState
		want           []expectedAction
		wantUnresolved []state.TeamMember
	}{
		{
			name:    "same-plan member create enables team membership",
			desired: teamParityDesired([]config.TeamMemberSpec{{Username: "alice", Role: "member"}}),
			actual: state.OrganizationState{
				Organization: "acme",
				Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
			},
			want: []expectedAction{
				{resourceType: plan.ActionResourceTypeOrganizationMember, operation: plan.ActionOperationCreate, resourceID: "alice", executable: true},
				{resourceType: plan.ActionResourceTypeTeamMember, operation: plan.ActionOperationCreate, resourceID: "platform/alice", executable: true},
			},
		},
		{
			name: "organization member role update",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "admin"}},
			},
			actual: state.OrganizationState{
				Organization: "acme",
				Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
			},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeOrganizationMember,
				operation:    plan.ActionOperationUpdate,
				resourceID:   "alice",
				executable:   true,
				changes:      []plan.FieldChange{{Field: "role", From: "member", To: "admin"}},
			}},
		},
		{
			name: "organization member no-op",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
			},
			actual: state.OrganizationState{
				Organization: "acme",
				Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPlanDiffParity(t, tt.desired, tt.actual, nil, nil, tt.want, tt.wantUnresolved)
		})
	}
}

func TestPlanAndDiffParityForTeamRepositoryPermissionAvailability(t *testing.T) {
	tests := []struct {
		name               string
		snapshotRepository bool
		desiredRepository  bool
		actualPermission   bool
		wantOperation      plan.ActionOperation
		wantExecutable     bool
	}{
		{name: "present repository create", snapshotRepository: true, wantOperation: plan.ActionOperationCreate, wantExecutable: true},
		{name: "present repository update", snapshotRepository: true, actualPermission: true, wantOperation: plan.ActionOperationUpdate, wantExecutable: true},
		{name: "desired-creatable repository create", desiredRepository: true, wantOperation: plan.ActionOperationCreate, wantExecutable: true},
		{name: "desired-creatable repository update", desiredRepository: true, actualPermission: true, wantOperation: plan.ActionOperationUpdate, wantExecutable: true},
		{name: "absent repository create is blocked", wantOperation: plan.ActionOperationCreate, wantExecutable: false},
		{name: "absent repository update is blocked", actualPermission: true, wantOperation: plan.ActionOperationUpdate, wantExecutable: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desiredRepositories := []config.RepositorySpec{}
			if tt.desiredRepository {
				desiredRepositories = append(desiredRepositories, config.RepositorySpec{
					Owner: "acme", Name: "repo", Visibility: "private",
				})
			}
			actualRepositories := []state.Repository{}
			if tt.snapshotRepository {
				actualRepositories = append(actualRepositories, state.Repository{
					Owner: "acme", Name: "repo", Visibility: "private",
				})
			}
			actualPermissions := []state.TeamRepositoryPermission{}
			if tt.actualPermission {
				actualPermissions = append(actualPermissions, state.TeamRepositoryPermission{
					TeamSlug: "platform", Owner: "acme", Name: "repo", Permission: "pull",
				})
			}
			desired := config.OrganizationConfig{
				Organization: "acme",
				Repositories: desiredRepositories,
				Teams: []config.TeamSpec{{
					Slug: "platform", Name: "Platform", Privacy: "closed",
					Repositories: []config.TeamRepositorySpec{{Owner: "acme", Name: "repo", Permission: "push"}},
				}},
			}
			actual := state.OrganizationState{
				Organization:              "acme",
				Repositories:              actualRepositories,
				Teams:                     []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
				TeamRepositoryPermissions: actualPermissions,
			}

			want := []expectedAction{{
				resourceType: plan.ActionResourceTypeTeamRepositoryPermission,
				operation:    tt.wantOperation,
				resourceID:   "platform/acme/repo",
				executable:   tt.wantExecutable,
			}}
			if tt.snapshotRepository && !tt.desiredRepository {
				want = append(want, expectedAction{
					resourceType: plan.ActionResourceTypeRepository,
					operation:    plan.ActionOperationDelete,
					resourceID:   "acme/repo",
					executable:   false,
				})
			} else if tt.desiredRepository && !tt.snapshotRepository {
				want = append(want, expectedAction{
					resourceType: plan.ActionResourceTypeRepository,
					operation:    plan.ActionOperationCreate,
					resourceID:   "acme/repo",
					executable:   true,
				})
			}
			if tt.actualPermission {
				for i := range want {
					if want[i].resourceType == plan.ActionResourceTypeTeamRepositoryPermission {
						want[i].changes = []plan.FieldChange{{Field: "permission", From: "pull", To: "push"}}
					}
				}
			}
			assertPlanDiffParity(t, desired, actual, nil, nil, want, nil)
		})
	}
}

func TestPlanAndDiffParityForTeamRepositoryPermissionNoOpAndRemoval(t *testing.T) {
	tests := []struct {
		name    string
		desired config.OrganizationConfig
		actual  state.OrganizationState
		want    []expectedAction
	}{
		{
			name: "matching permission is a no-op",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Repositories: []config.RepositorySpec{{Owner: "acme", Name: "repo", Visibility: "private"}},
				Teams: []config.TeamSpec{{
					Slug: "platform", Name: "Platform", Privacy: "closed",
					Repositories: []config.TeamRepositorySpec{{Owner: "acme", Name: "repo", Permission: "pull"}},
				}},
			},
			actual: state.OrganizationState{
				Organization:              "acme",
				Repositories:              []state.Repository{{Owner: "acme", Name: "repo", Visibility: "private"}},
				Teams:                     []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
				TeamRepositoryPermissions: []state.TeamRepositoryPermission{{TeamSlug: "platform", Owner: "acme", Name: "repo", Permission: "pull"}},
			},
		},
		{
			name: "undeclared permission remains a non-executable removal",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Teams:        []config.TeamSpec{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
			},
			actual: state.OrganizationState{
				Organization:              "acme",
				Repositories:              []state.Repository{{Owner: "acme", Name: "repo", Visibility: "private"}},
				Teams:                     []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
				TeamRepositoryPermissions: []state.TeamRepositoryPermission{{TeamSlug: "platform", Owner: "acme", Name: "repo", Permission: "pull"}},
			},
			want: []expectedAction{
				{resourceType: plan.ActionResourceTypeRepository, operation: plan.ActionOperationDelete, resourceID: "acme/repo", executable: false},
				{resourceType: plan.ActionResourceTypeTeamRepositoryPermission, operation: plan.ActionOperationRemove, resourceID: "platform/acme/repo", executable: false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPlanDiffParity(t, tt.desired, tt.actual, nil, nil, tt.want, nil)
		})
	}
}

func TestPlanAndDiffParityForInvitationIdentity(t *testing.T) {
	tests := []struct {
		name                  string
		desired               config.OrganizationConfig
		actual                state.OrganizationState
		userLoginsByID        map[int64]string
		resolvedIDsByUsername map[string]int64
		want                  []expectedAction
	}{
		{
			name: "username invite satisfied by pending invitation",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{Username: config.OptionalString{Present: true, Value: "octocat"}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{
				Organization:       "acme",
				PendingInvitations: []state.PendingInvitation{{Username: "OctoCat", Role: "direct_member"}},
			},
		},
		{
			name: "email invite satisfied by pending invitation",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{Email: config.OptionalString{Present: true, Value: "invite@example.com"}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{
				Organization:       "acme",
				PendingInvitations: []state.PendingInvitation{{Email: "INVITE@example.com", Role: "direct_member"}},
			},
		},
		{
			name: "username invite satisfied by existing member",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{Username: config.OptionalString{Present: true, Value: "OCTOCAT"}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{
				Organization: "acme",
				Members:      []state.OrganizationMember{{ID: 42, Username: "octocat", Role: "member"}},
			},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeOrganizationMember,
				operation:    plan.ActionOperationDelete,
				resourceID:   "octocat",
				executable:   false,
			}},
		},
		{
			name: "email invite satisfied by existing member",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{Email: config.OptionalString{Present: true, Value: "INVITE@example.com"}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{
				Organization: "acme",
				Members:      []state.OrganizationMember{{ID: 42, Username: "octocat", Email: "invite@EXAMPLE.com", Role: "member"}},
			},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeOrganizationMember,
				operation:    plan.ActionOperationDelete,
				resourceID:   "octocat",
				executable:   false,
			}},
		},
		{
			name: "user ID invite satisfied by existing member",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{UserID: config.OptionalInt64{Present: true, Value: 42}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{
				Organization: "acme",
				Members:      []state.OrganizationMember{{ID: 42, Username: "octocat", Role: "member"}},
			},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeOrganizationMember,
				operation:    plan.ActionOperationDelete,
				resourceID:   "octocat",
				executable:   false,
			}},
		},
		{
			name: "unsatisfied username invite is created",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{Username: config.OptionalString{Present: true, Value: "octocat"}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{Organization: "acme"},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeInvite,
				operation:    plan.ActionOperationCreate,
				resourceID:   "username:octocat",
				executable:   true,
			}},
		},
		{
			name: "unsatisfied email invite is created",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{Email: config.OptionalString{Present: true, Value: "invite@example.com"}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{Organization: "acme"},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeInvite,
				operation:    plan.ActionOperationCreate,
				resourceID:   "email:invite@example.com",
				executable:   true,
			}},
		},
		{
			name: "user ID invite is not satisfied by a different existing member",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{UserID: config.OptionalInt64{Present: true, Value: 42}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{
				Organization: "acme",
				Members:      []state.OrganizationMember{{ID: 43, Username: "octocat", Role: "member"}},
			},
			want: []expectedAction{
				{resourceType: plan.ActionResourceTypeOrganizationMember, operation: plan.ActionOperationDelete, resourceID: "octocat", executable: false},
				{resourceType: plan.ActionResourceTypeInvite, operation: plan.ActionOperationCreate, resourceID: "user_id:42", executable: true},
			},
		},
		{
			name: "user ID invite uses equivalent lookup evidence",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{UserID: config.OptionalInt64{Present: true, Value: 42}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{
				Organization:       "acme",
				PendingInvitations: []state.PendingInvitation{{Username: "octocat", Role: "direct_member"}},
			},
			userLoginsByID:        map[int64]string{42: "octocat"},
			resolvedIDsByUsername: map[string]int64{"octocat": 42},
		},
		{
			name: "unsatisfied user ID invite retains its identity",
			desired: config.OrganizationConfig{
				Organization: "acme",
				Invites:      []config.InviteSpec{{UserID: config.OptionalInt64{Present: true, Value: 42}, Role: "direct_member"}},
			},
			actual: state.OrganizationState{Organization: "acme"},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeInvite,
				operation:    plan.ActionOperationCreate,
				resourceID:   "user_id:42",
				executable:   true,
			}},
		},
		{
			name:    "undeclared pending invitation remains non-executable removal",
			desired: config.OrganizationConfig{Organization: "acme"},
			actual: state.OrganizationState{
				Organization:       "acme",
				PendingInvitations: []state.PendingInvitation{{Email: "orphan@example.com", Role: "direct_member"}},
			},
			want: []expectedAction{{
				resourceType: plan.ActionResourceTypeInvite,
				operation:    plan.ActionOperationRemove,
				resourceID:   "email:orphan@example.com",
				executable:   false,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPlanDiffParity(t, tt.desired, tt.actual, tt.userLoginsByID, tt.resolvedIDsByUsername, tt.want, nil)
		})
	}
}

func TestPlanAndDiffRejectInviteThatDuplicatesDesiredMemberByUserID(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "acme",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Invites:      []config.InviteSpec{{UserID: config.OptionalInt64{Present: true, Value: 42}, Role: "direct_member"}},
	}
	actual := normalizeParityActual(state.OrganizationState{Organization: "acme"})
	userService := parityUserService{42: "alice"}
	actualSnapshot := snapshot.NewActualSnapshot(time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC), &actual)
	actualSnapshot.ResolvedInviteUserIDsByUsername = map[string]int64{"alice": 42}

	_, planErr := plan.Build(context.Background(), plan.Options{
		Desired:     config.NormalizeDesiredState(desired),
		Actual:      &actual,
		UserService: userService,
	})
	_, diffErr := gitopsdiff.Build(gitopsdiff.Options{
		Desired:  config.NormalizeDesiredState(desired),
		Snapshot: &actualSnapshot,
	})

	if !errors.Is(planErr, gitopsgithub.ErrInvalidFieldValue) {
		t.Errorf("plan.Build error = %v, want ErrInvalidFieldValue", planErr)
	}
	if !errors.Is(diffErr, gitopsgithub.ErrInvalidFieldValue) {
		t.Errorf("diff.Build error = %v, want ErrInvalidFieldValue", diffErr)
	}
}

func teamParityDesired(members []config.TeamMemberSpec) config.OrganizationConfig {
	return config.OrganizationConfig{
		Organization: "acme",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Teams: []config.TeamSpec{{
			Slug: "platform", Name: "Platform", Privacy: "closed", Members: members,
		}},
	}
}

func teamParityActual(members []state.TeamMember) state.OrganizationState {
	return state.OrganizationState{
		Organization: "acme",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
		Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers:  members,
	}
}

func assertPlanDiffParity(
	t *testing.T,
	desired config.OrganizationConfig,
	actual state.OrganizationState,
	userLoginsByID map[int64]string,
	resolvedIDsByUsername map[string]int64,
	want []expectedAction,
	wantUnresolved []state.TeamMember,
) {
	t.Helper()

	normalizedActual := normalizeParityActual(actual)
	desiredForPlan := config.NormalizeDesiredState(desired)
	desiredForDiff := config.NormalizeDesiredState(desired)

	var userService ghusers.Service
	if userLoginsByID != nil {
		userService = parityUserService(userLoginsByID)
	}
	planReport, err := plan.Build(context.Background(), plan.Options{
		Desired:     desiredForPlan,
		Actual:      &normalizedActual,
		UserService: userService,
	})
	if err != nil {
		t.Fatalf("plan.Build returned error: %v", err)
	}

	actualSnapshot := snapshot.NewActualSnapshot(time.Date(2026, 11, 28, 0, 0, 0, 0, time.UTC), &normalizedActual)
	actualSnapshot.ResolvedInviteUserIDsByUsername = cloneResolvedIDs(resolvedIDsByUsername)
	diffReport, err := gitopsdiff.Build(gitopsdiff.Options{
		Desired:  desiredForDiff,
		Snapshot: &actualSnapshot,
	})
	if err != nil {
		t.Fatalf("diff.Build returned error: %v", err)
	}

	wantActions := expectedSemanticActions(want)
	planActions := semanticActions(planReport.Actions)
	diffActions := semanticActions(diffReport.Actions)
	if !reflect.DeepEqual(planActions, wantActions) {
		t.Errorf("plan semantic actions = %#v, want %#v", planActions, wantActions)
	}
	if !reflect.DeepEqual(diffActions, wantActions) {
		t.Errorf("diff semantic actions = %#v, want %#v", diffActions, wantActions)
	}
	if !reflect.DeepEqual(planActions, diffActions) {
		t.Errorf("plan/diff semantic action multisets differ:\nplan: %#v\ndiff: %#v", planActions, diffActions)
	}

	assertExpectedChanges(t, "plan", planReport.Actions, want)
	assertExpectedChanges(t, "diff", diffReport.Actions, want)
	wantUnresolved = normalizedUnresolved(wantUnresolved)
	assertUnresolvedMemberships(t, "plan", planReport.UnresolvedTeamMemberships, planReport.Summary, wantUnresolved, len(want) > 0)
	assertUnresolvedMemberships(t, "diff", diffReport.UnresolvedTeamMemberships, diffReport.Summary, wantUnresolved, len(want) > 0)
	assertActionSummary(t, "plan", planReport.Summary, want)
	assertActionSummary(t, "diff", diffReport.Summary, want)
}

func normalizeParityActual(actual state.OrganizationState) state.OrganizationState {
	actual.Members = slices.Clone(actual.Members)
	actual.PendingInvitations = slices.Clone(actual.PendingInvitations)
	for i := range actual.PendingInvitations {
		actual.PendingInvitations[i].TeamSlugs = slices.Clone(actual.PendingInvitations[i].TeamSlugs)
	}
	actual.Repositories = slices.Clone(actual.Repositories)
	for i := range actual.Repositories {
		actual.Repositories[i].Topics = slices.Clone(actual.Repositories[i].Topics)
	}
	actual.Teams = slices.Clone(actual.Teams)
	actual.TeamMembers = slices.Clone(actual.TeamMembers)
	actual.TeamRepositoryPermissions = slices.Clone(actual.TeamRepositoryPermissions)
	actual.Normalize()
	return actual
}

func cloneResolvedIDs(values map[string]int64) map[string]int64 {
	if values == nil {
		return nil
	}
	cloned := make(map[string]int64, len(values))
	for username, id := range values {
		cloned[username] = id
	}
	return cloned
}

func expectedSemanticActions(actions []expectedAction) []semanticAction {
	result := make([]semanticAction, 0, len(actions))
	for _, action := range actions {
		result = append(result, semanticAction{
			resourceType: string(action.resourceType),
			operation:    string(action.operation),
			resourceID:   action.resourceID,
			executable:   action.executable,
		})
	}
	sortSemanticActions(result)
	return result
}

func semanticActions(actions []plan.Action) []semanticAction {
	result := make([]semanticAction, 0, len(actions))
	for _, action := range actions {
		result = append(result, semanticAction{
			resourceType: string(action.ResourceType),
			operation:    string(action.Operation),
			resourceID:   action.ResourceID,
			executable:   action.Executable,
		})
	}
	sortSemanticActions(result)
	return result
}

// Sorting a slice rather than reducing to a set preserves action multiplicity.
func sortSemanticActions(actions []semanticAction) {
	slices.SortFunc(actions, func(a, b semanticAction) int {
		if c := strings.Compare(a.resourceType, b.resourceType); c != 0 {
			return c
		}
		if c := strings.Compare(a.operation, b.operation); c != 0 {
			return c
		}
		if c := strings.Compare(a.resourceID, b.resourceID); c != 0 {
			return c
		}
		if a.executable == b.executable {
			return 0
		}
		if a.executable {
			return 1
		}
		return -1
	})
}

func assertExpectedChanges(t *testing.T, name string, actions []plan.Action, want []expectedAction) {
	t.Helper()
	for _, expected := range want {
		if expected.changes == nil {
			continue
		}
		matches := 0
		for _, action := range actions {
			if action.ResourceType == expected.resourceType && action.Operation == expected.operation && action.ResourceID == expected.resourceID {
				matches++
				if !reflect.DeepEqual(action.Changes, expected.changes) {
					t.Errorf("%s field changes for %s = %#v, want %#v", name, expected.resourceID, action.Changes, expected.changes)
				}
			}
		}
		if matches != 1 {
			t.Errorf("%s actions matching %s/%s = %d, want exactly one for field-change assertion", name, expected.operation, expected.resourceID, matches)
		}
	}
}

func normalizedUnresolved(members []state.TeamMember) []state.TeamMember {
	if members == nil {
		return []state.TeamMember{}
	}
	actual := normalizeParityActual(state.OrganizationState{TeamMembers: members})
	return actual.TeamMembers
}

func assertUnresolvedMemberships(
	t *testing.T,
	name string,
	got []state.TeamMember,
	summary plan.Summary,
	want []state.TeamMember,
	wantHasChanges bool,
) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s unresolved memberships = %#v, want %#v", name, got, want)
	}
	if summary.HasUnresolved != (len(want) > 0) {
		t.Errorf("%s has_unresolved = %t, want %t", name, summary.HasUnresolved, len(want) > 0)
	}
	if summary.UnresolvedMemberships != len(want) {
		t.Errorf("%s unresolved_memberships = %d, want %d", name, summary.UnresolvedMemberships, len(want))
	}
	if summary.HasChanges != wantHasChanges {
		t.Errorf("%s has_changes = %t, want %t", name, summary.HasChanges, wantHasChanges)
	}
}

func assertActionSummary(t *testing.T, name string, got plan.Summary, want []expectedAction) {
	t.Helper()
	wantExecutable := 0
	for _, action := range want {
		if action.executable {
			wantExecutable++
		}
	}
	wantNonExecutable := len(want) - wantExecutable
	if got.Actions != len(want) || got.ExecutableActions != wantExecutable || got.NonExecutableActions != wantNonExecutable {
		t.Errorf("%s action summary = actions:%d executable:%d non_executable:%d, want actions:%d executable:%d non_executable:%d",
			name, got.Actions, got.ExecutableActions, got.NonExecutableActions,
			len(want), wantExecutable, wantNonExecutable)
	}
}
