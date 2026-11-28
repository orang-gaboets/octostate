package apply

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestExecuteFailureReturnsPartialResultAndActionReport(t *testing.T) {
	secret := &applyTestAPIError{message: "request failed for hidden@example.com with token ghp_sensitive"}
	writeCount := 0
	orgService := &testOrganizationService{
		editOrgMembershipFunc: func(_ context.Context, username, _ string, _ *gh.Membership) (*gh.Membership, *gh.Response, error) {
			writeCount++
			if username == "b-fail" {
				return nil, nil, secret
			}
			return &gh.Membership{}, nil, nil
		},
	}
	plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{
		{
			ResourceType: gitopsplan.ActionResourceTypeOrganizationMember,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   organizationMemberResourceID("a-complete"),
			Executable:   true,
			Message:      "add a-complete",
		},
		{
			ResourceType: gitopsplan.ActionResourceTypeOrganizationMember,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   organizationMemberResourceID("b-fail"),
			Executable:   true,
			Message:      "add b-fail",
		},
		{
			ResourceType: gitopsplan.ActionResourceTypeOrganizationMember,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   organizationMemberResourceID("c-not-attempted"),
			Executable:   true,
			Message:      "add c-not-attempted",
		},
		{
			ResourceType: gitopsplan.ActionResourceTypeInvite,
			Operation:    gitopsplan.ActionOperationDelete,
			ResourceID:   "email:hidden@example.com",
			Executable:   false,
			Message:      "remove hidden@example.com",
		},
	}}
	desired := config.OrganizationConfig{
		Organization: "org-a",
		Members: []config.OrganizationMemberSpec{
			{Username: "a-complete", Role: "member"},
			{Username: "b-fail", Role: "member"},
			{Username: "c-not-attempted", Role: "member"},
		},
	}

	result, err := Execute(context.Background(), testApplyOptions(
		desired,
		&state.OrganizationState{Organization: "org-a", TeamMembers: []state.TeamMember{{TeamSlug: "private-team", Username: "private-member", Role: "member", Origin: githubpkg.TeamMembershipOriginUnknown}}},
		plan,
		withOrganizationService(orgService),
	))
	if err == nil {
		t.Fatal("Execute returned no error")
	}
	if result == nil {
		t.Fatal("Execute discarded the partial result")
	}
	if !errors.Is(err, secret) {
		t.Fatalf("execution error does not unwrap the mutation cause: %v", err)
	}
	var cause *applyTestAPIError
	if !errors.As(err, &cause) || cause != secret {
		t.Fatalf("execution error does not preserve the original cause for errors.As: %#v", cause)
	}

	var executionErr *ExecutionError
	if !errors.As(err, &executionErr) {
		t.Fatalf("error has type %T, want *ExecutionError", err)
	}
	if writeCount != 2 {
		t.Fatalf("mutation count = %d, want stop after second action", writeCount)
	}
	assertSafeText(t, executionErr.Error(), secret.Error(), "hidden@example.com", "ghp_sensitive")
	assertRedactedFailureResult(t, result)
	assertInitialFailureReport(t, executionErr.Report(), secret.Error())
}

func TestTrackedServicesRecordEveryCurrentMutationMethod(t *testing.T) {
	tracker := newExecutionTracker([]gitopsplan.Action{{Executable: true}})
	tracker.beginAction(0)
	ctx := context.Background()

	repositoryService := &trackedRepositoryService{Service: &testRepoService{}, tracker: tracker}
	if _, _, err := repositoryService.CreateFromTemplate(ctx, "template-owner", "template", &gh.TemplateRepoRequest{}); err != nil {
		t.Fatalf("CreateFromTemplate: %v", err)
	}
	if _, _, err := repositoryService.Create(ctx, "org-a", &gh.Repository{}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, _, err := repositoryService.Edit(ctx, "org-a", "repo", &gh.Repository{}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if _, _, err := repositoryService.ReplaceAllTopics(ctx, "org-a", "repo", []string{"gitops"}); err != nil {
		t.Fatalf("ReplaceAllTopics: %v", err)
	}

	organizationService := &trackedOrganizationService{Service: &testOrganizationService{}, tracker: tracker}
	if _, _, err := organizationService.CreateOrgInvitation(ctx, "org-a", &gh.CreateOrgInvitationOptions{}); err != nil {
		t.Fatalf("CreateOrgInvitation: %v", err)
	}
	if _, _, err := organizationService.EditOrgMembership(ctx, "octocat", "org-a", &gh.Membership{}); err != nil {
		t.Fatalf("EditOrgMembership: %v", err)
	}

	teamService := &trackedTeamService{Service: &testTeamService{}, tracker: tracker}
	if _, _, err := teamService.CreateTeam(ctx, "org-a", gh.NewTeam{}); err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if _, _, err := teamService.EditTeamBySlug(ctx, "org-a", "platform", gh.NewTeam{}, false); err != nil {
		t.Fatalf("EditTeamBySlug: %v", err)
	}
	if _, _, err := teamService.AddTeamMembershipBySlug(ctx, "org-a", "platform", "octocat", &gh.TeamAddTeamMembershipOptions{}); err != nil {
		t.Fatalf("AddTeamMembershipBySlug: %v", err)
	}
	if _, err := teamService.AddTeamRepoBySlug(ctx, "org-a", "platform", "org-a", "repo", &gh.TeamAddTeamRepoOptions{}); err != nil {
		t.Fatalf("AddTeamRepoBySlug: %v", err)
	}

	want := []string{
		"create_repository",
		"create_repository",
		"apply_repository_settings",
		"replace_repository_topics",
		"create_invitation",
		"set_membership",
		"create_team",
		"update_team",
		"add_team_member",
		"grant_team_repository_permission",
	}
	if got := tracker.outcomes[0].ConfirmedSteps; !reflect.DeepEqual(got, want) {
		t.Fatalf("confirmed mutation steps = %#v, want %#v", got, want)
	}
}

func assertRedactedFailureResult(t *testing.T, result *Result) {
	t.Helper()
	if result == nil {
		t.Fatal("Execute discarded the partial result")
	}
	if len(result.Executed) != 1 || result.Executed[0].ResourceID != "" || result.Executed[0].Message != "" {
		t.Fatalf("partial result should retain one redacted completed action, got %#v", result.Executed)
	}
	if len(result.SkippedDrift) != 1 || result.SkippedDrift[0].ResourceID != "" {
		t.Fatalf("partial result should retain one redacted skipped action, got %#v", result.SkippedDrift)
	}
	if len(result.UnresolvedTeamMemberships) != 0 {
		t.Fatalf("failure result should omit unresolved member identities, got %#v", result.UnresolvedTeamMemberships)
	}
	encodedResult, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal partial result: %v", err)
	}
	assertSafeText(t, string(encodedResult), "private-member", "b-fail", "hidden@example.com")
}

func assertInitialFailureReport(t *testing.T, report ExecutionReport, rawCause string) {
	t.Helper()
	if report.FailedActionIndex != 1 {
		t.Fatalf("failed action index = %d, want 1", report.FailedActionIndex)
	}
	if len(report.ActionOutcomes) != 4 {
		t.Fatalf("action outcome count = %d, want 4", len(report.ActionOutcomes))
	}
	wantStatuses := []ActionOutcomeStatus{
		ActionOutcomeStatusCompleted,
		ActionOutcomeStatusOutcomeUnknown,
		ActionOutcomeStatusNotAttempted,
		ActionOutcomeStatusSkipped,
	}
	for i, want := range wantStatuses {
		if got := report.ActionOutcomes[i].Status; got != want {
			t.Errorf("action %d status = %q, want %q", i, got, want)
		}
		if got := report.ActionOutcomes[i].PlanIndex; got != i {
			t.Errorf("action %d plan index = %d, want %d", i, got, i)
		}
	}
	if got := report.ActionOutcomes[0].ConfirmedSteps; len(got) != 1 || got[0] != "set_membership" {
		t.Errorf("completed action steps = %#v, want [set_membership]", got)
	}
	if got := report.ActionOutcomes[1].UncertainStep; got != "set_membership" {
		t.Errorf("uncertain step = %q, want set_membership", got)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal execution report: %v", err)
	}
	assertSafeText(t, string(encoded), "hidden@example.com", "ghp_sensitive", rawCause)
}

func assertSafeText(t *testing.T, text string, sensitiveValues ...string) {
	t.Helper()
	for _, sensitive := range sensitiveValues {
		if strings.Contains(text, sensitive) {
			t.Fatalf("text exposed sensitive value %q: %s", sensitive, text)
		}
	}
}

type applyTestAPIError struct{ message string }

func (e *applyTestAPIError) Error() string { return e.message }

func TestExecuteFailureBeforeWriteReportsSkippedAndNotAttempted(t *testing.T) {
	plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeOrganizationMember, Operation: gitopsplan.ActionOperationCreate, ResourceID: organizationMemberResourceID("missing"), Executable: true, Message: "add missing"},
		{ResourceType: gitopsplan.ActionResourceTypeOrganizationMember, Operation: gitopsplan.ActionOperationCreate, ResourceID: organizationMemberResourceID("later"), Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationDelete, ResourceID: "email:person@example.com", Executable: false},
	}}
	writes := 0
	orgService := &testOrganizationService{editOrgMembershipFunc: func(context.Context, string, string, *gh.Membership) (*gh.Membership, *gh.Response, error) {
		writes++
		return &gh.Membership{}, nil, nil
	}}

	result, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{Organization: "org-a"}, &state.OrganizationState{Organization: "org-a"}, plan, withOrganizationService(orgService)))
	if !errors.Is(err, githubpkg.ErrNotFound) {
		t.Fatalf("Execute error = %v, want ErrNotFound", err)
	}
	if result == nil || writes != 0 {
		t.Fatalf("partial result = %#v, writes = %d; want result and no writes", result, writes)
	}
	report := executionReportFromError(t, err)
	if report.FailedActionIndex != 0 {
		t.Fatalf("failed action index = %d, want 0", report.FailedActionIndex)
	}
	want := []ActionOutcomeStatus{ActionOutcomeStatusFailedBeforeWrite, ActionOutcomeStatusNotAttempted, ActionOutcomeStatusSkipped}
	for index, status := range want {
		if got := report.ActionOutcomes[index].Status; got != status {
			t.Errorf("action %d status = %q, want %q", index, got, status)
		}
	}
}

func TestExecuteFailureRedactsUnsupportedActionCategories(t *testing.T) {
	tests := []struct {
		name       string
		resourceID string
		resource   gitopsplan.ActionResourceType
		operation  gitopsplan.ActionOperation
		wantType   gitopsplan.ActionResourceType
		wantOp     gitopsplan.ActionOperation
	}{
		{
			name:       "resource type",
			resourceID: "email:private@example.com",
			resource:   gitopsplan.ActionResourceType("private@example.com"),
			operation:  gitopsplan.ActionOperationCreate,
			wantType:   "unknown",
			wantOp:     gitopsplan.ActionOperationCreate,
		},
		{
			name:       "operation",
			resourceID: "ghp_sensitive",
			resource:   gitopsplan.ActionResourceTypeOrganizationMember,
			operation:  gitopsplan.ActionOperation("ghp_sensitive"),
			wantType:   gitopsplan.ActionResourceTypeOrganizationMember,
			wantOp:     "unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{
				{ResourceType: test.resource, Operation: test.operation, ResourceID: test.resourceID, Executable: false},
				{ResourceType: test.resource, Operation: test.operation, ResourceID: test.resourceID, Executable: true},
			}}
			result, err := Execute(context.Background(), testApplyOptions(
				config.OrganizationConfig{Organization: "org-a"},
				&state.OrganizationState{Organization: "org-a"},
				plan,
			))
			if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
				t.Fatalf("Execute error = %v, want ErrInvalidFieldValue", err)
			}
			if result == nil || len(result.SkippedDrift) != 1 {
				t.Fatalf("partial result = %#v, want one skipped action", result)
			}
			if got := result.SkippedDrift[0]; got.ResourceType != test.wantType || got.Operation != test.wantOp {
				t.Fatalf("redacted skipped action categories = %q/%q, want %q/%q", got.ResourceType, got.Operation, test.wantType, test.wantOp)
			}

			report := executionReportFromError(t, err)
			for index, outcome := range report.ActionOutcomes {
				if outcome.ResourceType != test.wantType || outcome.Operation != test.wantOp {
					t.Errorf("action %d categories = %q/%q, want %q/%q", index, outcome.ResourceType, outcome.Operation, test.wantType, test.wantOp)
				}
			}
			resultJSON, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				t.Fatalf("marshal partial result: %v", marshalErr)
			}
			reportJSON, marshalErr := json.Marshal(report)
			if marshalErr != nil {
				t.Fatalf("marshal execution report: %v", marshalErr)
			}
			assertSafeText(t, string(resultJSON), "private@example.com", "ghp_sensitive")
			assertSafeText(t, string(reportJSON), "private@example.com", "ghp_sensitive")
		})
	}
}

func TestExecuteRepositoryUpdateReportsConfirmedAndUncertainSteps(t *testing.T) {
	writeErr := errors.New("topic update response included private payload")
	desiredRepo := config.RepositorySpec{Owner: "org-a", Name: "repo-a", Visibility: "private", Description: "new description", Topics: []string{"gitops"}}
	plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeRepository, Operation: gitopsplan.ActionOperationUpdate, ResourceID: repositoryResourceID("org-a", "repo-a"), Executable: true, Message: "update org-a/repo-a", Changes: []gitopsplan.FieldChange{{Field: "description", From: "old", To: "new description"}, {Field: "topics", From: []string{"old"}, To: []string{"gitops"}}}},
		{ResourceType: gitopsplan.ActionResourceTypeOrganizationMember, Operation: gitopsplan.ActionOperationCreate, ResourceID: organizationMemberResourceID("later"), Executable: true},
	}}
	editCalls := 0
	topicCalls := 0
	repoService := &testRepoService{
		editFunc: func(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			editCalls++
			return &gh.Repository{}, nil, nil
		},
		replaceAllTopicsFunc: func(context.Context, string, string, []string) ([]string, *gh.Response, error) {
			topicCalls++
			return nil, nil, writeErr
		},
	}

	result, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{Organization: "org-a", Repositories: []config.RepositorySpec{desiredRepo}}, &state.OrganizationState{Organization: "org-a"}, plan, withRepoService(repoService)))
	if !errors.Is(err, writeErr) {
		t.Fatalf("Execute error = %v, want original topic error", err)
	}
	if result == nil || editCalls != 1 || topicCalls != 1 {
		t.Fatalf("partial result = %#v, edit calls = %d, topic calls = %d", result, editCalls, topicCalls)
	}
	report := executionReportFromError(t, err)
	outcome := report.ActionOutcomes[0]
	if outcome.Status != ActionOutcomeStatusPartiallyCompleted || outcome.UncertainStep != "replace_repository_topics" || !reflect.DeepEqual(outcome.ConfirmedSteps, []string{"apply_repository_settings"}) {
		t.Fatalf("repository outcome = %#v", outcome)
	}
	if report.FailedActionIndex != 0 || report.ActionOutcomes[1].Status != ActionOutcomeStatusNotAttempted {
		t.Fatalf("unexpected failure position or later action status: %#v", report)
	}
	if len(result.Executed) != 0 || len(result.SkippedDrift) != 0 {
		t.Fatalf("unexpected raw or uncompleted actions in partial result: %#v", result)
	}
}

func TestExecuteRepositoryCreateTracksAcknowledgedWritesBeforePostconditionFailure(t *testing.T) {
	desiredRepo := config.RepositorySpec{Owner: "org-a", Name: "repo-a", Visibility: "public", Topics: []string{"gitops"}}
	plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeRepository,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   repositoryResourceID("org-a", "repo-a"),
		Executable:   true,
	}}}
	repoService := &testRepoService{
		createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			return &gh.Repository{}, nil, nil
		},
		editFunc: func(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			return nil, nil, nil
		},
		replaceAllTopicsFunc: func(context.Context, string, string, []string) ([]string, *gh.Response, error) {
			return nil, nil, errors.New("topics failed")
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{Organization: "org-a", Repositories: []config.RepositorySpec{desiredRepo}}, &state.OrganizationState{Organization: "org-a"}, plan, withRepoService(repoService)))
	if err == nil {
		t.Fatal("Execute returned no error")
	}
	outcome := executionReportFromError(t, err).ActionOutcomes[0]
	if outcome.Status != ActionOutcomeStatusPartiallyCompleted || outcome.UncertainStep != "replace_repository_topics" {
		t.Fatalf("repository outcome = %#v", outcome)
	}
	if !reflect.DeepEqual(outcome.ConfirmedSteps, []string{"create_repository", "apply_repository_settings"}) {
		t.Fatalf("confirmed repository steps = %#v", outcome.ConfirmedSteps)
	}
}

func TestExecuteGroupedTeamCreationPreservesPlanIndexesWhenReordered(t *testing.T) {
	createErr := errors.New("team write failed")
	var created []string
	teamService := &testTeamService{
		getTeamBySlugFunc: func(context.Context, string, string) (*gh.Team, *gh.Response, error) {
			return &gh.Team{ID: githubpkg.Ptr(int64(20)), Slug: githubpkg.Ptr("parent"), Name: githubpkg.Ptr("Parent")}, nil, nil
		},
		createTeamFunc: func(_ context.Context, _ string, team gh.NewTeam) (*gh.Team, *gh.Response, error) {
			created = append(created, team.Name)
			if team.Name == "Child" {
				return nil, nil, createErr
			}
			return &gh.Team{ID: githubpkg.Ptr(int64(20)), Slug: githubpkg.Ptr("parent"), Name: githubpkg.Ptr("Parent")}, nil, nil
		},
	}
	desired := config.OrganizationConfig{Organization: "org-a", Teams: []config.TeamSpec{
		{Slug: "child", Name: "Child", Privacy: "closed", ParentSlug: "parent"},
		{Slug: "parent", Name: "Parent", Privacy: "closed"},
	}}
	plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("child"), Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("parent"), Executable: true},
	}}

	result, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "org-a"}, plan, withTeamService(teamService)))
	if !errors.Is(err, createErr) {
		t.Fatalf("Execute error = %v, want child team error", err)
	}
	if result == nil || !reflect.DeepEqual(created, []string{"Parent", "Child"}) {
		t.Fatalf("partial result = %#v, create order = %#v", result, created)
	}
	report := executionReportFromError(t, err)
	if report.FailedActionIndex != 0 || report.ActionOutcomes[0].Status != ActionOutcomeStatusOutcomeUnknown || report.ActionOutcomes[1].Status != ActionOutcomeStatusCompleted {
		t.Fatalf("grouped team outcomes do not follow original indexes: %#v", report)
	}
}

func TestExecuteTeamCreationWithoutReturnedIDIsPartiallyCompleted(t *testing.T) {
	teamService := &testTeamService{createTeamFunc: func(context.Context, string, gh.NewTeam) (*gh.Team, *gh.Response, error) {
		return &gh.Team{}, nil, nil
	}}
	desired := config.OrganizationConfig{Organization: "org-a", Teams: []config.TeamSpec{{Slug: "platform", Name: "Platform", Privacy: "closed"}}}
	plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeTeam,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   teamResourceID("platform"),
		Executable:   true,
	}}}

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "org-a"}, plan, withTeamService(teamService)))
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("Execute error = %v, want ErrInvalidFieldValue", err)
	}
	outcome := executionReportFromError(t, err).ActionOutcomes[0]
	if outcome.Status != ActionOutcomeStatusPartiallyCompleted || outcome.UncertainStep != "" || !reflect.DeepEqual(outcome.ConfirmedSteps, []string{"create_team"}) {
		t.Fatalf("team outcome = %#v", outcome)
	}
}

func TestExecuteInvitePreResolutionFailureIsIndexedAndBeforeWrite(t *testing.T) {
	lookupErr := errors.New("lookup failed for private@example.com with token ghp_sensitive")
	writes := 0
	orgService := &testOrganizationService{
		editOrgMembershipFunc: func(context.Context, string, string, *gh.Membership) (*gh.Membership, *gh.Response, error) {
			return &gh.Membership{}, nil, nil
		},
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			writes++
			return &gh.Invitation{}, nil, nil
		},
	}
	userService := &testUserService{getFunc: func(_ context.Context, username string) (*gh.User, *gh.Response, error) {
		if username == "second" {
			return nil, nil, lookupErr
		}
		return &gh.User{ID: githubpkg.Ptr(int64(42))}, nil, nil
	}}
	desired := config.OrganizationConfig{Organization: "org-a", Members: []config.OrganizationMemberSpec{{Username: "done", Role: "member"}}, Invites: []config.InviteSpec{inviteByUsername("first", "direct_member"), inviteByUsername("second", "direct_member")}}
	plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeOrganizationMember, Operation: gitopsplan.ActionOperationCreate, ResourceID: organizationMemberResourceID("done"), Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "username:first", Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "username:second", Executable: true},
	}}

	result, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "org-a"}, plan, withOrganizationService(orgService), withUserService(userService)))
	if !errors.Is(err, lookupErr) {
		t.Fatalf("Execute error = %v, want original lookup error", err)
	}
	if result == nil || len(result.Executed) != 1 || writes != 0 {
		t.Fatalf("partial result = %#v, invitation writes = %d", result, writes)
	}
	report := executionReportFromError(t, err)
	if report.FailedActionIndex != 2 || report.ActionOutcomes[0].Status != ActionOutcomeStatusCompleted || report.ActionOutcomes[1].Status != ActionOutcomeStatusNotAttempted || report.ActionOutcomes[2].Status != ActionOutcomeStatusFailedBeforeWrite {
		t.Fatalf("unexpected pre-resolution outcomes: %#v", report)
	}
	encoded, marshalErr := json.Marshal(report)
	if marshalErr != nil {
		t.Fatalf("marshal report: %v", marshalErr)
	}
	if strings.Contains(string(encoded), "private@example.com") || strings.Contains(string(encoded), "ghp_sensitive") {
		t.Fatalf("pre-resolution report exposed sensitive data: %s", encoded)
	}
}

func TestExecuteInvitePreResolutionParentCancellationHasNoActionIndex(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lookupCalls := 0
	writes := 0
	userService := &testUserService{getFunc: func(context.Context, string) (*gh.User, *gh.Response, error) {
		lookupCalls++
		cancel()
		return nil, nil, context.Canceled
	}}
	orgService := &testOrganizationService{createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
		writes++
		return &gh.Invitation{}, nil, nil
	}}
	desired := config.OrganizationConfig{
		Organization: "org-a",
		Invites: []config.InviteSpec{
			inviteByEmail("first@example.com", "direct_member"),
			inviteByUsername("second", "direct_member"),
		},
	}
	plan := &gitopsplan.Report{Organization: "org-a", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "email:first@example.com", Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "username:second", Executable: true},
	}}

	result, err := Execute(ctx, testApplyOptions(desired, &state.OrganizationState{Organization: "org-a"}, plan, withOrganizationService(orgService), withUserService(userService)))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Execute error = %v, want parent context cancellation", err)
	}
	if result == nil || len(result.Executed) != 0 || lookupCalls != 1 || writes != 0 {
		t.Fatalf("partial result = %#v, lookup calls = %d, invitation writes = %d; want no completed action or writes", result, lookupCalls, writes)
	}

	report := executionReportFromError(t, err)
	if report.FailedActionIndex != -1 {
		t.Errorf("failed action index = %d, want -1 for batch cancellation", report.FailedActionIndex)
	}
	if report.FailurePhase != ExecutionFailurePhaseInviteUsernameResolution {
		t.Errorf("failure phase = %q, want %q", report.FailurePhase, ExecutionFailurePhaseInviteUsernameResolution)
	}
	encoded, marshalErr := json.Marshal(report)
	if marshalErr != nil {
		t.Fatalf("marshal execution report: %v", marshalErr)
	}
	if !strings.Contains(string(encoded), `"failure_phase":"invite_username_resolution"`) {
		t.Errorf("serialized report missing failure phase: %s", encoded)
	}
	for index, outcome := range report.ActionOutcomes {
		if outcome.Status != ActionOutcomeStatusNotAttempted {
			t.Errorf("action %d status = %q, want not_attempted", index, outcome.Status)
		}
	}
}

func TestExecuteCancellationBeforeAndDuringMutationHaveDifferentStatuses(t *testing.T) {
	t.Run("before mutation call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		calls := 0
		orgService := &testOrganizationService{editOrgMembershipFunc: func(context.Context, string, string, *gh.Membership) (*gh.Membership, *gh.Response, error) {
			calls++
			return &gh.Membership{}, nil, nil
		}}
		plan := oneMembershipCreatePlan("org-a", "member")
		_, err := Execute(ctx, testApplyOptions(config.OrganizationConfig{Organization: "org-a", Members: []config.OrganizationMemberSpec{{Username: "member", Role: "member"}}}, &state.OrganizationState{Organization: "org-a"}, plan, withOrganizationService(orgService)))
		if !errors.Is(err, context.Canceled) || calls != 0 {
			t.Fatalf("Execute error = %v, calls = %d; want cancellation before service call", err, calls)
		}
		outcome := executionReportFromError(t, err).ActionOutcomes[0]
		if outcome.Status != ActionOutcomeStatusFailedBeforeWrite || outcome.UncertainStep != "" {
			t.Fatalf("pre-call cancellation outcome = %#v", outcome)
		}
	})

	t.Run("during mutation call", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		calls := 0
		orgService := &testOrganizationService{editOrgMembershipFunc: func(context.Context, string, string, *gh.Membership) (*gh.Membership, *gh.Response, error) {
			calls++
			cancel()
			return nil, nil, context.Canceled
		}}
		plan := oneMembershipCreatePlan("org-a", "member")
		_, err := Execute(ctx, testApplyOptions(config.OrganizationConfig{Organization: "org-a", Members: []config.OrganizationMemberSpec{{Username: "member", Role: "member"}}}, &state.OrganizationState{Organization: "org-a"}, plan, withOrganizationService(orgService)))
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("Execute error = %v, calls = %d; want entered call to return cancellation", err, calls)
		}
		outcome := executionReportFromError(t, err).ActionOutcomes[0]
		if outcome.Status != ActionOutcomeStatusOutcomeUnknown || outcome.UncertainStep != "set_membership" {
			t.Fatalf("in-call cancellation outcome = %#v", outcome)
		}
	})
}

func executionReportFromError(t *testing.T, err error) ExecutionReport {
	t.Helper()
	var executionErr *ExecutionError
	if !errors.As(err, &executionErr) {
		t.Fatalf("error type = %T, want *ExecutionError", err)
	}
	return executionErr.Report()
}

func oneMembershipCreatePlan(org, username string) *gitopsplan.Report {
	return &gitopsplan.Report{Organization: org, Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeOrganizationMember,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   organizationMemberResourceID(username),
		Executable:   true,
	}}}
}
