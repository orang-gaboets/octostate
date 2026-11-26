package actionswatcher

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestReconcileFiltersSourcesOrdersAndDeduplicatesFailures(t *testing.T) {
	repo := RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	start := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	newAttempt := attempt(20, 2, "CI", "push", "main", "failure", start.Add(2*time.Hour), "https://github.com/org/repo/actions/runs/20/attempts/2")
	newAttempt.Attempt = 2
	attempts := []RunAttempt{
		newAttempt,
		attempt(10, 1, "CI", "push", "main", "failure", start, "https://github.com/org/repo/actions/runs/10"),
		attempt(10, 1, "CI", "push", "main", "failure", start, "https://github.com/org/repo/actions/runs/10"),
		attempt(30, 1, "CI", "pull_request", "main", "failure", start.Add(3*time.Hour), "https://github.com/org/repo/actions/runs/30"),
		attempt(40, 1, "CI", "schedule", "release", "failure", start.Add(4*time.Hour), "https://github.com/org/repo/actions/runs/40"),
		attempt(50, 1, "CI", "push", "main", "cancelled", start.Add(5*time.Hour), "https://github.com/org/repo/actions/runs/50"),
	}

	plan, err := Reconcile(repo, "actions-watcher[bot]", attempts, nil, start.Add(6*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.Action != ActionCreate {
		t.Fatalf("Action = %q, want %q", change.Action, ActionCreate)
	}
	if change.CurrentStatus != StatusFailing {
		t.Fatalf("CurrentStatus = %q, want %q", change.CurrentStatus, StatusFailing)
	}
	if change.FailureCount != 2 {
		t.Fatalf("FailureCount = %d, want 2", change.FailureCount)
	}
	if !strings.Contains(change.Title, "ci(actions): investigate CI workflow failures on main") {
		t.Fatalf("incident title does not identify CI on main: %q", change.Title)
	}
	for _, want := range []string{
		"Repository: `orang-gaboets/octostate`", "Workflow: **CI**", "Branch: `main`",
		"Latest observed relevant status: **FAILING**", "First failure: 2026-11-25T01:00:00Z",
		"Latest failure: 2026-11-25T03:00:00Z", "Total distinct failed attempts: 2",
		"run ID 10", "run ID 20", "attempt 2", "Commit SHA: `abc123`", "Failed jobs: `test-linux`",
		"Latest observed recovery: none", "https://github.com/orang-gaboets/octostate/actions/runs/20/attempts/2",
	} {
		if !strings.Contains(change.Body, want) {
			t.Errorf("managed body does not preserve %q", want)
		}
	}
	if strings.Contains(change.Body, "raw log") || strings.Contains(change.Body, "log excerpt") {
		t.Fatalf("managed body unexpectedly contains raw logs: %q", change.Body)
	}
}

func TestReconcileUsesAttemptStartOrderingAndOnlySuccessRecovers(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started.Add(time.Hour), "https://example.test/failure")
	lateOldSuccess := attempt(2, 1, "CI", "push", "main", "success", started, "https://example.test/old-success")
	neutral := attempt(3, 1, "CI", "push", "main", "neutral", started.Add(2*time.Hour), "https://example.test/neutral")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{failure, lateOldSuccess, neutral}, nil, started.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got := onlyChange(t, plan).CurrentStatus; got != StatusFailing {
		t.Fatalf("status after stale success and neutral = %q, want failing", got)
	}

	success := attempt(4, 1, "CI", "workflow_dispatch", "main", "success", started.Add(3*time.Hour), "https://example.test/recovery")
	plan, err = Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{failure, success}, nil, started.Add(4*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if got := onlyChange(t, plan).CurrentStatus; got != StatusRecovered {
		t.Fatalf("status after newer success = %q, want recovered", got)
	}
}

func TestReconcileAndPersistedStateRejectRunURLsForAnotherRepositoryOrRun(t *testing.T) {
	repo := testRepository()
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	valid := attempt(42, 42, "CI", "push", "main", "failure", started, "https://github.com/orang-gaboets/octostate/actions/runs/42")
	invalidURLs := []string{
		"https://github.com/other/repo/actions/runs/42",
		"https://github.com/orang-gaboets/octostate/actions/runs/43",
		"https://github.com.evil.test/orang-gaboets/octostate/actions/runs/42",
	}
	for _, badURL := range invalidURLs {
		bad := valid
		bad.HTMLURL = badURL
		if _, err := Reconcile(repo, "actions-watcher[bot]", []RunAttempt{bad}, nil, started.Add(time.Hour)); err == nil {
			t.Errorf("Reconcile accepted run URL %q", badURL)
		}
	}

	body, _, err := RenderBody(repo, IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing,
		Failures: []RunAttempt{valid}, Latest: valid,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	forged := valid
	forged.HTMLURL = invalidURLs[1]
	tampered := strings.Replace(body, renderFailureLine(valid), renderFailureLine(forged), 1)
	if tampered == body {
		t.Fatal("test failed to tamper with the persisted failure record")
	}
	if _, err := RenderBodyState(tampered, repo, 100, "CI"); err == nil {
		t.Fatal("RenderBodyState accepted a persisted URL for a different run ID")
	}
}

func TestReconcileRecoversOnLaterAttemptWithSameStartTime(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/failure")
	success := attempt(2, 2, "CI", "push", "main", "success", started, "https://example.test/recovery")

	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{failure, success}, nil, started.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.CurrentStatus != StatusRecovered {
		t.Fatalf("CurrentStatus = %q, want recovered", change.CurrentStatus)
	}
	if len(change.Comments) != 1 || !strings.Contains(change.Comments[0].Body, "Observed a successful run after the latest failure") {
		t.Fatalf("recovery comments = %#v, want one recovery transition", change.Comments)
	}
	state, err := RenderBodyState(change.Body, testRepository(), 100, "CI")
	if err != nil {
		t.Fatalf("RenderBodyState() error = %v", err)
	}
	if state.Recovery == nil || !sameAttempt(*state.Recovery, success) {
		t.Fatalf("persisted recovery = %#v, want the later same-time success", state.Recovery)
	}
}

func TestReconcileClosedIssueIgnoresReplayButStartsFreshIncidentAfterClosure(t *testing.T) {
	closedAt := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	closed := managedIssue(t, 51, "closed", closedAt, "actions-watcher[bot]", "CI", 100)
	before := attempt(1, 1, "CI", "push", "main", "failure", closedAt.Add(-time.Minute), "https://example.test/before")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{before}, []IssueSnapshot{closed}, closedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("pre-closure replay produced changes: %#v", plan.Changes)
	}

	postClose := attempt(1, 1, "CI", "push", "main", "failure", closedAt.Add(time.Minute), "https://example.test/rerun")
	postClose.Attempt = 2
	plan, err = Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{before, postClose}, []IssueSnapshot{closed}, closedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.Action != ActionCreate || change.PreviousIssueNumber != 51 || change.FailureCount != 1 {
		t.Fatalf("post-close change = %#v; want fresh linked incident with one failure", change)
	}
	if strings.Contains(change.Body, "attempt 1") || !strings.Contains(change.Body, "attempt 2") {
		t.Fatalf("new incident history did not isolate post-close attempts: %q", change.Body)
	}
	parsed, err := RenderBodyState(change.Body, testRepository(), 100, "CI")
	if err != nil || parsed.PreviousIssueNumber != 51 {
		t.Fatalf("previous incident link was not persisted: incident=%#v err=%v", parsed, err)
	}
}

func TestReconcileIgnoresUntrustedWatcherCommentMarkers(t *testing.T) {
	closedAt := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", closedAt.Add(-time.Hour), "https://example.test/failure")
	recovery := attempt(2, 1, "CI", "schedule", "main", "success", closedAt, "https://example.test/recovery")
	state := IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusRecovered,
		Failures: []RunAttempt{failure}, Latest: recovery, Recovery: &recovery,
	}
	body, _, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	issue := IssueSnapshot{
		Number: 52, State: "open", AuthorLogin: "actions-watcher[bot]", Title: incidentTitle("CI"), Body: body,
		Labels: []string{ManagedLabel, RecoveredLabel},
		Comments: []CommentSnapshot{
			{ID: 1, AuthorLogin: "public-user", Body: "quoted: " + archiveMarkerPrefix},
			{ID: 2, AuthorLogin: "public-user", Body: transitionMarkerPrefix + transitionKey(StatusFailing, attempt(3, 1, "CI", "push", "main", "failure", closedAt.Add(time.Hour), "https://example.test/new-failure")) + " -->"},
		},
	}
	newFailure := attempt(3, 1, "CI", "push", "main", "failure", closedAt.Add(time.Hour), "https://example.test/new-failure")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{newFailure}, []IssueSnapshot{issue}, closedAt.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("Reconcile() rejected untrusted comment markers: %v", err)
	}
	change := onlyChange(t, plan)
	if change.CurrentStatus != StatusFailing || len(change.Comments) != 1 {
		t.Fatalf("change = %#v, want failing status and one App-authored transition", change)
	}
}

func TestReconcilePostClosureInitialBodyRetainsPreviousIssueNumber(t *testing.T) {
	closedAt := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	closed := managedIssue(t, 51, "closed", closedAt, "actions-watcher[bot]", "CI", 100)
	failure := attempt(1, 1, "CI", "push", "main", "failure", closedAt.Add(time.Minute), "https://example.test/failure")
	recovery := attempt(2, 2, "CI", "schedule", "main", "success", closedAt.Add(2*time.Minute), "https://example.test/recovery")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{failure, recovery}, []IssueSnapshot{closed}, closedAt.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.InitialBody == "" {
		t.Fatal("expected staged initial body for the recovered incident")
	}
	initial, err := RenderBodyState(change.InitialBody, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if initial.PreviousIssueNumber != 51 {
		t.Fatalf("initial PreviousIssueNumber = %d, want 51", initial.PreviousIssueNumber)
	}
}

func TestReconcileKeepsPreClosureFailuresOutOfSuccessorHistory(t *testing.T) {
	closedAt := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	closed := managedIssue(t, 51, "closed", closedAt, "actions-watcher[bot]", "CI", 100)
	before := attempt(1, 1, "CI", "push", "main", "failure", closedAt.Add(-time.Minute), "https://example.test/before")
	after := attempt(2, 2, "CI", "push", "main", "failure", closedAt.Add(time.Minute), "https://example.test/after")
	when := closedAt.Add(time.Hour)

	created, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{before, after}, []IssueSnapshot{closed}, when)
	if err != nil {
		t.Fatal(err)
	}
	newIssue := onlyChange(t, created)
	successor := IssueSnapshot{
		Number: 52, State: "open", AuthorLogin: "actions-watcher[bot]", Title: newIssue.Title, Body: newIssue.Body,
		Labels: []string{ManagedLabel, FailingLabel},
	}

	updated, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{before, after}, []IssueSnapshot{closed, successor}, when.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Changes) != 0 {
		t.Fatalf("repeat sweep changed successor history: %#v", updated.Changes)
	}
	state, err := RenderBodyState(successor.Body, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if state.Failures[0].RunID != 2 || len(state.Failures) != 1 {
		t.Fatalf("successor history = %#v, want only the post-closure failure", state.Failures)
	}
}

func TestReconcileManualReopenResumesExistingIncident(t *testing.T) {
	previouslyClosed := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	issue := managedIssue(t, 71, "open", previouslyClosed, "actions-watcher[bot]", "CI", 100)
	newFailure := attempt(9, 1, "CI", "push", "main", "failure", previouslyClosed.Add(time.Hour), "https://example.test/reopened")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{newFailure}, []IssueSnapshot{issue}, previouslyClosed.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.Action != ActionUpdate || change.IssueNumber != 71 {
		t.Fatalf("reopened incident change = %#v; want update to issue 71", change)
	}
}

func TestReconcileFailsClosedForWrongAppAndMultipleOpenIncidents(t *testing.T) {
	issue := managedIssue(t, 80, "open", time.Time{}, "somebody[bot]", "CI", 100)
	if _, err := Reconcile(testRepository(), "actions-watcher[bot]", nil, []IssueSnapshot{issue}, time.Now()); err == nil {
		t.Fatal("Reconcile accepted a managed issue with the wrong App author")
	}

	first := managedIssue(t, 81, "open", time.Time{}, "actions-watcher[bot]", "CI", 100)
	second := managedIssue(t, 82, "open", time.Time{}, "actions-watcher[bot]", "CI", 100)
	if _, err := Reconcile(testRepository(), "actions-watcher[bot]", nil, []IssueSnapshot{first, second}, time.Now()); err == nil {
		t.Fatal("Reconcile accepted multiple open managed incidents for one workflow")
	}
}

func TestReconcileLateFailureKeepsNewerRecoveryAndAddsHistory(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	first := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/first")
	recovery := attempt(2, 2, "CI", "schedule", "main", "success", started.Add(2*time.Hour), "https://example.test/recovery")
	state, err := stateFromAttempts(workflowKey{id: 100, name: "CI"}, nil, []RunAttempt{first, recovery})
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	issue := IssueSnapshot{Number: 90, State: "open", AuthorLogin: "actions-watcher[bot]", Title: incidentTitle("CI"), Body: body, Labels: []string{ManagedLabel, RecoveredLabel, "keep-me"}}
	late := attempt(3, 3, "CI", "push", "main", "failure", started.Add(time.Hour), "https://example.test/late-failure")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{late}, []IssueSnapshot{issue}, started.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.CurrentStatus != StatusRecovered {
		t.Fatalf("late older failure changed status to %q, want recovered", change.CurrentStatus)
	}
	if change.FailureCount != 2 || !strings.Contains(change.Body, "run ID 3, attempt 1") {
		t.Fatalf("late failure was not retained in history: count=%d body=%q", change.FailureCount, change.Body)
	}
	if len(change.Comments) != 0 {
		t.Fatalf("late older failure created a transition comment: %#v", change.Comments)
	}
}

func TestReconcilePlansRecoveryAndRegressionCommentsOnce(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/first")
	initialRecovery := attempt(4, 4, "CI", "schedule", "main", "success", started.Add(30*time.Minute), "https://example.test/initial-recovery")
	state, err := stateFromAttempts(workflowKey{id: 100, name: "CI"}, nil, []RunAttempt{failure, initialRecovery})
	if err != nil {
		t.Fatal(err)
	}
	body, _, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	issue := IssueSnapshot{Number: 91, State: "open", AuthorLogin: "actions-watcher[bot]", Title: incidentTitle("CI"), Body: body, Labels: []string{ManagedLabel, RecoveredLabel, "keep-me"}}
	regression := attempt(2, 2, "CI", "push", "main", "failure", started.Add(time.Hour), "https://example.test/regression")
	recovery := attempt(3, 3, "CI", "workflow_dispatch", "main", "success", started.Add(2*time.Hour), "https://example.test/recovery")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{regression, recovery}, []IssueSnapshot{issue}, started.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.CurrentStatus != StatusRecovered || len(change.Comments) != 2 {
		t.Fatalf("change status/comments = %q/%d, want recovered/2", change.CurrentStatus, len(change.Comments))
	}
	if len(change.AddLabels) != 0 || len(change.RemoveLabels) != 0 {
		t.Fatalf("status label delta = add %#v, remove %#v", change.AddLabels, change.RemoveLabels)
	}
	for _, comment := range change.Comments {
		issue.Comments = append(issue.Comments, CommentSnapshot{AuthorLogin: "actions-watcher[bot]", Body: comment.Body})
	}
	issue.Body = change.Body
	issue.Labels = []string{ManagedLabel, RecoveredLabel, "keep-me"}
	plan, err = Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{regression, recovery}, []IssueSnapshot{issue}, started.Add(4*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 0 {
		t.Fatalf("replaying already-applied transitions produced changes: %#v", plan.Changes)
	}
}

func TestReconcileAdvancesOrderingCursorWithoutChangingRecoveryTransition(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started, "")
	recovery := attempt(2, 2, "CI", "schedule", "main", "success", started.Add(time.Hour), "")
	initial, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{failure, recovery}, nil, started.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	created := onlyChange(t, initial)
	issue := IssueSnapshot{
		Number: 90, State: "open", AuthorLogin: "actions-watcher[bot]", Title: created.Title,
		Body: created.Body, Labels: []string{ManagedLabel, RecoveredLabel},
	}
	laterSuccess := attempt(3, 3, "CI", "workflow_dispatch", "main", "success", started.Add(3*time.Hour), "")
	continued, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{laterSuccess}, []IssueSnapshot{issue}, started.Add(4*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, continued)
	if len(change.Comments) != 0 || len(change.AddLabels) != 0 || len(change.RemoveLabels) != 0 {
		t.Fatalf("continued success produced a transition notification or label change: %#v", change)
	}
	state, err := RenderBodyState(change.Body, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if state.Latest.RunID != laterSuccess.RunID || state.Recovery == nil || state.Recovery.RunID != recovery.RunID {
		t.Fatalf("continued-success state = latest %d, recovery %#v; want cursor %d and original recovery %d", state.Latest.RunID, state.Recovery, laterSuccess.RunID, recovery.RunID)
	}

	lateFailure := attempt(4, 4, "CI", "push", "main", "failure", started.Add(2*time.Hour+30*time.Minute), "")
	updated, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{lateFailure}, []IssueSnapshot{{
		Number: 90, State: "open", AuthorLogin: "actions-watcher[bot]", Title: change.Title,
		Body: change.Body, Labels: []string{ManagedLabel, RecoveredLabel},
	}}, started.Add(5*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	lateChange := onlyChange(t, updated)
	lateState, err := RenderBodyState(lateChange.Body, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if lateChange.CurrentStatus != StatusRecovered || lateState.Latest.RunID != laterSuccess.RunID || lateState.Recovery == nil || lateState.Recovery.RunID != laterSuccess.RunID {
		t.Fatalf("late failure changed recovered ordering state: status=%q latest=%d recovery=%#v", lateChange.CurrentStatus, lateState.Latest.RunID, lateState.Recovery)
	}
}

func TestReconcileInitialSnapshotRecordsObservedTransitions(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/failure")
	recovery := attempt(2, 2, "CI", "schedule", "main", "success", started.Add(time.Hour), "https://example.test/recovery")
	regression := attempt(3, 3, "CI", "push", "main", "startup_failure", started.Add(2*time.Hour), "https://example.test/regression")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{failure, recovery, regression}, nil, started.Add(3*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.Action != ActionCreate || change.CurrentStatus != StatusFailing || len(change.Comments) != 2 {
		t.Fatalf("initial incident did not retain transition history: action=%q status=%q comments=%d", change.Action, change.CurrentStatus, len(change.Comments))
	}
}

func TestReconcileIsNoopAfterCreatedIncidentIsReadBack(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/failure")
	first, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{failure}, nil, started.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	created := onlyChange(t, first)
	issue := IssueSnapshot{Number: 101, State: "open", AuthorLogin: "actions-watcher[bot]", Title: created.Title, Body: created.Body, Labels: created.AddLabels}
	second, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{failure}, []IssueSnapshot{issue}, started.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Changes) != 0 {
		t.Fatalf("read-back replay produced duplicate changes: %#v", second.Changes)
	}
}

func TestReconcileRestoresIncidentTitleAfterTitleOnlyEdit(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	issue := managedIssue(t, 101, "open", time.Time{}, "actions-watcher[bot]", "CI", 100)
	issue.Title = "manually renamed incident"

	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", nil, []IssueSnapshot{issue}, started.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.Action != ActionUpdate || change.Title != incidentTitle("CI") {
		t.Fatalf("title-only drift change = %#v, want an update restoring the managed title", change)
	}
}

func TestReconcileEnrichesCheckpointedFailureWhenJobsBecomeAvailable(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	checkpointed := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/run/1")
	checkpointed.JobDataAvailable = false
	checkpointed.FailedJobs = nil
	body, _, err := RenderBody(testRepository(), IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing,
		Failures: []RunAttempt{checkpointed}, Latest: checkpointed,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	issue := IssueSnapshot{
		Number: 101, State: "open", AuthorLogin: "actions-watcher[bot]", Title: incidentTitle("CI"), Body: body,
		Labels: []string{ManagedLabel, FailingLabel},
	}
	enriched := checkpointed
	enriched.JobDataAvailable = true
	enriched.FailedJobs = []string{"test-linux"}

	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{enriched}, []IssueSnapshot{issue}, started.Add(time.Hour))
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	change := onlyChange(t, plan)
	if change.Action != ActionUpdate || !strings.Contains(change.Body, "Failed jobs: `test-linux`") {
		t.Fatalf("enriched checkpoint change = %#v, want updated available job metadata", change)
	}
	state, err := RenderBodyState(change.Body, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if !state.Latest.JobDataAvailable || len(state.Latest.FailedJobs) != 1 || state.Latest.FailedJobs[0] != "test-linux" || !sameAttempt(state.Latest, state.Failures[0]) {
		t.Fatalf("latest/failure state was not enriched consistently: latest=%#v failures=%#v", state.Latest, state.Failures)
	}
}

func TestReconcileRejectsConflictingAttemptAndWorkflowIdentity(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	first := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/first")
	duplicate := first
	duplicate.HeadSHA = "different"
	if _, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{first, duplicate}, nil, started.Add(time.Hour)); err == nil {
		t.Fatal("Reconcile accepted conflicting metadata for a duplicate run attempt")
	}
	otherName := attempt(2, 2, "CodeQL Security", "push", "main", "failure", started.Add(time.Hour), "https://example.test/other")
	if _, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{first, otherName}, nil, started.Add(2*time.Hour)); err == nil {
		t.Fatal("Reconcile accepted one workflow ID with two workflow names")
	}
	unknown := attempt(3, 3, "Unmonitored workflow", "push", "main", "failure", started.Add(time.Hour), "https://example.test/unknown")
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{unknown}, nil, started.Add(2*time.Hour))
	if err != nil || len(plan.Changes) != 0 {
		t.Fatalf("unmonitored workflow was not ignored safely: plan=%#v err=%v", plan, err)
	}

	wrongRepo := managedIssue(t, 89, "open", time.Time{}, "actions-watcher[bot]", "CI", 100)
	wrongRepo.Body = strings.Replace(wrongRepo.Body, "repo_id=99", "repo_id=98", 1)
	if _, err := Reconcile(testRepository(), "actions-watcher[bot]", nil, []IssueSnapshot{wrongRepo}, started.Add(2*time.Hour)); err == nil {
		t.Fatal("Reconcile accepted an issue from another repository identity")
	}
}

func TestReconcileUsesTieBreakersForEqualStartTimes(t *testing.T) {
	started := time.Date(2026, 11, 25, 1, 0, 0, 0, time.UTC)
	first := attempt(20, 2, "CI", "push", "main", "failure", started, "https://example.test/20")
	first.RunNumber = 4
	second := attempt(10, 1, "CI", "push", "main", "timed_out", started, "https://example.test/10")
	second.RunNumber = 5
	plan, err := Reconcile(testRepository(), "actions-watcher[bot]", []RunAttempt{first, second}, nil, started.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	body := onlyChange(t, plan).Body
	history := body[strings.Index(body, "## Failure history"):]
	if strings.Index(history, "run ID 20") > strings.Index(history, "run ID 10") {
		t.Fatalf("equal-start failures were not ordered by run number: %q", body)
	}
}

func TestRenderBodyPreservesHumanTextAndRejectsMalformedMarkers(t *testing.T) {
	state := IncidentState{
		WorkflowID:   100,
		WorkflowName: "CI",
		Status:       StatusFailing,
		Failures:     []RunAttempt{attempt(1, 1, "CI", "push", "main", "failure", time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC), "https://example.test/1")},
		Latest:       attempt(1, 1, "CI", "push", "main", "failure", time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC), "https://example.test/1"),
	}
	original := "Keep this human investigation.\n\n" + identityMarker(testRepository(), 100, "CI") + "\n" + managedStart + "\nold managed content\n" + managedEnd + "\n\nFollow-up owned by maintainers."
	got, _, err := RenderBody(testRepository(), state, original, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "Keep this human investigation.") || !strings.HasSuffix(got, "Follow-up owned by maintainers.") {
		t.Fatalf("human-owned text was not preserved: %q", got)
	}
	if strings.Count(got, managedStart) != 1 || strings.Count(got, managedEnd) != 1 {
		t.Fatalf("rendered body has malformed managed markers: %q", got)
	}
	if _, _, err := RenderBody(testRepository(), state, original+"\n"+managedStart, nil); err == nil {
		t.Fatal("RenderBody accepted duplicate/malformed managed markers")
	}
	validBody, _, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	identity := identityMarker(testRepository(), 100, "CI")
	quotedBody := "For reference, " + identity + "\n" + validBody
	movedIdentity := strings.Replace(quotedBody, identity+"\n\n"+managedStart, managedStart+"\n"+identity, 1)
	if _, _, err := RenderBody(testRepository(), state, movedIdentity, nil); err == nil {
		t.Fatal("RenderBody accepted an identity marker inside the replaceable managed region")
	}
}

func TestRenderBodyStateRejectsRecoveryThatDiffersFromLatest(t *testing.T) {
	started := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/failure")
	recovery := attempt(2, 2, "CI", "schedule", "main", "success", started.Add(time.Hour), "https://example.test/recovery")
	state := IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusRecovered,
		Failures: []RunAttempt{failure}, Latest: recovery, Recovery: &recovery,
	}
	body, _, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	forgedRecovery := recovery
	forgedRecovery.HTMLURL = "https://example.test/forged-recovery"
	tampered := state
	tampered.Recovery = &forgedRecovery
	body = strings.Replace(body, stateMarker(state), stateMarker(tampered), 1)
	if _, err := RenderBodyState(body, testRepository(), 100, "CI"); err == nil {
		t.Fatal("RenderBodyState accepted recovery metadata that differs from verified latest")
	}
}

func TestRenderBodyStatePersistsPendingRunIDs(t *testing.T) {
	started := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/failure")
	body, _, err := RenderBody(testRepository(), IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing,
		Failures: []RunAttempt{failure}, Latest: failure, PendingRunIDs: []int64{303, 202, 303},
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	state, err := RenderBodyState(body, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{202, 303}
	if !slices.Equal(state.PendingRunIDs, want) {
		t.Fatalf("PendingRunIDs = %v, want sorted unique IDs %v", state.PendingRunIDs, want)
	}
}

func TestReconcileClearsConsumedPendingRunIDs(t *testing.T) {
	started := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	failure := attempt(100, 100, "CI", "push", "main", "failure", started, "https://example.test/failure")
	body, _, err := RenderBody(testRepository(), IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing,
		Failures: []RunAttempt{failure}, Latest: failure, PendingRunIDs: []int64{200},
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	issue := IssueSnapshot{
		Number: 7, State: "open", AuthorLogin: "actions-watcher[bot]", Title: incidentTitle("CI"),
		Body: body, Labels: []string{ManagedLabel, FailingLabel},
	}
	plan, err := Reconcile(testRepository(), issue.AuthorLogin, []RunAttempt{failure}, []IssueSnapshot{issue}, started.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	change := onlyChange(t, plan)
	if change.Action != ActionUpdate {
		t.Fatalf("Action = %q, want update that clears the consumed pending run", change.Action)
	}
	state, err := RenderBodyState(change.Body, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if len(state.PendingRunIDs) != 0 {
		t.Fatalf("final PendingRunIDs = %v, want none", state.PendingRunIDs)
	}
}

func TestRenderBodyLinksRerunsToTheirSpecificAttempts(t *testing.T) {
	started := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	runURL := "https://github.com/orang-gaboets/octostate/actions/runs/42"
	first := attempt(42, 42, "CI", "push", "main", "failure", started, runURL)
	second := first
	second.Attempt = 2
	second.StartedAt = started.Add(time.Hour)
	second.CompletedAt = second.StartedAt.Add(time.Minute)
	state := IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing,
		Failures: []RunAttempt{first, second}, Latest: second,
	}
	body, _, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"[run ID 42, attempt 1](" + runURL + "/attempts/1)",
		"[run ID 42, attempt 2](" + runURL + "/attempts/2)",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("managed body does not contain attempt-specific link %q", want)
		}
	}
}

func TestRenderBodyStateRejectsFailureLaterThanLatestCursor(t *testing.T) {
	started := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	latest := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/latest")
	body, _, err := RenderBody(testRepository(), IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing,
		Failures: []RunAttempt{latest}, Latest: latest,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	forged := attempt(999, 999, "CI", "push", "main", "failure", started.Add(time.Hour), "https://example.test/forged")
	end := strings.Index(body, managedEnd)
	if end < 0 {
		t.Fatal("rendered body is missing managed end marker")
	}
	body = body[:end] + renderFailureLine(forged) + "\n" + body[end:]
	if _, err := RenderBodyState(body, testRepository(), 100, "CI"); err == nil {
		t.Fatal("RenderBodyState accepted failure evidence later than the persisted latest cursor")
	}
}

func TestRenderBodyStateRejectsFailingCursorMissingFromHistory(t *testing.T) {
	started := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	first := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/first")
	latest := attempt(2, 2, "CI", "push", "main", "failure", started.Add(time.Hour), "https://example.test/latest")
	body, _, err := RenderBody(testRepository(), IncidentState{
		WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing,
		Failures: []RunAttempt{first, latest}, Latest: latest,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	body = strings.Replace(body, renderFailureLine(latest)+"\n", "", 1)
	if _, err := RenderBodyState(body, testRepository(), 100, "CI"); err == nil {
		t.Fatal("RenderBodyState accepted a failing cursor missing from failure history")
	}
}

func TestRenderBodyArchivesOverflowWithoutDroppingFailureEvidence(t *testing.T) {
	state := IncidentState{WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing}
	for i := 1; i <= 400; i++ {
		started := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute)
		failure := attempt(int64(i), 1, "CI", "push", "main", "failure", started, "")
		failure.FailedJobs = []string{strings.Repeat("x", 500)}
		state.Failures = append(state.Failures, failure)
	}
	state.Latest = state.Failures[len(state.Failures)-1]
	body, archives, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > MaxIssueBodyBytes {
		t.Fatalf("issue body is %d bytes, exceeds %d", len(body), MaxIssueBodyBytes)
	}
	if len(archives) == 0 {
		t.Fatal("overflow did not produce marked archive comments")
	}
	archiveSnapshots := make([]CommentSnapshot, 0, len(archives))
	for _, archive := range archives {
		if !strings.Contains(archive.Marker, "<!-- actions-watcher:archive:v1") || !strings.Contains(archive.Body, "<!-- actions-watcher:failure:v1") {
			t.Fatalf("archive comment lacks machine markers: %#v", archive)
		}
		if len(archive.Attempts) == 0 {
			t.Fatalf("archive comment omitted attempts for API verification: %#v", archive)
		}
		archiveSnapshots = append(archiveSnapshots, CommentSnapshot{AuthorLogin: "actions-watcher[bot]", Body: archive.Body})
	}
	secondBody, secondArchives, err := RenderBody(testRepository(), state, body, archiveSnapshots)
	if err != nil {
		t.Fatal(err)
	}
	if secondBody != body || len(secondArchives) != 0 {
		t.Fatalf("archive readback was not idempotent: body_same=%t new_archives=%d", secondBody == body, len(secondArchives))
	}
	partial, err := RenderBodyState(body, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := RenderBody(testRepository(), partial, body, nil); err == nil {
		t.Fatal("RenderBody silently removed references to missing archive evidence")
	}
	for _, failure := range state.Failures {
		if !strings.Contains(body, failure.HTMLURL) && !archiveContains(archives, failure.HTMLURL) {
			t.Errorf("failure URL %q was not preserved in body or archive", failure.HTMLURL)
		}
	}
}

func TestRenderBodyArchiveIndexStableWhenLateFailureOverflows(t *testing.T) {
	started := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	state := IncidentState{WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing}
	for i := 1; i <= 400; i++ {
		failure := attempt(int64(i), i, "CI", "push", "main", "failure", started.Add(time.Duration(i)*time.Minute), "")
		failure.FailedJobs = []string{strings.Repeat("x", 500)}
		state.Failures = append(state.Failures, failure)
	}
	state.Latest = state.Failures[len(state.Failures)-1]
	body, archives, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) == 0 {
		t.Fatal("initial history did not create archives")
	}

	late := attempt(1000, 1000, "CI", "push", "main", "failure", started, "")
	late.FailedJobs = []string{strings.Repeat("y", 25000)}
	state.Failures = append(state.Failures, late)
	comments := make([]CommentSnapshot, 0, len(archives))
	for i, archive := range archives {
		comments = append(comments, CommentSnapshot{ID: int64(i + 1), AuthorLogin: "actions-watcher[bot]", Body: archive.Body})
	}

	updated, newArchives, err := RenderBody(testRepository(), state, body, comments)
	if err != nil {
		t.Fatalf("RenderBody() with a late older failure: %v", err)
	}
	if len(newArchives) == 0 {
		t.Fatal("late failure did not force a new archive")
	}
	nextCommentID := int64(len(comments) + 1)
	for i, archive := range newArchives {
		comments = append(comments, CommentSnapshot{ID: nextCommentID + int64(i), AuthorLogin: "actions-watcher[bot]", Body: archive.Body})
	}
	readback, duplicateArchives, err := RenderBody(testRepository(), state, updated, comments)
	if err != nil {
		t.Fatalf("RenderBody() readback after adding a late archive: %v", err)
	}
	if readback != updated || len(duplicateArchives) != 0 {
		t.Fatalf("late archive readback was not stable: body_same=%t new_archives=%d", readback == updated, len(duplicateArchives))
	}
	partialReadback, recoveredArchives, err := RenderBody(testRepository(), state, body, comments)
	if err != nil {
		t.Fatalf("RenderBody() recovery after archive comment write: %v", err)
	}
	if partialReadback != updated || len(recoveredArchives) != 0 {
		t.Fatalf("partial archive write was not recoverable: body_same=%t new_archives=%d", partialReadback == updated, len(recoveredArchives))
	}
}

func TestRenderCheckpointBodyArchivesHistoryBeforePendingRunOverflow(t *testing.T) {
	state := IncidentState{WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing}
	for i := 1; i <= 25; i++ {
		started := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Minute)
		state.Failures = append(state.Failures, attempt(int64(i), i, "CI", "push", "main", "failure", started, "https://example.test/attempt/"+string(rune('a'+i%26))))
	}
	state.Latest = state.Failures[len(state.Failures)-1]
	body, archives, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 0 {
		t.Fatalf("initial body unexpectedly overflowed into %d archives", len(archives))
	}

	const note = "maintainer note "
	notes := "\n" + strings.Repeat(note, (MaxIssueBodyBytes-len(body)-32)/len(note))
	body += notes
	if len(body) >= MaxIssueBodyBytes {
		t.Fatalf("test body is %d bytes; expected it just below the limit", len(body))
	}
	pendingRunIDs := make([]int64, MaxPendingRunIDs)
	for i := range pendingRunIDs {
		pendingRunIDs[i] = int64(10000 + i)
	}

	checkpoint, newArchives, err := RenderCheckpointBody(body, testRepository(), pendingRunIDs, nil)
	if err != nil {
		t.Fatalf("RenderCheckpointBody() error = %v", err)
	}
	if len(checkpoint) > MaxIssueBodyBytes {
		t.Fatalf("checkpoint body is %d bytes, exceeds %d", len(checkpoint), MaxIssueBodyBytes)
	}
	if len(newArchives) == 0 {
		t.Fatal("checkpoint did not archive failure history to make room for pending run IDs")
	}
	if !strings.HasSuffix(checkpoint, notes) {
		t.Fatal("checkpoint did not preserve maintainer notes")
	}
	checkpointState, err := RenderBodyState(checkpoint, testRepository(), 100, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpointState.PendingRunIDs) != len(pendingRunIDs) {
		t.Fatalf("checkpoint stored %d pending run IDs, want %d", len(checkpointState.PendingRunIDs), len(pendingRunIDs))
	}
	archivedKeys := make(map[string]bool)
	for _, archive := range newArchives {
		for _, failure := range archive.Attempts {
			archivedKeys[attemptKey(failure)] = true
		}
	}
	for _, failure := range checkpointState.Failures {
		archivedKeys[attemptKey(failure)] = true
	}
	for _, failure := range state.Failures {
		if !archivedKeys[attemptKey(failure)] {
			t.Errorf("checkpoint dropped failure run %d", failure.RunID)
		}
	}
}

func TestRenderCheckpointBodyReconstructsArchivedFailureSummary(t *testing.T) {
	state := IncidentState{WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing}
	started := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for i := 1; i <= 400; i++ {
		state.Failures = append(state.Failures, attempt(int64(i), i, "CI", "push", "main", "failure", started.Add(time.Duration(i)*time.Minute), "https://example.test/attempt/"+strings.Repeat("x", 200)+string(rune('a'+i%26))))
	}
	state.Latest = state.Failures[len(state.Failures)-1]
	body, archives, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) == 0 {
		t.Fatal("initial history did not create archives")
	}
	comments := make([]CommentSnapshot, 0, len(archives))
	for _, archive := range archives {
		comments = append(comments, CommentSnapshot{AuthorLogin: "actions-watcher[bot]", Body: archive.Body})
	}

	checkpoint, _, err := RenderCheckpointBody(body, testRepository(), []int64{5000}, comments)
	if err != nil {
		t.Fatalf("RenderCheckpointBody() error = %v", err)
	}
	for _, want := range []string{
		"First failure: " + formatTime(state.Failures[0].StartedAt),
		"Latest failure: " + formatTime(state.Failures[len(state.Failures)-1].StartedAt),
		"Total distinct failed attempts: 400",
	} {
		if !strings.Contains(checkpoint, want) {
			t.Errorf("checkpoint body does not contain reconstructed summary %q", want)
		}
	}
}

func TestRenderBodyBoundsArchiveReferences(t *testing.T) {
	const archiveCount = 2000
	started := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	state := IncidentState{WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing}
	comments := make([]CommentSnapshot, 0, archiveCount)
	for i := range archiveCount {
		failure := attempt(int64(i+1), i+1, "CI", "push", "main", "failure", started.Add(time.Duration(i)*time.Minute), "https://example.test/run")
		state.Failures = append(state.Failures, failure)
		archive, err := makeArchive(state.WorkflowID, []RunAttempt{failure})
		if err != nil {
			t.Fatal(err)
		}
		comments = append(comments, CommentSnapshot{Body: archive.Body})
	}
	state.Latest = state.Failures[len(state.Failures)-1]

	body, newArchives, err := RenderBody(testRepository(), state, "", comments)
	if err != nil {
		t.Fatalf("RenderBody() with %d existing archives: %v", archiveCount, err)
	}
	if len(newArchives) != 0 {
		t.Fatalf("RenderBody() created %d duplicate archives", len(newArchives))
	}
	if len(body) > MaxIssueBodyBytes {
		t.Fatalf("rendered body is %d bytes, exceeds %d", len(body), MaxIssueBodyBytes)
	}
	references, err := bodyArchiveReferences(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(references) != 20 {
		t.Fatalf("body contains %d archive references, want 20", len(references))
	}
	indexed, hasIndex, err := bodyArchiveIndex(body)
	if err != nil {
		t.Fatal(err)
	}
	if !hasIndex || indexed.Version != 2 || indexed.Count != archiveCount || len(indexed.Digest) != sha256.Size*2 {
		t.Fatalf("body archive index = %#v (present=%t), want v2 count %d with one SHA-256 digest", indexed, hasIndex, archiveCount)
	}

	secondBody, secondArchives, err := RenderBody(testRepository(), state, body, comments)
	if err != nil {
		t.Fatalf("RenderBody() on archived body: %v", err)
	}
	if secondBody != body || len(secondArchives) != 0 {
		t.Fatalf("archive rendering was not idempotent: body_same=%t new_archives=%d", secondBody == body, len(secondArchives))
	}

	if _, _, err := RenderBody(testRepository(), state, body, comments[1:]); err == nil {
		t.Fatal("RenderBody silently accepted a missing archive that was not listed in the issue body")
	}

	withoutIndex := body
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, archiveIndexPrefix) {
			withoutIndex = strings.Replace(body, line+"\n", "", 1)
			break
		}
	}
	stateWithoutOldest := state
	stateWithoutOldest.Failures = append([]RunAttempt(nil), state.Failures[1:]...)
	if _, _, err := RenderBody(testRepository(), stateWithoutOldest, withoutIndex, comments[1:]); err == nil {
		t.Fatal("RenderBody accepted archived history without its full-set index")
	}
}

func TestRenderBodyUpgradesLegacyArchiveIndex(t *testing.T) {
	failure := attempt(1, 1, "CI", "push", "main", "failure", time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), "https://example.test/run")
	state := IncidentState{WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing, Failures: []RunAttempt{failure}, Latest: failure}
	archive, err := makeArchive(state.WorkflowID, []RunAttempt{failure})
	if err != nil {
		t.Fatal(err)
	}
	comments := []CommentSnapshot{{AuthorLogin: "actions-watcher[bot]", Body: archive.Body}}
	body, _, err := RenderBody(testRepository(), state, "", comments)
	if err != nil {
		t.Fatal(err)
	}
	markerHash := sha256.Sum256([]byte(archive.Marker))
	legacyIndex := legacyArchiveIndexPrefix + "count=1 hashes=" + base64.RawStdEncoding.EncodeToString(markerHash[:]) + " -->"
	legacyBody := strings.Replace(body, archiveIndexMarker([]ArchiveComment{archive}), legacyIndex, 1)
	if legacyBody == body {
		t.Fatal("test failed to replace the v2 archive index with the legacy v1 marker")
	}
	upgraded, newArchives, err := RenderBody(testRepository(), state, legacyBody, comments)
	if err != nil {
		t.Fatalf("RenderBody() rejected legacy archive index: %v", err)
	}
	if len(newArchives) != 0 || !strings.Contains(upgraded, archiveIndexPrefix) || strings.Contains(upgraded, legacyArchiveIndexPrefix) {
		t.Fatalf("legacy index was not upgraded in place: new_archives=%d body=%s", len(newArchives), upgraded)
	}
}

func TestRenderBodyEscapesUntrustedJobNamesAndRemainsValidUTF8(t *testing.T) {
	started := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	failure := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/1")
	failure.FailedJobs = []string{"🔐 @everyone <script>alert(1)</script>"}
	state := IncidentState{WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing, Failures: []RunAttempt{failure}, Latest: failure}
	body, _, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(body) || len(body) > MaxIssueBodyBytes {
		t.Fatalf("rendered body is invalid UTF-8 or too large: valid=%t bytes=%d", utf8.ValidString(body), len(body))
	}
	if strings.Contains(body, "@everyone") || !strings.Contains(body, "&#64;everyone") || strings.Contains(body, "<script>") {
		t.Fatalf("untrusted job name was not escaped: %q", body)
	}
}

func TestRenderBodyDistinguishesUnavailableJobsFromKnownEmpty(t *testing.T) {
	started := time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC)
	unavailable := attempt(1, 1, "CI", "push", "main", "failure", started, "https://example.test/1")
	unavailable.JobDataAvailable = false
	unavailable.FailedJobs = nil
	state := IncidentState{WorkflowID: 100, WorkflowName: "CI", Status: StatusFailing, Failures: []RunAttempt{unavailable}, Latest: unavailable}
	body, _, err := RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Failed jobs: unavailable") {
		t.Fatalf("unavailable job data was not shown as unavailable: %q", body)
	}
	knownEmpty := unavailable
	knownEmpty.JobDataAvailable = true
	state.Failures, state.Latest = []RunAttempt{knownEmpty}, knownEmpty
	body, _, err = RenderBody(testRepository(), state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "Failed jobs: none reported") {
		t.Fatalf("known-empty job response was not distinguished: %q", body)
	}
}

func attempt(runID int64, number int, name, event, branch, conclusion string, started time.Time, _ string) RunAttempt {
	return RunAttempt{
		WorkflowID: 100, WorkflowName: name, RunID: runID, RunNumber: number, Attempt: 1,
		Event: event, HeadBranch: branch, Conclusion: conclusion, HeadSHA: "abc123",
		HTMLURL:   fmt.Sprintf("https://github.com/%s/%s/actions/runs/%d", testRepository().Owner, testRepository().Name, runID),
		StartedAt: started, CompletedAt: started.Add(time.Minute),
		FailedJobs: []string{"test-linux"}, JobDataAvailable: true,
	}
}

func testRepository() RepositoryIdentity {
	return RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
}

func onlyChange(t *testing.T, plan Plan) IssueChange {
	t.Helper()
	if len(plan.Changes) != 1 {
		t.Fatalf("got %d changes, want exactly one: %#v", len(plan.Changes), plan.Changes)
	}
	return plan.Changes[0]
}

func managedIssue(t *testing.T, number int, state string, closedAt time.Time, author, workflow string, workflowID int64) IssueSnapshot {
	t.Helper()
	seed := attempt(1000+int64(number), number, workflow, "push", "main", "failure", time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), "https://example.test/seed")
	incident := IncidentState{WorkflowID: workflowID, WorkflowName: workflow, Status: StatusFailing, Failures: []RunAttempt{seed}, Latest: seed}
	body, _, err := RenderBody(testRepository(), incident, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	var closed *time.Time
	if !closedAt.IsZero() {
		closed = &closedAt
	}
	return IssueSnapshot{Number: number, State: state, ClosedAt: closed, AuthorLogin: author, Title: incidentTitle(workflow), Body: body, Labels: []string{ManagedLabel, FailingLabel}}
}

func archiveContains(archives []ArchiveComment, value string) bool {
	for _, archive := range archives {
		if strings.Contains(archive.Body, value) {
			return true
		}
	}
	return false
}
