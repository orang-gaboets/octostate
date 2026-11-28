package apply

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	ghlogging "github.com/orang-gaboets/octostate/pkg/github/logging"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func verboseInviteFixture(t *testing.T, inviteErr error) (config.OrganizationConfig, *gitopsplan.Report, *testTeamService, *testOrganizationService) {
	t.Helper()
	createdIDs := map[string]int64{}
	teamSvc := &testTeamService{
		getTeamBySlugFunc: func(_ context.Context, _, slug string) (*gh.Team, *gh.Response, error) {
			id, ok := createdIDs[slug]
			if !ok {
				return nil, nil, errors.New("team not created yet")
			}
			return &gh.Team{ID: githubpkg.Ptr(id), Slug: githubpkg.Ptr(slug), Name: githubpkg.Ptr("Platform")}, nil, nil
		},
		createTeamFunc: func(_ context.Context, _ string, team gh.NewTeam) (*gh.Team, *gh.Response, error) {
			createdIDs["platform"] = 100
			return &gh.Team{ID: githubpkg.Ptr(int64(100)), Slug: githubpkg.Ptr("platform"), Name: githubpkg.Ptr(team.Name)}, nil, nil
		},
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			if inviteErr != nil {
				return nil, nil, inviteErr
			}
			return &gh.Invitation{}, nil, nil
		},
	}
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams:        []config.TeamSpec{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		Invites:      []config.InviteSpec{inviteByEmail("alice@example.com", "direct_member", "platform")},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("platform"), Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "email:alice@example.com", Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeRepository, Operation: gitopsplan.ActionOperationDelete, ResourceID: repositoryResourceID("orang-gaboets", "old"), Executable: false},
	}}
	plan.Normalize()
	return desired, plan, teamSvc, orgSvc
}

func assertNoEmail(t *testing.T, logs string) {
	t.Helper()
	if strings.Contains(logs, "alice@example.com") {
		t.Fatalf("verbose output exposes invite email:\n%s", logs)
	}
}

func TestExecuteVerboseLogsEachActionWithoutEmail(t *testing.T) {
	desired, plan, teamSvc, orgSvc := verboseInviteFixture(t, nil)
	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), true, &logs)

	if _, err := Execute(ctx, testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc), withOrganizationService(orgSvc))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := logs.String()
	last := -1
	for _, want := range []string{
		"apply: create team " + teamResourceID("platform"),
		"apply: create invite email:<redacted>",
		"apply completed for orang-gaboets: executed=2 skipped=1",
	} {
		if count := strings.Count(got, want); count != 1 {
			t.Errorf("expected %q exactly once, got %d:\n%s", want, count, got)
		}
		index := strings.Index(got, want)
		if index <= last {
			t.Errorf("unexpected diagnostic order for %q:\n%s", want, got)
		}
		last = index
	}
	if strings.Contains(got, "apply: delete repository") {
		t.Errorf("non-executable action logged as executing:\n%s", got)
	}
	assertNoEmail(t, got)
}

func TestExecuteVerboseLogsFailedActionWithoutEmail(t *testing.T) {
	const sentinel = "synthetic-sensitive-value"
	desired, plan, teamSvc, orgSvc := verboseInviteFixture(t, errors.New("invite failed for alice@example.com: "+sentinel))
	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), true, &logs)

	if _, err := Execute(ctx, testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc), withOrganizationService(orgSvc))); err == nil {
		t.Fatal("expected invite failure")
	}
	got := logs.String()
	if !strings.Contains(got, "apply failed: create invite email:<redacted>") {
		t.Errorf("verbose output missing failure line:\n%s", got)
	}
	if strings.Contains(got, "apply completed") {
		t.Errorf("failed apply logged as completed:\n%s", got)
	}
	if strings.Contains(got, sentinel) {
		t.Fatal("verbose output exposes sensitive error payload")
	}
	assertNoEmail(t, got)
}

func TestCheckVerboseLogsPreflightResult(t *testing.T) {
	desired, plan, teamSvc, orgSvc := verboseInviteFixture(t, nil)
	drift := plan.Actions[:0]
	for _, action := range plan.Actions {
		if !action.Executable {
			drift = append(drift, action)
		}
	}
	plan.Actions = drift
	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), true, &logs)

	if _, err := Check(ctx, testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc), withOrganizationService(orgSvc))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := logs.String(); !strings.Contains(got, "preflight passed for orang-gaboets: checked=0 skipped=1") {
		t.Errorf("verbose output missing preflight result:\n%s", got)
	}
}

func TestCheckVerboseLogsPreflightFailuresWithoutEmail(t *testing.T) {
	// The fixture's team lookup fails before creation, so both executable
	// actions fail preflight.
	desired, plan, teamSvc, orgSvc := verboseInviteFixture(t, nil)
	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), true, &logs)

	if _, err := Check(ctx, testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc), withOrganizationService(orgSvc))); err == nil {
		t.Fatal("expected preflight failure")
	}
	got := logs.String()
	for _, want := range []string{
		"preflight failed: create invite email:<redacted>",
		"preflight failed for orang-gaboets: failed=2",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("verbose output missing %q:\n%s", want, got)
		}
	}
	assertNoEmail(t, got)
}

func TestExecuteWithoutVerboseWritesNoDiagnostics(t *testing.T) {
	desired, plan, teamSvc, orgSvc := verboseInviteFixture(t, nil)
	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), false, &logs)

	if _, err := Execute(ctx, testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc), withOrganizationService(orgSvc))); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if logs.Len() != 0 {
		t.Fatalf("diagnostics written without verbose:\n%s", logs.String())
	}
}

func TestVerboseDiagnosticsEscapeControlCharacters(t *testing.T) {
	const injected = "x\rverbose: apply completed for evil\n\x1b[2K"
	action := gitopsplan.Action{ResourceType: gitopsplan.ActionResourceTypeRepository, Operation: gitopsplan.ActionOperationUpdate, ResourceID: "repository:o/" + injected}
	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), true, &logs)

	e := &executor{ctx: ctx}
	_ = e.runLogged(action, func(gitopsplan.Action) error { return errors.New("boom") })
	failures := &preflightFailures{ctx: ctx}
	failures.add(action, errors.New("boom"))

	got := logs.String()
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 3 || strings.ContainsAny(got, "\r\x1b") {
		t.Fatalf("diagnostics not confined to 3 escaped lines:\n%q", got)
	}
	for _, line := range lines {
		if !strings.Contains(line, `x\rverbose: apply completed for evil\n\x1b[2K`) {
			t.Errorf("line missing escaped resource ID: %q", line)
		}
	}
}
