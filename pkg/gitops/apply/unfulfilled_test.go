package apply

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"

	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func planWith(actions ...gitopsplan.Action) *gitopsplan.Report {
	report := &gitopsplan.Report{Organization: "org-a", Actions: actions}
	report.Normalize()
	return report
}

func blockedRepositoryCreate() gitopsplan.Action {
	return gitopsplan.Action{
		ResourceType: gitopsplan.ActionResourceTypeRepository,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   repositoryResourceID("org-a", "application"),
		Executable:   false,
		Message:      "repository org-a/application cannot be created because its template is unusable",
	}
}

func unsupportedRepositoryDelete() gitopsplan.Action {
	return gitopsplan.Action{
		ResourceType: gitopsplan.ActionResourceTypeRepository,
		Operation:    gitopsplan.ActionOperationDelete,
		ResourceID:   repositoryResourceID("org-a", "legacy"),
		Executable:   false,
		Message:      "repository org-a/legacy exists in live state but is not declared in desired config",
	}
}

// The distinction the whole issue rests on: a non-executable create is a
// desired mutation that will not happen, while a non-executable delete is
// drift Octostate intentionally declines to reconcile.
func TestUnfulfilledDesiredActionsSeparatesBlockedMutationsFromUnsupportedDrift(t *testing.T) {
	t.Parallel()

	executable := gitopsplan.Action{
		ResourceType: gitopsplan.ActionResourceTypeTeam,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   "platform",
		Executable:   true,
	}
	removal := gitopsplan.Action{
		ResourceType: gitopsplan.ActionResourceTypeTeamMember,
		Operation:    gitopsplan.ActionOperationRemove,
		ResourceID:   "platform/bob",
		Executable:   false,
	}

	got := UnfulfilledDesiredActions([]gitopsplan.Action{
		executable, unsupportedRepositoryDelete(), removal, blockedRepositoryCreate(),
	})

	if len(got) != 1 || got[0].ResourceID != repositoryResourceID("org-a", "application") {
		t.Fatalf("expected only the blocked create, got %#v", got)
	}
}

func TestUnfulfilledDesiredActionsEmptyForDriftOnlyPlan(t *testing.T) {
	t.Parallel()

	if got := UnfulfilledDesiredActions([]gitopsplan.Action{unsupportedRepositoryDelete()}); len(got) != 0 {
		t.Fatalf("unsupported drift must not be reported as unfulfilled: %#v", got)
	}
}

func requireExecutableOptions(t *testing.T, plan *gitopsplan.Report) Options {
	t.Helper()

	opts := testApplyOptions(paddedDesired(), &state.OrganizationState{Organization: "orang-gaboets"}, plan)
	opts.Desired.Organization = "org-a"
	opts.Actual = &state.OrganizationState{Organization: "org-a"}
	opts.RequireExecutableDesiredActions = true
	return opts
}

func TestCheckFailsOnUnfulfilledDesiredActionWhenRequired(t *testing.T) {
	t.Parallel()

	_, err := Check(context.Background(), requireExecutableOptions(t, planWith(blockedRepositoryCreate())))
	if err == nil {
		t.Fatal("check must fail when a desired create cannot execute and executability is required")
	}
	if !errors.Is(err, ErrUnfulfilledDesiredAction) {
		t.Fatalf("expected ErrUnfulfilledDesiredAction, got %v", err)
	}
}

func TestCheckSucceedsOnUnsupportedDriftWhenRequired(t *testing.T) {
	t.Parallel()

	if _, err := Check(context.Background(), requireExecutableOptions(t, planWith(unsupportedRepositoryDelete()))); err != nil {
		t.Fatalf("unsupported drift must not fail the requirement: %v", err)
	}
}

func TestCheckIgnoresUnfulfilledDesiredActionByDefault(t *testing.T) {
	t.Parallel()

	opts := requireExecutableOptions(t, planWith(blockedRepositoryCreate()))
	opts.RequireExecutableDesiredActions = false

	result, err := Check(context.Background(), opts)
	if err != nil {
		t.Fatalf("default behavior must be unchanged: %v", err)
	}
	if len(result.SkippedActions) != 1 {
		t.Fatalf("expected the blocked create to remain reported as skipped, got %#v", result.SkippedActions)
	}
}

// Execute must refuse before mutating anything, so a partially applied plan
// cannot result from a requirement the caller stated up front.
func TestExecuteFailsBeforeMutatingWhenDesiredActionUnfulfilled(t *testing.T) {
	t.Parallel()

	executableTeam := gitopsplan.Action{
		ResourceType: gitopsplan.ActionResourceTypeTeam,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   "platform",
		Executable:   true,
	}

	opts := requireExecutableOptions(t, planWith(executableTeam, blockedRepositoryCreate()))
	opts.TeamService = &testTeamService{
		createTeamFunc: func(context.Context, string, gh.NewTeam) (*gh.Team, *gh.Response, error) {
			t.Fatal("no GitHub write may happen once the requirement is known to fail")
			return nil, nil, nil
		},
	}

	if _, err := Execute(context.Background(), opts); !errors.Is(err, ErrUnfulfilledDesiredAction) {
		t.Fatalf("expected ErrUnfulfilledDesiredAction, got %v", err)
	}
}
