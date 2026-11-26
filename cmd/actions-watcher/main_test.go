package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	gh "github.com/google/go-github/v88/github"
	watcher "github.com/orang-gaboets/octostate/internal/actionswatcher"
)

func pendingRunEvents(repo watcher.RepositoryIdentity, appLogin string, issues []watcher.IssueSnapshot) ([]triggerEvent, []triggerEvent, error) {
	return pendingRunEventsAt(repo, appLogin, issues, time.Time{})
}

func TestReadConfigRequiresTrustedRepositoryAndDedicatedApp(t *testing.T) {
	eventFile := t.TempDir() + "/event.json"
	if err := os.WriteFile(eventFile, []byte(`{"ref":"refs/heads/main"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"GITHUB_REPOSITORY": expectedRepository, "GITHUB_REPOSITORY_ID": "99", "GITHUB_EVENT_NAME": "push",
		"GITHUB_EVENT_PATH": eventFile, "GITHUB_REF": "refs/heads/main", "GITHUB_API_URL": "https://api.github.com",
		"ACTIONS_WATCHER_TOKEN": "installation-token", "ACTIONS_WATCHER_APP_SLUG": expectedAppSlug,
		"ACTIONS_WATCHER_ACTIVATED_AT": "2026-11-24T00:00:00Z",
	}
	getenv := func(key string) string { return values[key] }
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	if _, err := readConfig(getenv, now); err != nil {
		t.Fatalf("readConfig() error = %v", err)
	}

	for key, value := range map[string]string{
		"GITHUB_REPOSITORY": "owner/other", "GITHUB_REF": "refs/heads/feature",
		"ACTIONS_WATCHER_TOKEN": "", "ACTIONS_WATCHER_APP_SLUG": "octostate-test-app",
	} {
		values[key] = value
		if _, err := readConfig(getenv, now); err == nil {
			t.Errorf("readConfig() accepted %s=%q", key, value)
		}
		values[key] = map[string]string{
			"GITHUB_REPOSITORY": expectedRepository, "GITHUB_REF": "refs/heads/main",
			"ACTIONS_WATCHER_TOKEN": "installation-token", "ACTIONS_WATCHER_APP_SLUG": expectedAppSlug,
		}[key]
	}
}

func TestPendingRunEventsReadsManagedState(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	failure := testFailureAttempt(100, 100, time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC))
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: []int64{103, 104},
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	closedAt := time.Date(2026, 11, 25, 9, 0, 0, 0, time.UTC)
	events, deferred, err := pendingRunEvents(repo, expectedAppSlug+"[bot]", []watcher.IssueSnapshot{
		{Number: 7, State: "open", Body: body},
		{Number: 6, State: "closed", ClosedAt: &closedAt, Body: body},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []triggerEvent{{RunID: 103, WorkflowName: "CI", IssueNumber: 6}, {RunID: 104, WorkflowName: "CI", IssueNumber: 6}}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("pendingRunEvents() = %#v, want %#v", events, want)
	}
	if len(deferred) != 0 {
		t.Fatalf("pendingRunEvents() deferred = %#v, want none", deferred)
	}
}

func TestPendingRunEventsRetainsClosedBacklogsAcrossSuccessors(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	closedAt := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(100, 100, closedAt.Add(-time.Hour))
	makeBody := func(pending []int64, previous int) string {
		t.Helper()
		body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
			WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
			PreviousIssueNumber: previous, Failures: []watcher.RunAttempt{failure}, Latest: failure,
			PendingRunIDs: pending,
		}, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	olderClosedAt := closedAt.Add(-time.Hour)
	older := watcher.IssueSnapshot{Number: 6, State: "closed", ClosedAt: &olderClosedAt, Body: makeBody([]int64{101}, 0)}
	latestClosed := watcher.IssueSnapshot{Number: 7, State: "closed", ClosedAt: &closedAt, Body: makeBody([]int64{101, 102}, 0)}

	events, deferred, err := pendingRunEvents(repo, expectedAppSlug+"[bot]", []watcher.IssueSnapshot{older, latestClosed})
	if err != nil {
		t.Fatal(err)
	}
	want := []triggerEvent{
		{RunID: 101, WorkflowName: "CI", IssueNumber: 6},
		{RunID: 102, WorkflowName: "CI", IssueNumber: 7},
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("pendingRunEvents() without open successor = %#v, want %#v", events, want)
	}
	if len(deferred) != 0 {
		t.Fatalf("pendingRunEvents() without open successor deferred = %#v, want none", deferred)
	}

	successor := watcher.IssueSnapshot{
		Number: 8, State: "open", Body: makeBody([]int64{101, 102, 103}, 7),
	}
	events, deferred, err = pendingRunEvents(repo, expectedAppSlug+"[bot]", []watcher.IssueSnapshot{older, latestClosed, successor})
	if err != nil {
		t.Fatal(err)
	}
	want = []triggerEvent{
		{RunID: 101, WorkflowName: "CI", IssueNumber: 6},
		{RunID: 102, WorkflowName: "CI", IssueNumber: 7},
		{RunID: 103, WorkflowName: "CI", IssueNumber: 8},
	}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("pendingRunEvents() with open successor = %#v, want %#v", events, want)
	}
	if len(deferred) != 0 {
		t.Fatalf("pendingRunEvents() with open successor deferred = %#v, want none", deferred)
	}
}

func TestPendingRunEventsBatchesClosedIncidentsOldestFirst(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(1, 1, started)
	makeBody := func(pending []int64) string {
		t.Helper()
		body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
			WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
			Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: pending,
		}, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}

	const issueCount = 15
	issues := make([]watcher.IssueSnapshot, issueCount+1)
	for i := range issues {
		if i == issueCount {
			pending := make([]int64, maxPendingRunEvents)
			for j := range pending {
				pending[j] = int64(issueCount*maxPendingRunEvents + j + 1)
			}
			issues[i] = watcher.IssueSnapshot{Number: issueCount + 1, State: "open", Body: makeBody(pending)}
			continue
		}
		pending := make([]int64, maxPendingRunEvents)
		for j := range pending {
			pending[j] = int64(i*maxPendingRunEvents + j + 1)
		}
		closedAt := started.Add(time.Duration(i+1) * time.Hour)
		issues[i] = watcher.IssueSnapshot{
			Number: i + 1, State: "closed", ClosedAt: &closedAt, Body: makeBody(pending),
		}
	}

	events, deferred, err := pendingRunEvents(repo, expectedAppSlug+"[bot]", issues)
	if err != nil {
		t.Fatalf("pendingRunEvents() first batch error = %v", err)
	}
	const batchLimit = maxPendingRunEvents
	if len(events) != batchLimit {
		t.Fatalf("first batch size = %d, want %d", len(events), batchLimit)
	}
	if events[0].RunID != issueCount*maxPendingRunEvents+1 || events[0].IssueNumber != issueCount+1 ||
		events[len(events)-1].RunID != (issueCount+1)*maxPendingRunEvents || events[len(events)-1].IssueNumber != issueCount+1 || len(deferred) != 0 {
		t.Fatalf("first batch = first=%#v last=%#v deferred=%d", events[0], events[len(events)-1], len(deferred))
	}
	issues[issueCount].Body = makeBody(nil)
	for i := 0; i < issueCount; i++ {
		events, deferred, err = pendingRunEvents(repo, expectedAppSlug+"[bot]", issues)
		if err != nil {
			t.Fatalf("pendingRunEvents() closed batch %d error = %v", i, err)
		}
		if len(events) != maxPendingRunEvents || events[0].RunID != int64(i*maxPendingRunEvents+1) ||
			events[len(events)-1].RunID != int64((i+1)*maxPendingRunEvents) || events[0].IssueNumber != i+1 || events[len(events)-1].IssueNumber != i+1 || len(deferred) != 0 {
			t.Fatalf("closed batch %d = first=%#v last=%#v deferred=%d", i, events[0], events[len(events)-1], len(deferred))
		}
		issues[i].Body = makeBody(nil)
	}
	events, deferred, err = pendingRunEvents(repo, expectedAppSlug+"[bot]", issues)
	if err != nil || len(events) != 0 || len(deferred) != 0 {
		t.Fatalf("pendingRunEvents() after draining = %d selected, %d deferred, err=%v; want none", len(events), len(deferred), err)
	}
}

func TestPendingRunEventsRotatesClosedCheckpointsWithoutMutatingIssues(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(1, 1, started)
	makeBody := func(pending []int64) string {
		t.Helper()
		body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
			WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
			Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: pending,
		}, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}

	const pendingCount = maxPendingRunEvents*2 + 2
	firstPendingIDs := make([]int64, maxPendingRunEvents+1)
	for i := range firstPendingIDs {
		firstPendingIDs[i] = int64(10_001 + i)
	}
	secondPendingIDs := make([]int64, maxPendingRunEvents+1)
	for i := range secondPendingIDs {
		secondPendingIDs[i] = int64(10_001 + maxPendingRunEvents + i + 1)
	}
	firstClosedAt, secondClosedAt := started, started.Add(time.Hour)
	issues := []watcher.IssueSnapshot{
		{Number: 7, State: "closed", ClosedAt: &firstClosedAt, Body: makeBody(firstPendingIDs)},
		{Number: 8, State: "closed", ClosedAt: &secondClosedAt, Body: makeBody(secondPendingIDs)},
	}
	originalIssues := append([]watcher.IssueSnapshot(nil), issues...)
	for i := range originalIssues {
		closedAt := *originalIssues[i].ClosedAt
		originalIssues[i].ClosedAt = &closedAt
	}
	seen := make(map[int64]bool, pendingCount)
	for hour := int64(0); hour < 3; hour++ {
		reconcileAt := time.Unix(hour*int64(time.Hour/time.Second), 0).UTC()
		events, deferred, err := pendingRunEventsAt(repo, expectedAppSlug+"[bot]", issues, reconcileAt)
		if err != nil {
			t.Fatalf("pendingRunEventsAt(hour %d) error = %v", hour, err)
		}
		if len(events) != maxPendingRunEvents || len(deferred) != 0 {
			t.Fatalf("pendingRunEventsAt(hour %d) = %d selected, %d deferred; want %d and 0", hour, len(events), len(deferred), maxPendingRunEvents)
		}
		for _, event := range events {
			seen[event.RunID] = true
		}
		if !reflect.DeepEqual(issues, originalIssues) {
			t.Fatalf("pendingRunEventsAt(hour %d) mutated closed issue checkpoints", hour)
		}
	}
	if len(seen) != pendingCount {
		t.Fatalf("three rotating selections visited %d pending IDs, want %d", len(seen), pendingCount)
	}
}

func TestPendingRunEventsDoesNotStarveClosedCheckpointsBehindOpenBacklog(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	makeBody := func(failure watcher.RunAttempt, pending []int64) string {
		t.Helper()
		body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
			WorkflowID: failure.WorkflowID, WorkflowName: failure.WorkflowName, Status: watcher.StatusFailing,
			Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: pending,
		}, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	ciFailure := testFailureAttempt(1, 1, started)
	codeQLFailure := testFailureAttempt(2, 2, started.Add(time.Minute))
	codeQLFailure.WorkflowID, codeQLFailure.WorkflowName = 2, "CodeQL Security"
	makeIDs := func(first int64, count int) []int64 {
		ids := make([]int64, count)
		for i := range ids {
			ids[i] = first + int64(i)
		}
		return ids
	}
	closedAt := started.Add(time.Hour)
	issues := []watcher.IssueSnapshot{
		{Number: 7, State: "open", Body: makeBody(ciFailure, makeIDs(1, maxPendingRunEvents))},
		{Number: 8, State: "open", Body: makeBody(codeQLFailure, makeIDs(maxPendingRunEvents+1, maxPendingRunEvents))},
		{Number: 6, State: "closed", ClosedAt: &closedAt, Body: makeBody(ciFailure, makeIDs(2*maxPendingRunEvents+1, maxPendingRunEvents+1))},
	}
	originalIssues := append([]watcher.IssueSnapshot(nil), issues...)
	originalClosedAt := *originalIssues[2].ClosedAt
	originalIssues[2].ClosedAt = &originalClosedAt
	closedRunIDs := make(map[int64]bool, maxPendingRunEvents+1)
	for _, runID := range makeIDs(2*maxPendingRunEvents+1, maxPendingRunEvents+1) {
		closedRunIDs[runID] = true
	}
	seenClosedRunIDs := make(map[int64]bool, len(closedRunIDs))
	for hour := int64(0); hour < 4; hour++ {
		reconcileAt := time.Unix(hour*int64(time.Hour/time.Second), 0).UTC()
		events, _, err := pendingRunEventsAt(repo, expectedAppSlug+"[bot]", issues, reconcileAt)
		if err != nil {
			t.Fatalf("pendingRunEventsAt(hour %d) error = %v", hour, err)
		}
		if len(events) != maxPendingRunEvents {
			t.Fatalf("pendingRunEventsAt(hour %d) selected %d events, want %d", hour, len(events), maxPendingRunEvents)
		}
		for _, event := range events {
			if closedRunIDs[event.RunID] {
				seenClosedRunIDs[event.RunID] = true
			}
		}
		if !reflect.DeepEqual(issues, originalIssues) {
			t.Fatalf("pendingRunEventsAt(hour %d) mutated issue checkpoints", hour)
		}
	}
	if len(seenClosedRunIDs) != len(closedRunIDs) {
		t.Fatalf("four rotating selections visited %d of %d closed pending IDs", len(seenClosedRunIDs), len(closedRunIDs))
	}
}

func TestPendingRunEventsDefersOpenCheckpointsAndPreservesThemInPlan(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	ciFailure := testFailureAttempt(1, 1, started)
	codeQLFailure := testFailureAttempt(2, 2, started.Add(time.Minute))
	codeQLFailure.WorkflowID, codeQLFailure.WorkflowName = 2, "CodeQL Security"
	makeBody := func(failure watcher.RunAttempt, pending []int64) string {
		t.Helper()
		body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
			WorkflowID: failure.WorkflowID, WorkflowName: failure.WorkflowName,
			Status: watcher.StatusFailing, Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: pending,
		}, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	ciIDs := make([]int64, maxPendingRunEvents)
	codeQLIDs := make([]int64, maxPendingRunEvents)
	for i := range ciIDs {
		ciIDs[i] = int64(i + 1)
		codeQLIDs[i] = int64(maxPendingRunEvents + i + 1)
	}
	issues := []watcher.IssueSnapshot{
		{Number: 7, State: "open", Body: makeBody(ciFailure, ciIDs)},
		{Number: 8, State: "open", Body: makeBody(codeQLFailure, codeQLIDs)},
	}
	events, deferred, err := pendingRunEvents(repo, expectedAppSlug+"[bot]", issues)
	if err != nil {
		t.Fatalf("pendingRunEvents() error = %v", err)
	}
	if len(events) != maxPendingRunEvents {
		t.Fatalf("selected batch has %d events, want %d", len(events), maxPendingRunEvents)
	}
	if events[0].IssueNumber != 7 || events[len(events)-1].IssueNumber != 7 {
		t.Fatalf("selected batch boundaries = %#v / %#v, want issue #7", events[0], events[len(events)-1])
	}
	if len(deferred) != maxPendingRunEvents {
		t.Fatalf("deferred batch has %d events, want %d", len(deferred), maxPendingRunEvents)
	}
	if deferred[0].IssueNumber != 8 || deferred[len(deferred)-1].IssueNumber != 8 {
		t.Fatalf("deferred batch boundaries = %#v / %#v, want issue #8", deferred[0], deferred[len(deferred)-1])
	}
	planBody := makeBody(codeQLFailure, nil)
	plan := watcher.Plan{Changes: []watcher.IssueChange{{
		Action: watcher.ActionUpdate, WorkflowID: 2, WorkflowName: "CodeQL Security", IssueNumber: 8, Body: planBody,
	}}}
	if err := preserveDeferredRunEvents(&plan, deferred, issues, repo, expectedAppSlug+"[bot]"); err != nil {
		t.Fatalf("preserveDeferredRunEvents() error = %v", err)
	}
	state, err := watcher.RenderManagedBodyState(plan.Changes[0].Body, repo)
	if err != nil {
		t.Fatalf("RenderManagedBodyState() error = %v", err)
	}
	if !reflect.DeepEqual(state.PendingRunIDs, codeQLIDs) || len(plan.Changes[0].PendingRunIDs) != 0 {
		t.Fatalf("preserved deferred IDs = body:%d checkpoint:%d; want %d body IDs and no selected IDs", len(state.PendingRunIDs), len(plan.Changes[0].PendingRunIDs), len(codeQLIDs))
	}
}

func TestPreserveDeferredRunEventsKeepsOverflowInCheckpointComments(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(1, 1, started)
	bodyIDs := make([]int64, watcher.MaxPendingRunIDs)
	for i := range bodyIDs {
		bodyIDs[i] = int64(i + 1)
	}
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: bodyIDs,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	commentIDs := make([]int64, watcher.MaxPendingRunIDs)
	for i := range commentIDs {
		commentIDs[i] = int64(watcher.MaxPendingRunIDs + i + 1)
	}
	marker, err := pendingCheckpointMarker(1, commentIDs)
	if err != nil {
		t.Fatal(err)
	}
	issue := watcher.IssueSnapshot{
		Number: 7, State: "open", Body: body,
		Comments: []watcher.CommentSnapshot{{ID: 1, AuthorLogin: expectedAppSlug + "[bot]", Body: marker}},
	}
	events, deferred, err := pendingRunEvents(repo, expectedAppSlug+"[bot]", []watcher.IssueSnapshot{issue})
	if err != nil {
		t.Fatalf("pendingRunEvents() error = %v", err)
	}
	if len(events) != maxPendingRunEvents || len(deferred) != 1002 {
		t.Fatalf("pendingRunEvents() = %d selected, %d deferred; want %d and 1002", len(events), len(deferred), maxPendingRunEvents)
	}
	planBody, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{failure}, Latest: failure,
	}, body, nil)
	if err != nil {
		t.Fatal(err)
	}
	selectedIDs := make([]int64, len(events))
	for i, event := range events {
		selectedIDs[i] = event.RunID
	}
	plan := watcher.Plan{Changes: []watcher.IssueChange{{
		Action: watcher.ActionUpdate, WorkflowID: 1, WorkflowName: "CI", IssueNumber: 7,
		Body: planBody, PendingRunIDs: selectedIDs,
	}}}
	if err := preserveDeferredRunEvents(&plan, deferred, []watcher.IssueSnapshot{issue}, repo, expectedAppSlug+"[bot]"); err != nil {
		t.Fatalf("preserveDeferredRunEvents() error = %v", err)
	}
	state, err := watcher.RenderManagedBodyState(plan.Changes[0].Body, repo)
	if err != nil {
		t.Fatalf("RenderManagedBodyState() error = %v", err)
	}
	if len(state.PendingRunIDs) != watcher.MaxPendingRunIDs {
		t.Fatalf("planned body stores %d deferred IDs, want at most %d", len(state.PendingRunIDs), watcher.MaxPendingRunIDs)
	}
	if !reflect.DeepEqual(plan.Changes[0].PendingRunIDs, selectedIDs) {
		t.Fatalf("plan checkpoint IDs grew to %d, want only selected IDs (%d)", len(plan.Changes[0].PendingRunIDs), len(selectedIDs))
	}
}

func TestApplyPlanKeepsDeferredOverflowCheckpointComment(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(1, 1, started)
	bodyIDs := make([]int64, watcher.MaxPendingRunIDs)
	for i := range bodyIDs {
		bodyIDs[i] = int64(i + 1)
	}
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: bodyIDs,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	commentIDs := make([]int64, watcher.MaxPendingRunIDs)
	for i := range commentIDs {
		commentIDs[i] = int64(watcher.MaxPendingRunIDs + i + 1)
	}
	marker, err := pendingCheckpointMarker(1, commentIDs)
	if err != nil {
		t.Fatal(err)
	}
	initial := watcher.IssueSnapshot{
		Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "incident",
		Body: body, Labels: []string{watcher.ManagedLabel, watcher.FailingLabel},
		Comments: []watcher.CommentSnapshot{{ID: 1, AuthorLogin: expectedAppSlug + "[bot]", Body: marker}},
	}
	apiIssue := issueSnapshotJSON(initial)
	apiComments := []any{map[string]any{
		"id": 1, "body": marker, "user": map[string]any{"login": expectedAppSlug + "[bot]"},
	}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, apiComments)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, apiIssue)
		case r.Method == http.MethodPatch && path == "repos/orang-gaboets/octostate/issues/7":
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode issue patch: %v", err)
			}
			apiIssue["title"], apiIssue["body"], apiIssue["labels"] = request.Title, request.Body, labelObjects(request.Labels)
			writeJSON(w, http.StatusOK, apiIssue)
		case r.Method == http.MethodDelete && path == "repos/orang-gaboets/octostate/issues/comments/1":
			apiComments = nil
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	deferredBodyIDs := make([]int64, watcher.MaxPendingRunIDs)
	for i := range deferredBodyIDs {
		deferredBodyIDs[i] = int64(maxPendingRunEvents + i + 1)
	}
	planBody, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: deferredBodyIDs,
	}, body, initial.Comments)
	if err != nil {
		t.Fatal(err)
	}
	selectedIDs := make([]int64, maxPendingRunEvents)
	for i := range selectedIDs {
		selectedIDs[i] = int64(i + 1)
	}
	change := watcher.IssueChange{
		Action: watcher.ActionUpdate, WorkflowID: 1, WorkflowName: "CI", IssueNumber: 7,
		Title: "incident", Body: planBody, PendingRunIDs: selectedIDs,
	}
	client := testWatcherClient(t, server.URL, repo)
	if err := client.applyPlan(context.Background(), watcher.Plan{Changes: []watcher.IssueChange{change}}, []watcher.IssueSnapshot{initial}); err != nil {
		t.Fatalf("applyPlan() error = %v", err)
	}
	if len(apiComments) != 1 || apiComments[0].(map[string]any)["body"] != marker {
		t.Fatalf("pending checkpoint comments after apply = %#v; want the marker containing unprocessed run %d", apiComments, commentIDs[len(commentIDs)-1])
	}
	state, err := watcher.RenderManagedBodyState(apiIssue["body"].(string), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.PendingRunIDs) != watcher.MaxPendingRunIDs {
		t.Fatalf("final body stores %d pending IDs, want %d", len(state.PendingRunIDs), watcher.MaxPendingRunIDs)
	}
}

func TestRemovePendingCheckpointCommentsKeepsUncompletedRunIDs(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	completedMarker, err := pendingCheckpointMarker(1, []int64{101})
	if err != nil {
		t.Fatal(err)
	}
	remainingMarker, err := pendingCheckpointMarker(1, []int64{102})
	if err != nil {
		t.Fatal(err)
	}
	comments := []any{
		map[string]any{"id": 1, "body": completedMarker, "user": map[string]any{"login": expectedAppSlug + "[bot]"}},
		map[string]any{"id": 2, "body": remainingMarker, "user": map[string]any{"login": expectedAppSlug + "[bot]"}},
	}
	expected := watcher.IssueSnapshot{
		Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "incident",
		Body: "managed body", Comments: []watcher.CommentSnapshot{
			{ID: 1, AuthorLogin: expectedAppSlug + "[bot]", Body: completedMarker},
			{ID: 2, AuthorLogin: expectedAppSlug + "[bot]", Body: remainingMarker},
		},
	}
	apiIssue := issueSnapshotJSON(expected)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, comments)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, apiIssue)
		case r.Method == http.MethodDelete && path == "repos/orang-gaboets/octostate/issues/comments/1":
			comments = comments[1:]
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, repo)
	updated, err := client.removePendingCheckpointComments(context.Background(), expected, 1, map[int64]bool{101: true})
	if err != nil {
		t.Fatalf("removePendingCheckpointComments() error = %v", err)
	}
	if len(updated.Comments) != 1 || updated.Comments[0].ID != 2 || updated.Comments[0].Body != remainingMarker {
		t.Fatalf("remaining pending checkpoints = %#v, want only uncompleted run 102", updated.Comments)
	}
}

func TestClosedIssuesRejectWatcherCommentAndCheckpointMutations(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	marker, err := pendingCheckpointMarker(1, []int64{101})
	if err != nil {
		t.Fatal(err)
	}
	closed := watcher.IssueSnapshot{
		Number: 7, State: "closed", AuthorLogin: expectedAppSlug + "[bot]", Title: "incident",
		Body: "managed body", Comments: []watcher.CommentSnapshot{{ID: 1, AuthorLogin: expectedAppSlug + "[bot]", Body: marker}},
	}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "closed issue should not be read for mutation"})
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, repo)

	if _, err := client.ensureMarkedComment(context.Background(), closed, "marker", "comment"); err == nil {
		t.Fatal("ensureMarkedComment() accepted a closed issue")
	}
	if _, err := client.removePendingCheckpointComments(context.Background(), closed, 1, nil); err == nil {
		t.Fatal("removePendingCheckpointComments() accepted a closed issue")
	}
	if requests != 0 {
		t.Fatalf("closed issue mutation guards made %d API requests, want none", requests)
	}
}

func TestCompleteOpenPendingRunsRetriesCheckpointCleanupAfterNoop(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(100, 100, started)
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{failure}, Latest: failure,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	event := triggerEvent{RunID: 101, WorkflowName: "CI", IssueNumber: 7}
	missing := unavailableRun{Event: event, StatusCode: http.StatusGone}
	pendingMarker, err := pendingCheckpointMarker(1, []int64{event.RunID})
	if err != nil {
		t.Fatal(err)
	}
	issue := watcher.IssueSnapshot{
		Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "incident",
		Body: body, Labels: []string{watcher.ManagedLabel, watcher.FailingLabel},
		Comments: []watcher.CommentSnapshot{{ID: 1, AuthorLogin: expectedAppSlug + "[bot]", Body: pendingMarker}},
	}
	apiIssue := issueSnapshotJSON(issue)
	comments := []any{map[string]any{
		"id": 1, "body": pendingMarker, "user": map[string]any{"login": expectedAppSlug + "[bot]"},
	}}
	deleteAttempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, comments)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, apiIssue)
		case r.Method == http.MethodPost && path == "repos/orang-gaboets/octostate/issues/7/comments":
			var request struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode comment request: %v", err)
			}
			comments = append(comments, map[string]any{
				"id": 2, "body": request.Body, "user": map[string]any{"login": expectedAppSlug + "[bot]"},
			})
			writeJSON(w, http.StatusCreated, comments[len(comments)-1])
		case r.Method == http.MethodDelete && path == "repos/orang-gaboets/octostate/issues/comments/1":
			deleteAttempts++
			if deleteAttempts == 1 {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "temporary deletion failure"})
				return
			}
			comments = comments[1:]
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, repo)

	err = client.completeOpenPendingRuns(context.Background(), []watcher.IssueSnapshot{issue}, []triggerEvent{event}, []unavailableRun{missing}, watcher.Plan{})
	if err == nil || deleteAttempts != 1 || len(comments) != 2 {
		t.Fatalf("first checkpoint cleanup error/deletes/comments = %v/%d/%d; want failure, one attempt, and both markers", err, deleteAttempts, len(comments))
	}
	issue.Comments = append(issue.Comments, watcher.CommentSnapshot{
		ID: 2, AuthorLogin: expectedAppSlug + "[bot]", Body: unavailableRunComment(1, missing).Body,
	})
	err = client.completeOpenPendingRuns(context.Background(), []watcher.IssueSnapshot{issue}, []triggerEvent{event}, []unavailableRun{missing}, watcher.Plan{})
	if err != nil {
		t.Fatalf("retry checkpoint cleanup after noop plan: %v", err)
	}
	if deleteAttempts != 2 || len(comments) != 1 || comments[0].(map[string]any)["body"] != unavailableRunComment(1, missing).Body {
		t.Fatalf("checkpoint comments after retry = %#v with %d deletes; want only the visible unavailable marker", comments, deleteAttempts)
	}
}

//nolint:gocyclo // The API route branches are part of this end-to-end checkpoint scenario.
func TestRunLeavesClosedIncidentUntouchedAndRetriesPendingRuns(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	closedAt := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	previousFailure := testFailureAttempt(100, 100, closedAt.Add(-time.Hour))
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{previousFailure}, Latest: previousFailure,
		PendingRunIDs: []int64{901, 902},
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	oldIssue := issueJSON(7, "closed", "ci(actions): investigate CI workflow failures on main", body, []string{watcher.ManagedLabel, watcher.FailingLabel})
	oldIssue["closed_at"] = closedAt.Format(time.RFC3339)
	issues := map[int]map[string]any{7: oldIssue}
	comments := map[int][]any{7: {}}
	nextIssueNumber := 7
	closedIssueWrites := 0
	started := closedAt.Add(5 * time.Minute)
	newRun := runFixture(repo.ID, 901, 1, "failure")
	newRun["run_started_at"] = started.Format(time.RFC3339)
	newRun["updated_at"] = started.Add(time.Minute).Format(time.RFC3339)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "installation/repositories":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "repositories": []any{map[string]any{"id": repo.ID, "full_name": expectedRepository}}})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/workflows":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 7, "workflows": workflowFixtures()})
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/workflows/") && strings.HasSuffix(path, "/runs"):
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 0, "workflow_runs": []any{}})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/runs/901":
			writeJSON(w, http.StatusOK, newRun)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/runs/902":
			writeJSON(w, http.StatusGone, map[string]any{"message": "run evidence has expired"})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/runs/901/attempts/1":
			attempt := runAttemptFixture(repo.ID, 901, 1, "failure")
			attempt["run_number"] = 901
			attempt["run_started_at"] = started.Format(time.RFC3339)
			attempt["updated_at"] = started.Add(time.Minute).Format(time.RFC3339)
			writeJSON(w, http.StatusOK, attempt)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/runs/901/attempts/1/jobs":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "jobs": []any{map[string]any{"name": "test", "conclusion": "failure"}}})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/labels":
			writeJSON(w, http.StatusOK, labelObjects([]string{watcher.ManagedLabel, watcher.FailingLabel, watcher.RecoveredLabel}))
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues":
			listed := make([]any, 0, len(issues))
			for number := 1; number <= nextIssueNumber; number++ {
				if issue, ok := issues[number]; ok {
					listed = append(listed, issue)
				}
			}
			writeJSON(w, http.StatusOK, listed)
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/") && strings.HasSuffix(path, "/comments"):
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			writeJSON(w, http.StatusOK, comments[number])
		case r.Method == http.MethodPost && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/") && strings.HasSuffix(path, "/comments"):
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			if number == 7 {
				closedIssueWrites++
			}
			var request struct {
				Body string `json:"body"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&request); decodeErr != nil {
				t.Errorf("decode comment creation: %v", decodeErr)
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid comment"})
				return
			}
			comment := map[string]any{"id": 1, "body": request.Body, "user": map[string]any{"login": expectedAppSlug + "[bot]"}}
			comments[number] = append(comments[number], comment)
			writeJSON(w, http.StatusCreated, comment)
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/"):
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			if issue, ok := issues[number]; ok {
				writeJSON(w, http.StatusOK, issue)
			} else {
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "unknown issue"})
			}
		case r.Method == http.MethodPost && path == "repos/orang-gaboets/octostate/issues":
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&request); decodeErr != nil {
				t.Errorf("decode issue creation: %v", decodeErr)
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid issue"})
				return
			}
			nextIssueNumber++
			issues[nextIssueNumber] = issueJSON(nextIssueNumber, "open", request.Title, request.Body, request.Labels)
			comments[nextIssueNumber] = []any{}
			writeJSON(w, http.StatusCreated, issues[nextIssueNumber])
		case r.Method == http.MethodPatch && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/"):
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			if number == 7 {
				closedIssueWrites++
			}
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&request); decodeErr != nil {
				t.Errorf("decode issue update: %v", decodeErr)
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid issue update"})
				return
			}
			issue := issues[number]
			if request.Title != "" {
				issue["title"] = request.Title
			}
			if request.Body != "" {
				issue["body"] = request.Body
			}
			if request.Labels != nil {
				issue["labels"] = labelObjects(request.Labels)
			}
			writeJSON(w, http.StatusOK, issue)
		case r.Method == http.MethodDelete && path == "repos/orang-gaboets/octostate/issues/comments/1":
			closedIssueWrites++
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	eventPath := t.TempDir() + "/event.json"
	if err := os.WriteFile(eventPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"GITHUB_REPOSITORY": expectedRepository, "GITHUB_REPOSITORY_ID": "99", "GITHUB_EVENT_NAME": "schedule",
		"GITHUB_EVENT_PATH": eventPath, "GITHUB_REF": "refs/heads/main", "GITHUB_API_URL": server.URL,
		"ACTIONS_WATCHER_TOKEN": "installation-token", "ACTIONS_WATCHER_APP_SLUG": expectedAppSlug,
		"ACTIONS_WATCHER_ACTIVATED_AT": "2026-11-24T00:00:00Z",
	}
	var stdout bytes.Buffer
	err = run(context.Background(), func(key string) string { return values[key] }, &stdout, func() time.Time { return started.Add(time.Hour) })
	if err != nil {
		t.Fatalf("first run() error = %v", err)
	}
	if nextIssueNumber != 8 {
		t.Fatalf("created successor issues after first run = %d, want one successor", nextIssueNumber-7)
	}
	if !strings.Contains(stdout.String(), `"run_id":902`) || !strings.Contains(stdout.String(), `"status_code":410`) {
		t.Fatalf("first run() output = %q, want unavailable run 902 with HTTP 410", stdout.String())
	}
	stdout.Reset()
	err = run(context.Background(), func(key string) string { return values[key] }, &stdout, func() time.Time { return started.Add(time.Hour) })
	if err != nil {
		t.Fatalf("retry run() error = %v", err)
	}
	if nextIssueNumber != 8 {
		t.Fatalf("created successor issues = %d, want one successor", nextIssueNumber-7)
	}
	if !strings.Contains(stdout.String(), `"run_id":902`) || !strings.Contains(stdout.String(), `"status_code":410`) {
		t.Fatalf("retry run() output = %q, want unavailable run 902 with HTTP 410", stdout.String())
	}
	if closedIssueWrites != 0 || issues[7]["body"] != body || len(comments[7]) != 0 {
		t.Fatalf("closed issue changed: writes=%d body_same=%t comments=%#v", closedIssueWrites, issues[7]["body"] == body, comments[7])
	}
	if issues[7]["state"] != "closed" {
		t.Fatalf("closed incident state = %v, want closed", issues[7]["state"])
	}
	if newIssueState, err := watcher.RenderManagedBodyState(issues[8]["body"].(string), repo); err != nil || newIssueState.PreviousIssueNumber != 7 {
		t.Fatalf("successor state = %#v, err=%v; want previous issue #7", newIssueState, err)
	}
}

func TestRecordRunEventsPersistsUnavailableRunAndKeepsOtherCheckpoints(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(100, 100, started)
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: []int64{901},
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	otherFailure := testFailureAttempt(902, 902, started.Add(time.Hour))
	otherFailure.WorkflowID, otherFailure.WorkflowName = 2, "CodeQL Security"
	plan, err := watcher.Reconcile(repo, expectedAppSlug+"[bot]", []watcher.RunAttempt{otherFailure}, []watcher.IssueSnapshot{{
		Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "incident", Body: body,
		Labels: []string{watcher.ManagedLabel, watcher.FailingLabel},
	}}, started.Add(2*time.Hour))
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("Reconcile() changes = %#v, want CI update and CodeQL create", plan.Changes)
	}
	events := []triggerEvent{
		{RunID: 901, WorkflowName: "CI", IssueNumber: 7},
		{RunID: 902, WorkflowName: "CodeQL Security"},
	}
	for runID := int64(10000); runID < 10000+watcher.MaxPendingRunIDs+1; runID++ {
		events = append(events, triggerEvent{RunID: runID, WorkflowName: "CI", IssueNumber: 6})
	}
	unavailable := []unavailableRun{{Event: events[0], StatusCode: http.StatusGone}}
	if err := recordRunEvents(&plan, events, unavailable); err != nil {
		t.Fatalf("recordRunEvents() error = %v", err)
	}
	var ciChange, codeqlChange *watcher.IssueChange
	for i := range plan.Changes {
		switch plan.Changes[i].WorkflowName {
		case "CI":
			ciChange = &plan.Changes[i]
		case "CodeQL Security":
			codeqlChange = &plan.Changes[i]
		}
	}
	if ciChange == nil || codeqlChange == nil {
		t.Fatalf("plan omitted a workflow change: %#v", plan.Changes)
	}
	ciState, err := watcher.RenderBodyState(ciChange.Body, repo, 1, "CI")
	if err != nil || len(ciState.PendingRunIDs) != 0 {
		t.Fatalf("CI plan retained pending run: state=%#v err=%v", ciState, err)
	}
	if !reflect.DeepEqual(ciChange.PendingRunIDs, []int64{901}) || len(ciChange.Comments) != 1 {
		t.Fatalf("CI change pending/comments = %v/%#v, want the unavailable run checkpointed and one marker", ciChange.PendingRunIDs, ciChange.Comments)
	}
	if comment := ciChange.Comments[0]; !strings.Contains(comment.Marker, "run_id=901 status=410") || !strings.Contains(comment.Body, "does not indicate a successful run") {
		t.Fatalf("unavailable marker comment = %#v", comment)
	}
	if got := codeqlChange.PendingRunIDs; !reflect.DeepEqual(got, []int64{902}) {
		t.Fatalf("CodeQL pending run IDs = %v, want [902]", got)
	}
	result, err := json.Marshal(resultFor(1, plan, unavailable...))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result), `"unavailable_runs":[{"workflow":"CI","issue":7,"run_id":901,"status_code":410}]`) {
		t.Fatalf("result omitted unavailable run: %s", result)
	}
}

func TestCollectAttemptsRecordsGoneRunAndContinuesOtherWorkflow(t *testing.T) {
	const repoID = 99
	var missingCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/workflows/") && strings.HasSuffix(path, "/runs"):
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 0, "workflow_runs": []any{}})
		case path == "repos/orang-gaboets/octostate/actions/runs/901":
			missingCalls.Add(1)
			writeJSON(w, http.StatusGone, map[string]any{"message": "workflow run expired"})
		case path == "repos/orang-gaboets/octostate/actions/runs/902":
			run := runFixture(repoID, 902, 1, "failure")
			run["name"], run["workflow_id"], run["path"] = "CodeQL Security", 2, expectedWorkflowPaths["CodeQL Security"]+"@main"
			writeJSON(w, http.StatusOK, run)
		case path == "repos/orang-gaboets/octostate/actions/runs/902/attempts/1":
			attempt := runAttemptFixture(repoID, 902, 1, "failure")
			attempt["name"], attempt["workflow_id"], attempt["path"] = "CodeQL Security", 2, expectedWorkflowPaths["CodeQL Security"]+"@main"
			writeJSON(w, http.StatusOK, attempt)
		case path == "repos/orang-gaboets/octostate/actions/runs/902/attempts/1/jobs":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 0, "jobs": []any{}})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	collector := attemptCollector{
		client: client, activatedAt: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC),
		seen: make(map[string]watcher.RunAttempt), seenRunNames: make(map[int64]string),
	}
	workflows := map[string]*gh.Workflow{
		"CI":              {ID: gh.Ptr(int64(1)), Name: gh.Ptr("CI"), Path: gh.Ptr(expectedWorkflowPaths["CI"])},
		"CodeQL Security": {ID: gh.Ptr(int64(2)), Name: gh.Ptr("CodeQL Security"), Path: gh.Ptr(expectedWorkflowPaths["CodeQL Security"])},
	}
	events := map[int64]triggerEvent{
		901: {RunID: 901, WorkflowName: "CI", IssueNumber: 7},
		902: {RunID: 902, WorkflowName: "CodeQL Security"},
	}
	if err := collector.collect(context.Background(), workflows, time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC), events); err != nil {
		t.Fatalf("collect() error = %v, want other workflow collection to continue", err)
	}
	if missingCalls.Load() != 1 || len(collector.unavailable) != 1 || collector.unavailable[0].Event.RunID != 901 || collector.unavailable[0].StatusCode != http.StatusGone || collector.unavailable[0].Event.IssueNumber != 7 {
		t.Fatalf("missing run calls/unavailable = %d/%#v", missingCalls.Load(), collector.unavailable)
	}
	if len(collector.result) != 1 || collector.result[0].RunID != 902 || collector.result[0].WorkflowName != "CodeQL Security" {
		t.Fatalf("collected attempts = %#v, want CodeQL run 902 despite CI run 901 being gone", collector.result)
	}
}

func TestUnavailableWorkflowRunStatus(t *testing.T) {
	for status, want := range map[int]bool{
		http.StatusNotFound:            true,
		http.StatusGone:                true,
		http.StatusForbidden:           false,
		http.StatusInternalServerError: false,
	} {
		if got := unavailableWorkflowRunStatus(status); got != want {
			t.Errorf("unavailableWorkflowRunStatus(%d) = %t, want %t", status, got, want)
		}
	}
}

func TestPendingRunEventsAcceptsBacklogsAcrossIncidents(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(100, 100, started)
	firstIDs := make([]int64, 26)
	for i := range firstIDs {
		firstIDs[i] = int64(1000 + i)
	}
	firstBody, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{failure}, Latest: failure, PendingRunIDs: firstIDs,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	secondFailure := failure
	secondFailure.WorkflowID = 2
	secondFailure.WorkflowName = "CodeQL Security"
	secondIDs := make([]int64, 25)
	for i := range secondIDs {
		secondIDs[i] = int64(2000 + i)
	}
	secondBody, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 2, WorkflowName: "CodeQL Security", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{secondFailure}, Latest: secondFailure, PendingRunIDs: secondIDs,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	events, deferred, err := pendingRunEvents(repo, expectedAppSlug+"[bot]", []watcher.IssueSnapshot{
		{Number: 7, State: "open", Body: firstBody},
		{Number: 8, State: "open", Body: secondBody},
	})
	if err != nil {
		t.Fatalf("pendingRunEvents() error = %v, want valid cross-incident backlog", err)
	}
	if len(events) != 51 {
		t.Fatalf("pendingRunEvents() returned %d events, want 51", len(events))
	}
	if len(deferred) != 0 {
		t.Fatalf("pendingRunEvents() deferred %d events, want none", len(deferred))
	}
	if events[0] != (triggerEvent{RunID: 1000, WorkflowName: "CI", IssueNumber: 7}) || events[25] != (triggerEvent{RunID: 1025, WorkflowName: "CI", IssueNumber: 7}) ||
		events[26] != (triggerEvent{RunID: 2000, WorkflowName: "CodeQL Security", IssueNumber: 8}) || events[50] != (triggerEvent{RunID: 2024, WorkflowName: "CodeQL Security", IssueNumber: 8}) {
		t.Fatalf("pendingRunEvents() returned unexpected boundaries: first=%#v last=%#v", events[0], events[len(events)-1])
	}
}

func TestRunRejectsForgedPersistedLatestAttempt(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	priorFailure := testFailureAttempt(100, 100, now.Add(-time.Hour))
	forgedLatest := testFailureAttempt(999, 999, now.Add(time.Hour))
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{priorFailure, forgedLatest}, Latest: forgedLatest,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}

	var cursorCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "installation/repositories":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "repositories": []any{map[string]any{"id": repo.ID, "full_name": expectedRepository}}})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/workflows":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 7, "workflows": workflowFixtures()})
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/workflows/") && strings.HasSuffix(path, "/runs"):
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 0, "workflow_runs": []any{}})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/runs/999/attempts/1":
			cursorCalls.Add(1)
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "workflow run attempt not found"})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues":
			writeJSON(w, http.StatusOK, []any{issueJSON(7, "open", "incident", body, []string{watcher.ManagedLabel, watcher.FailingLabel})})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, []any{})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	eventPath := t.TempDir() + "/event.json"
	if err := os.WriteFile(eventPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"GITHUB_REPOSITORY": expectedRepository, "GITHUB_REPOSITORY_ID": "99", "GITHUB_EVENT_NAME": "schedule",
		"GITHUB_EVENT_PATH": eventPath, "GITHUB_REF": "refs/heads/main", "GITHUB_API_URL": server.URL,
		"ACTIONS_WATCHER_TOKEN": "installation-token", "ACTIONS_WATCHER_APP_SLUG": expectedAppSlug,
		"ACTIONS_WATCHER_ACTIVATED_AT": "2026-11-24T00:00:00Z",
	}
	var stdout bytes.Buffer
	err = run(context.Background(), func(key string) string { return values[key] }, &stdout, func() time.Time { return now })
	if err == nil || !strings.Contains(err.Error(), "verify persisted latest attempt") {
		t.Fatalf("run() error = %v, want persisted latest attempt verification failure", err)
	}
	if cursorCalls.Load() != 1 || stdout.Len() != 0 {
		t.Fatalf("cursor calls/stdout = %d/%q, want one verification request and no success output", cursorCalls.Load(), stdout.String())
	}
}

//nolint:gocyclo // The API route branches exercise unavailable evidence and archive refresh together.
func TestRunRefreshesUnavailableFailureBeforeArchiving(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	started := time.Date(2026, 11, 25, 10, 1, 0, 0, time.UTC)
	history := make([]watcher.RunAttempt, 0)
	var body string
	for i := 0; i < 100; i++ {
		runID := int64(1001 + i)
		failure := testFailureAttempt(runID, int(runID), started)
		failure.HeadSHA = strings.Repeat("x", 900)
		if i == 0 {
			failure.JobDataAvailable = false
			failure.FailedJobs = nil
		}
		candidate := append(append([]watcher.RunAttempt(nil), history...), failure)
		candidateBody, archives, err := watcher.RenderBody(repo, watcher.IncidentState{
			WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
			Failures: candidate, Latest: candidate[len(candidate)-1],
		}, "", nil)
		if err != nil {
			t.Fatalf("RenderBody() for initial history: %v", err)
		}
		if len(archives) != 0 {
			break
		}
		history = candidate
		body = candidateBody
	}
	if len(history) < 2 {
		t.Fatalf("fixture has %d failures before body limit; want at least 2", len(history))
	}
	if body == "" {
		t.Fatal("fixture did not produce a body below the archive limit")
	}
	unavailable := history[0]
	newFailure := testFailureAttempt(9000, 9000, started.Add(time.Hour))
	newFailure.CompletedAt = newFailure.StartedAt.Add(10 * time.Minute)
	newFailure.HeadSHA = strings.Repeat("n", 12000)

	attemptsByID := make(map[int64]watcher.RunAttempt, len(history)+1)
	for _, attempt := range history {
		attemptsByID[attempt.RunID] = attempt
	}
	attemptsByID[newFailure.RunID] = newFailure
	runJSON := func(attempt watcher.RunAttempt) map[string]any {
		value := runAttemptFixture(repo.ID, attempt.RunID, attempt.Attempt, attempt.Conclusion)
		value["run_number"] = attempt.RunNumber
		value["run_started_at"] = attempt.StartedAt.Format(time.RFC3339)
		value["updated_at"] = attempt.CompletedAt.Format(time.RFC3339)
		value["head_sha"] = attempt.HeadSHA
		value["html_url"] = attempt.HTMLURL
		return value
	}
	newRun := runJSON(newFailure)
	issueTitle := "ci(actions): investigate CI workflow failures on main"
	issueLabels := []string{watcher.ManagedLabel, watcher.FailingLabel}
	var comments []any
	var nextCommentID int64 = 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "installation/repositories":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "repositories": []any{map[string]any{"id": repo.ID, "full_name": expectedRepository}}})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/workflows":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 7, "workflows": workflowFixtures()})
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/workflows/") && strings.HasSuffix(path, "/runs"):
			if strings.Contains(path, "/workflows/1/") {
				writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "workflow_runs": []any{newRun}})
			} else {
				writeJSON(w, http.StatusOK, map[string]any{"total_count": 0, "workflow_runs": []any{}})
			}
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/") && strings.Contains(path, "/attempts/") && strings.HasSuffix(path, "/jobs"):
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "jobs": []any{map[string]any{"name": "test", "conclusion": "failure"}}})
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/") && strings.Contains(path, "/attempts/"):
			parts := strings.Split(path, "/")
			runID, err := strconv.ParseInt(parts[5], 10, 64)
			if err != nil {
				t.Errorf("invalid run ID in request %q: %v", path, err)
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid run ID"})
				return
			}
			attempt, ok := attemptsByID[runID]
			if !ok {
				t.Errorf("unexpected attempt request for run %d", runID)
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "unknown run"})
				return
			}
			writeJSON(w, http.StatusOK, runJSON(attempt))
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues":
			writeJSON(w, http.StatusOK, []any{issueJSON(7, "open", issueTitle, body, issueLabels)})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, comments)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, issueJSON(7, "open", issueTitle, body, issueLabels))
		case r.Method == http.MethodPost && path == "repos/orang-gaboets/octostate/issues/7/comments":
			var request struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode comment request: %v", err)
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid comment"})
				return
			}
			comment := map[string]any{"id": nextCommentID, "body": request.Body, "user": map[string]any{"login": expectedAppSlug + "[bot]"}}
			nextCommentID++
			comments = append(comments, comment)
			writeJSON(w, http.StatusCreated, comment)
		case r.Method == http.MethodPatch && path == "repos/orang-gaboets/octostate/issues/7":
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode issue update: %v", err)
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid issue update"})
				return
			}
			issueTitle, body, issueLabels = request.Title, request.Body, request.Labels
			writeJSON(w, http.StatusOK, issueJSON(7, "open", issueTitle, body, issueLabels))
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	eventPath := t.TempDir() + "/event.json"
	if err := os.WriteFile(eventPath, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{
		"GITHUB_REPOSITORY": expectedRepository, "GITHUB_REPOSITORY_ID": "99", "GITHUB_EVENT_NAME": "schedule",
		"GITHUB_EVENT_PATH": eventPath, "GITHUB_REF": "refs/heads/main", "GITHUB_API_URL": server.URL,
		"ACTIONS_WATCHER_TOKEN": "installation-token", "ACTIONS_WATCHER_APP_SLUG": expectedAppSlug,
		"ACTIONS_WATCHER_ACTIVATED_AT": "2026-11-24T00:00:00Z",
	}
	var stdout bytes.Buffer
	err := run(context.Background(), func(key string) string { return values[key] }, &stdout, func() time.Time { return now })
	if err != nil {
		t.Fatalf("run() error = %v", err)
	}
	foundRefreshedArchive := false
	for _, raw := range comments {
		comment, _ := raw.(map[string]any)
		text, _ := comment["body"].(string)
		if strings.Contains(text, fmt.Sprintf("run ID %d, attempt 1", unavailable.RunID)) {
			foundRefreshedArchive = strings.Contains(text, "failed jobs: `test`")
		}
	}
	if !foundRefreshedArchive {
		t.Fatalf("archive did not preserve newly available job evidence for run %d", unavailable.RunID)
	}
}

func TestVerifyPersistedAttemptAcceptsMatchingGitHubMetadata(t *testing.T) {
	const repoID = 99
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		if r.Method != http.MethodGet || path != "repos/orang-gaboets/octostate/actions/runs/101/attempts/1" {
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
			return
		}
		writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 101, 1, "failure"))
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	started := time.Date(2026, 11, 25, 10, 1, 0, 0, time.UTC)
	expected := testFailureAttempt(101, 101, started)
	expected.CompletedAt = started.Add(30 * time.Second)
	if err := client.verifyPersistedAttempt(context.Background(), expected); err != nil {
		t.Fatalf("verifyPersistedAttempt() error = %v", err)
	}
}

//nolint:gocyclo // The mock API routes assert checkpoint, transition, and retry ordering.
func TestApplyPlanCheckpointsPendingRunBeforeTransitionComment(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	first := testFailureAttempt(100, 100, started)
	const fullPendingBatch = watcher.MaxPendingRunIDs
	pendingIDs := make([]int64, fullPendingBatch)
	for i := range pendingIDs {
		pendingIDs[i] = int64(200 + i)
	}
	oldBody, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{first}, Latest: first, PendingRunIDs: pendingIDs,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	second := testFailureAttempt(101, 101, started.Add(time.Hour))
	triggerRunID := int64(10000)
	targetBody, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{first, second}, Latest: second,
	}, oldBody, nil)
	if err != nil {
		t.Fatal(err)
	}
	initialIssue := watcher.IssueSnapshot{
		Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "incident",
		Body: oldBody, Labels: []string{watcher.ManagedLabel, watcher.FailingLabel},
	}
	apiIssue := issueSnapshotJSON(initialIssue)
	var writes []string
	var apiComments []any
	nextCommentID := 1
	failTransitionComment := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, apiComments)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, apiIssue)
		case r.Method == http.MethodPatch && path == "repos/orang-gaboets/octostate/issues/7":
			writes = append(writes, "patch")
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode issue patch: %v", err)
			}
			apiIssue["title"], apiIssue["body"], apiIssue["labels"] = request.Title, request.Body, labelObjects(request.Labels)
			writeJSON(w, http.StatusOK, apiIssue)
		case r.Method == http.MethodPost && path == "repos/orang-gaboets/octostate/issues/7/comments":
			var request struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode comment request: %v", err)
			}
			if strings.HasPrefix(request.Body, pendingCheckpointPrefix) {
				writes = append(writes, "checkpoint-comment")
				comment := map[string]any{"id": nextCommentID, "body": request.Body, "user": map[string]any{"login": expectedAppSlug + "[bot]"}}
				nextCommentID++
				apiComments = append(apiComments, comment)
				writeJSON(w, http.StatusCreated, comment)
				return
			}
			writes = append(writes, "transition-comment")
			if failTransitionComment {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "injected comment failure"})
				return
			}
			comment := map[string]any{"id": nextCommentID, "body": request.Body, "user": map[string]any{"login": expectedAppSlug + "[bot]"}}
			nextCommentID++
			apiComments = append(apiComments, comment)
			writeJSON(w, http.StatusCreated, comment)
		case r.Method == http.MethodDelete && path == "repos/orang-gaboets/octostate/issues/comments/1":
			writes = append(writes, "delete-checkpoint-comment")
			apiComments = apiComments[1:]
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, repo)
	transitionMarker := "<!-- actions-watcher:transition:v1 test -->"
	change := watcher.IssueChange{
		Action: watcher.ActionUpdate, WorkflowID: 1, WorkflowName: "CI", IssueNumber: 7,
		Title: "incident", Body: targetBody, CurrentStatus: watcher.StatusFailing,
		PendingRunIDs: append(append([]int64(nil), pendingIDs...), triggerRunID),
		Comments:      []watcher.ManagedComment{{Marker: transitionMarker, Body: transitionMarker + "\ntransition"}},
	}
	err = client.applyPlan(context.Background(), watcher.Plan{Changes: []watcher.IssueChange{change}}, []watcher.IssueSnapshot{initialIssue})
	if err == nil || !strings.Contains(err.Error(), "injected comment failure") {
		t.Fatalf("applyPlan() error = %v, want injected comment failure", err)
	}
	state, err := watcher.RenderBodyState(apiIssue["body"].(string), repo, 1, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(state.PendingRunIDs, pendingIDs) {
		t.Fatalf("persisted PendingRunIDs = %v, want %v", state.PendingRunIDs, pendingIDs)
	}
	if got := len(apiIssue["body"].(string)); got > watcher.MaxIssueBodyBytes {
		t.Fatalf("checkpoint body is %d bytes, exceeds %d", got, watcher.MaxIssueBodyBytes)
	}
	if len(apiComments) != 1 {
		t.Fatalf("persisted checkpoint comments = %d, want one", len(apiComments))
	}
	comment, _ := apiComments[0].(map[string]any)
	marker, runIDs, found, err := parsePendingCheckpoint(comment["body"].(string))
	if err != nil || !found || marker != 1 || !reflect.DeepEqual(runIDs, []int64{triggerRunID}) {
		t.Fatalf("persisted pending checkpoint = workflow %d, IDs %v, found %t, err %v", marker, runIDs, found, err)
	}
	if !reflect.DeepEqual(writes, []string{"checkpoint-comment", "transition-comment"}) {
		t.Fatalf("write order = %v, want checkpoint marker before transition comment", writes)
	}

	issueComments := []watcher.CommentSnapshot{{ID: 1, AuthorLogin: expectedAppSlug + "[bot]", Body: comment["body"].(string)}}
	events, deferred, err := pendingRunEvents(repo, expectedAppSlug+"[bot]", []watcher.IssueSnapshot{{
		Number: 7, State: "open", Body: apiIssue["body"].(string), Comments: issueComments,
	}})
	if err != nil {
		t.Fatalf("pendingRunEvents() after partial apply = %v", err)
	}
	const expectedBatchSize = 1000
	if len(events) != expectedBatchSize || len(deferred) != 2 || deferred[1].RunID != triggerRunID {
		t.Fatalf("recovered pending batch = %d selected, deferred %#v; want %d and trigger %d deferred", len(events), deferred, expectedBatchSize, triggerRunID)
	}
	deferredChange := change
	deferredChange.Body = targetBody
	deferredChange.PendingRunIDs = nil
	for _, event := range events {
		deferredChange.PendingRunIDs = append(deferredChange.PendingRunIDs, event.RunID)
	}
	retryPlan := watcher.Plan{Changes: []watcher.IssueChange{deferredChange}}
	if err := preserveDeferredRunEvents(&retryPlan, deferred, []watcher.IssueSnapshot{{
		Number: 7, State: "open", Body: apiIssue["body"].(string), Comments: issueComments,
	}}, repo, expectedAppSlug+"[bot]"); err != nil {
		t.Fatalf("preserveDeferredRunEvents() after partial apply: %v", err)
	}
	retryChange := retryPlan.Changes[0]
	deferredState, err := watcher.RenderBodyState(retryChange.Body, repo, 1, "CI")
	if err != nil {
		t.Fatal(err)
	}
	wantDeferredIDs := []int64{pendingIDs[len(pendingIDs)-1], triggerRunID}
	if !reflect.DeepEqual(deferredState.PendingRunIDs, wantDeferredIDs) {
		t.Fatalf("retry plan pending IDs = %v, want %v", deferredState.PendingRunIDs, wantDeferredIDs)
	}

	failTransitionComment = false
	retryIssue := initialIssue
	retryIssue.Body = apiIssue["body"].(string)
	retryIssue.Comments = issueComments
	if err := client.applyPlan(context.Background(), watcher.Plan{Changes: []watcher.IssueChange{retryChange}}, []watcher.IssueSnapshot{retryIssue}); err != nil {
		t.Fatalf("applyPlan() retry: %v", err)
	}
	finalState, err := watcher.RenderBodyState(apiIssue["body"].(string), repo, 1, "CI")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(finalState.PendingRunIDs, wantDeferredIDs) {
		t.Fatalf("final PendingRunIDs = %v, want %v", finalState.PendingRunIDs, wantDeferredIDs)
	}
	if len(apiComments) != 1 || strings.HasPrefix(apiComments[0].(map[string]any)["body"].(string), pendingCheckpointPrefix) {
		t.Fatalf("comments after successful retry = %#v, want only the transition comment", apiComments)
	}
}

//nolint:gocyclo // The mock API routes cover checkpoint and archive failure recovery in one scenario.
func TestApplyPlanPersistsNewRunBeforeCheckpointArchiveWrite(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	first := testFailureAttempt(100, 100, started)
	second := testFailureAttempt(101, 101, started.Add(time.Hour))
	state := watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{first, second}, Latest: second,
	}
	body, _, err := watcher.RenderBody(repo, state, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	candidate := body + strings.Repeat("x", watcher.MaxIssueBodyBytes-len(body)-1)
	checkpointBody, checkpointArchives, err := watcher.RenderCheckpointBody(candidate, repo, []int64{9000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpointArchives) == 0 {
		t.Fatal("fixture did not require checkpoint-time archiving")
	}
	body = candidate

	issue := watcher.IssueSnapshot{
		Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "incident",
		Body: body, Labels: []string{watcher.ManagedLabel, watcher.FailingLabel},
	}
	apiIssue := issueSnapshotJSON(issue)
	comments := make([]any, 0)
	nextCommentID := int64(500)
	failArchive := true
	var writes []string
	attempts := map[int64]watcher.RunAttempt{100: first, 101: second}
	runJSON := func(attempt watcher.RunAttempt) map[string]any {
		value := runAttemptFixture(repo.ID, attempt.RunID, attempt.Attempt, attempt.Conclusion)
		value["run_number"] = attempt.RunNumber
		value["run_started_at"] = attempt.StartedAt.Format(time.RFC3339)
		value["updated_at"] = attempt.CompletedAt.Format(time.RFC3339)
		value["head_sha"] = attempt.HeadSHA
		value["html_url"] = attempt.HTMLURL
		return value
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/") && strings.Contains(path, "/attempts/") && strings.HasSuffix(path, "/jobs"):
			parts := strings.Split(path, "/")
			runID, _ := strconv.ParseInt(parts[5], 10, 64)
			attempt := attempts[runID]
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "jobs": []any{map[string]any{"name": attempt.FailedJobs[0], "conclusion": "failure"}}})
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/") && strings.Contains(path, "/attempts/"):
			parts := strings.Split(path, "/")
			runID, _ := strconv.ParseInt(parts[5], 10, 64)
			writeJSON(w, http.StatusOK, runJSON(attempts[runID]))
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, comments)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, apiIssue)
		case r.Method == http.MethodPost && path == "repos/orang-gaboets/octostate/issues/7/comments":
			var request struct {
				Body string `json:"body"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&request); decodeErr != nil {
				t.Errorf("decode comment request: %v", decodeErr)
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid comment"})
				return
			}
			if strings.Contains(request.Body, "actions-watcher:archive:v1") {
				writes = append(writes, "archive-comment")
				if failArchive {
					writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "injected archive failure"})
					return
				}
			} else {
				writes = append(writes, "checkpoint-comment")
			}
			comments = append(comments, map[string]any{
				"id": nextCommentID, "body": request.Body,
				"user": map[string]any{"login": expectedAppSlug + "[bot]"},
			})
			nextCommentID++
			writeJSON(w, http.StatusCreated, comments[len(comments)-1])
		case r.Method == http.MethodDelete && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/comments/"):
			commentID, _ := strconv.ParseInt(strings.TrimPrefix(path, "repos/orang-gaboets/octostate/issues/comments/"), 10, 64)
			for i, raw := range comments {
				comment, _ := raw.(map[string]any)
				id, _ := comment["id"].(int64)
				if id == commentID {
					comments = append(comments[:i], comments[i+1:]...)
					break
				}
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPatch && path == "repos/orang-gaboets/octostate/issues/7":
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if decodeErr := json.NewDecoder(r.Body).Decode(&request); decodeErr != nil {
				t.Errorf("decode issue patch: %v", decodeErr)
				writeJSON(w, http.StatusBadRequest, map[string]any{"message": "invalid issue patch"})
				return
			}
			apiIssue["title"], apiIssue["body"], apiIssue["labels"] = request.Title, request.Body, labelObjects(request.Labels)
			writeJSON(w, http.StatusOK, apiIssue)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, repo)
	change := watcher.IssueChange{
		Action: watcher.ActionUpdate, WorkflowID: 1, WorkflowName: "CI", IssueNumber: 7,
		Title: "incident", Body: checkpointBody, CurrentStatus: watcher.StatusFailing,
		PendingRunIDs: []int64{9000},
	}
	err = client.applyPlan(context.Background(), watcher.Plan{Changes: []watcher.IssueChange{change}}, []watcher.IssueSnapshot{issue})
	if err == nil || !strings.Contains(err.Error(), "injected archive failure") {
		t.Fatalf("applyPlan() error = %v, want injected archive failure", err)
	}
	if !reflect.DeepEqual(writes, []string{"checkpoint-comment", "archive-comment"}) {
		t.Fatalf("write order = %v, want checkpoint comment before archive comment", writes)
	}
	current, err := client.getIssueSnapshot(context.Background(), issue.Number)
	if err != nil {
		t.Fatal(err)
	}
	events, deferred, err := pendingRunEvents(repo, expectedAppSlug+"[bot]", []watcher.IssueSnapshot{current})
	if err != nil {
		t.Fatal(err)
	}
	want := []triggerEvent{{RunID: 9000, WorkflowName: "CI", IssueNumber: 7}}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("pendingRunEvents() = %#v, want %#v", events, want)
	}
	if len(deferred) != 0 {
		t.Fatalf("pendingRunEvents() deferred = %#v, want none", deferred)
	}
	failArchive = false
	if err := client.applyPlan(context.Background(), watcher.Plan{Changes: []watcher.IssueChange{change}}, []watcher.IssueSnapshot{current}); err != nil {
		t.Fatalf("applyPlan() retry after checkpoint archive failure: %v", err)
	}
	current, err = client.getIssueSnapshot(context.Background(), issue.Number)
	if err != nil {
		t.Fatal(err)
	}
	if current.Body != checkpointBody {
		t.Fatal("successful retry did not persist checkpointed body")
	}
	for _, comment := range current.Comments {
		if strings.Contains(comment.Body, pendingCheckpointPrefix) {
			t.Fatalf("successful retry left transient checkpoint comment: %q", comment.Body)
		}
	}
}

func TestVerifyOpenIncidentCursorRejectsTamperedLatestFailureJobs(t *testing.T) {
	const repoID = 99
	var attemptCalls atomic.Int32
	var jobCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch path {
		case "repos/orang-gaboets/octostate/actions/runs/101/attempts/1":
			attemptCalls.Add(1)
			writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 101, 1, "failure"))
		case "repos/orang-gaboets/octostate/actions/runs/101/attempts/1/jobs":
			jobCalls.Add(1)
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "jobs": []any{map[string]any{"name": "test", "conclusion": "failure"}}})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	repo := watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"}
	client := testWatcherClient(t, server.URL, repo)
	started := time.Date(2026, 11, 25, 10, 1, 0, 0, time.UTC)
	latest := testFailureAttempt(101, 101, started)
	latest.CompletedAt = started.Add(30 * time.Second)
	latest.FailedJobs = []string{"forged job"}
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{latest}, Latest: latest,
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	err = client.verifyOpenIncidentCursors(context.Background(), []watcher.IssueSnapshot{{Number: 7, State: "open", Body: body}})
	if err == nil || !strings.Contains(err.Error(), "job metadata") {
		t.Fatalf("verifyOpenIncidentCursors() error = %v, want persisted job metadata mismatch", err)
	}
	if attemptCalls.Load() != 1 || jobCalls.Load() != 1 {
		t.Fatalf("attempt/job API calls = %d/%d, want 1/1", attemptCalls.Load(), jobCalls.Load())
	}
}

func TestVerifyOpenIncidentCursorAuthenticatesRecoverySeparateFromOrderingCursor(t *testing.T) {
	const repoID = 99
	var recoveryCalls, latestCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch path {
		case "repos/orang-gaboets/octostate/actions/runs/102/attempts/1":
			recoveryCalls.Add(1)
			writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 102, 1, "success"))
		case "repos/orang-gaboets/octostate/actions/runs/103/attempts/1":
			latestCalls.Add(1)
			writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 103, 1, "success"))
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	repo := watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"}
	client := testWatcherClient(t, server.URL, repo)
	started := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	failure := testFailureAttempt(101, 101, started)
	recovery := testFailureAttempt(102, 102, started.Add(time.Minute))
	recovery.Conclusion = "success"
	recovery.CompletedAt = recovery.StartedAt.Add(30 * time.Second)
	latest := testFailureAttempt(103, 103, started.Add(time.Minute))
	latest.Conclusion = "success"
	latest.CompletedAt = latest.StartedAt.Add(30 * time.Second)
	forgedRecovery := recovery
	forgedRecovery.CompletedAt = forgedRecovery.CompletedAt.Add(time.Minute)
	body, _, err := watcher.RenderBody(repo, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusRecovered,
		Failures: []watcher.RunAttempt{failure}, Latest: latest, Recovery: &forgedRecovery,
	}, "", nil)
	if err != nil {
		t.Fatalf("RenderBody() for recovered state: %v", err)
	}
	err = client.verifyOpenIncidentCursors(context.Background(), []watcher.IssueSnapshot{{Number: 7, State: "open", Body: body}})
	if err == nil || !strings.Contains(err.Error(), "verify persisted recovery attempt") {
		t.Fatalf("verifyOpenIncidentCursors() error = %v, want tampered recovery rejection", err)
	}
	if recoveryCalls.Load() != 1 || latestCalls.Load() != 1 {
		t.Fatalf("recovery/latest API calls = %d/%d, want 1/1", recoveryCalls.Load(), latestCalls.Load())
	}
}

func TestListFailedJobsDeduplicatesRepeatedNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		if path != "repos/orang-gaboets/octostate/actions/runs/101/attempts/1/jobs" {
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"total_count": 2, "jobs": []any{
			map[string]any{"name": "test", "conclusion": "failure"},
			map[string]any{"name": "test", "conclusion": "failure"},
		}})
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})
	jobs, available, err := client.listFailedJobs(context.Background(), 101, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !available || !reflect.DeepEqual(jobs, []string{"test"}) {
		t.Fatalf("listFailedJobs() = (%v, %t), want ([test], true)", jobs, available)
	}
}

func TestVerifyPersistedFailureMatchesRunAndJobMetadata(t *testing.T) {
	const repoID = 99
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch path {
		case "repos/orang-gaboets/octostate/actions/runs/101/attempts/1":
			writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 101, 1, "failure"))
		case "repos/orang-gaboets/octostate/actions/runs/101/attempts/1/jobs":
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "jobs": []any{map[string]any{"name": "test", "conclusion": "failure"}}})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	started := time.Date(2026, 11, 25, 10, 1, 0, 0, time.UTC)
	expected := testFailureAttempt(101, 101, started)
	expected.CompletedAt = started.Add(30 * time.Second)
	if err := client.verifyPersistedFailure(context.Background(), expected); err != nil {
		t.Fatalf("verifyPersistedFailure() error = %v", err)
	}

	tampered := expected
	tampered.FailedJobs = []string{"forged job"}
	if err := client.verifyPersistedFailure(context.Background(), tampered); err == nil || !strings.Contains(err.Error(), "job metadata") {
		t.Fatalf("verifyPersistedFailure() error = %v, want job metadata mismatch", err)
	}
}

func TestVerifyPersistedLatestFailureAllowsUnavailableJobsToResolve(t *testing.T) {
	const repoID = 99
	var jobCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch path {
		case "repos/orang-gaboets/octostate/actions/runs/101/attempts/1":
			writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 101, 1, "failure"))
		case "repos/orang-gaboets/octostate/actions/runs/101/attempts/1/jobs":
			if jobCalls.Add(1) == 1 {
				writeJSON(w, http.StatusGone, map[string]any{"message": "job data expired"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 1, "jobs": []any{map[string]any{"name": "test", "conclusion": "failure"}}})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	started := time.Date(2026, 11, 25, 10, 1, 0, 0, time.UTC)
	expected := testFailureAttempt(101, 101, started)
	expected.CompletedAt = started.Add(30 * time.Second)
	expected.JobDataAvailable = false
	expected.FailedJobs = nil
	if err := client.verifyPersistedFailure(context.Background(), expected); err != nil {
		t.Fatalf("verifyPersistedFailure() with unavailable API data error = %v", err)
	}
	if err := client.verifyPersistedFailure(context.Background(), expected); err == nil || !strings.Contains(err.Error(), "job metadata") {
		t.Fatalf("verifyPersistedFailure() accepted unresolved data that is now available: %v", err)
	}
	if err := client.verifyPersistedLatestFailure(context.Background(), expected); err != nil {
		t.Fatalf("verifyPersistedLatestFailure() with later-available data error = %v", err)
	}

	forgedUnavailable := expected
	forgedUnavailable.FailedJobs = []string{"forged job"}
	if err := client.verifyPersistedFailure(context.Background(), forgedUnavailable); err == nil || !strings.Contains(err.Error(), "job metadata") {
		t.Fatalf("verifyPersistedFailure() accepted unavailable record with job names: %v", err)
	}
}

func TestApplyPlanRejectsUnverifiedArchiveBeforeMutation(t *testing.T) {
	const repoID = 99
	var attemptCalls atomic.Int32
	var mutationCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		if path == "repos/orang-gaboets/octostate/actions/runs/999/attempts/1" {
			attemptCalls.Add(1)
			writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 999, 1, "success"))
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPatch {
			mutationCalls.Add(1)
		}
		t.Errorf("unexpected API request before archive verification: %s %s", r.Method, r.URL.String())
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	started := time.Date(2026, 11, 25, 10, 1, 0, 0, time.UTC)
	forged := testFailureAttempt(999, 999, started)
	forged.CompletedAt = started.Add(30 * time.Second)
	plan := watcher.Plan{Changes: []watcher.IssueChange{{
		Action: watcher.ActionUpdate, IssueNumber: 7,
		Archives: []watcher.ArchiveComment{{Attempts: []watcher.RunAttempt{forged}}},
	}}}
	err := client.applyPlan(context.Background(), plan, []watcher.IssueSnapshot{{Number: 7, State: "open"}})
	if err == nil || !strings.Contains(err.Error(), "before archiving issue #7") {
		t.Fatalf("applyPlan() error = %v, want archive verification failure", err)
	}
	if attemptCalls.Load() != 1 || mutationCalls.Load() != 0 {
		t.Fatalf("attempt/mutation API calls = %d/%d, want 1/0", attemptCalls.Load(), mutationCalls.Load())
	}
}

func TestReadTriggerEventRequiresCompletedAllowlistedWorkflowRun(t *testing.T) {
	path := t.TempDir() + "/event.json"
	for _, tc := range []struct {
		name    string
		payload string
		wantErr bool
	}{
		{name: "completed allowlisted", payload: `{"action":"completed","workflow_run":{"id":123,"name":"CI"}}`},
		{name: "wrong action", payload: `{"action":"requested","workflow_run":{"id":123,"name":"CI"}}`, wantErr: true},
		{name: "unknown workflow", payload: `{"action":"completed","workflow_run":{"id":123,"name":"Untrusted"}}`, wantErr: true},
		{name: "missing run ID", payload: `{"action":"completed","workflow_run":{"name":"CI"}}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.payload), 0o600); err != nil {
				t.Fatal(err)
			}
			trigger, err := readTriggerEvent("workflow_run", path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("readTriggerEvent() error = %v, wantErr=%v", err, tc.wantErr)
			}
			if !tc.wantErr && (trigger.RunID != 123 || trigger.WorkflowName != "CI") {
				t.Fatalf("readTriggerEvent() = %#v, want run 123 for CI", trigger)
			}
		})
	}
}

func TestReadTriggerEventDoesNotParsePushPayload(t *testing.T) {
	path := t.TempDir() + "/event.json"
	if err := os.WriteFile(path, []byte(strings.Repeat("x", (1<<20)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	trigger, err := readTriggerEvent("push", path)
	if err != nil {
		t.Fatalf("readTriggerEvent() error = %v, want push payload to be ignored", err)
	}
	if trigger != (triggerEvent{}) {
		t.Fatalf("readTriggerEvent() = %#v, want empty trigger", trigger)
	}
}

func TestResultForEmitsStableMachineSummary(t *testing.T) {
	plan := watcher.Plan{Changes: []watcher.IssueChange{{
		Action: watcher.ActionCreate, WorkflowName: "CI", PreviousIssueNumber: 49,
		CurrentStatus: watcher.StatusFailing, FailureCount: 2,
		Body: "body content is intentionally omitted from the command summary",
	}}}
	first, err := json.Marshal(resultFor(3, plan))
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal(resultFor(3, plan))
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("command summary changed between identical inputs: %s != %s", first, second)
	}
	if got := string(first); got != `{"repository":"orang-gaboets/octostate","attempts":3,"changes":[{"action":"create","workflow":"CI","previous_issue":49,"status":"failing","failures":2}]}` {
		t.Fatalf("summary JSON = %s", got)
	}
}

type collectorCallCounts struct {
	workflowPages atomic.Int32
	runPages      atomic.Int32
	attempts      atomic.Int32
	jobs          atomic.Int32
}

func newPagedCollectorServer(t *testing.T, repoID int64) (*httptest.Server, *collectorCallCounts) {
	t.Helper()
	calls := &collectorCallCounts{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want installation bearer token", got)
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case path == "installation/repositories":
			writeJSON(w, 200, map[string]any{"total_count": 1, "repositories": []any{map[string]any{"id": repoID, "full_name": expectedRepository}}})
		case path == "repos/orang-gaboets/octostate/actions/workflows":
			serveWorkflowPage(calls, w, r)
		case strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/workflows/") && strings.HasSuffix(path, "/runs"):
			serveRunPage(calls, repoID, w, r, path)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/actions/runs/103":
			serveTriggerRun(repoID, w)
		case strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/") && strings.Contains(path, "/attempts/") && !strings.HasSuffix(path, "/jobs"):
			serveRunAttempt(calls, repoID, w, path)
		case strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/") && strings.HasSuffix(path, "/jobs"):
			serveJobs(calls, w, path)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, 404, map[string]any{"message": "unexpected route"})
		}
	}))
	return server, calls
}

func serveWorkflowPage(calls *collectorCallCounts, w http.ResponseWriter, r *http.Request) {
	calls.workflowPages.Add(1)
	workflows := workflowFixtures()
	if r.URL.Query().Get("page") == "1" {
		w.Header().Set("Link", `</api/v3/repos/orang-gaboets/octostate/actions/workflows?per_page=100&page=2>; rel="next"`)
		writeJSON(w, 200, map[string]any{"total_count": 7, "workflows": workflows[:6]})
		return
	}
	writeJSON(w, 200, map[string]any{"total_count": 7, "workflows": workflows[6:]})
}

func serveRunPage(calls *collectorCallCounts, repoID int64, w http.ResponseWriter, r *http.Request, path string) {
	id := strings.TrimSuffix(strings.TrimPrefix(path, "repos/orang-gaboets/octostate/actions/workflows/"), "/runs")
	if id != "1" {
		writeJSON(w, 200, map[string]any{"total_count": 0, "workflow_runs": []any{}})
		return
	}
	calls.runPages.Add(1)
	if r.URL.Query().Get("page") == "1" {
		w.Header().Set("Link", `</api/v3/repos/orang-gaboets/octostate/actions/workflows/1/runs?per_page=100&page=2>; rel="next"`)
		writeJSON(w, 200, map[string]any{"total_count": 2, "workflow_runs": []any{runFixture(repoID, 101, 2, "failure")}})
		return
	}
	writeJSON(w, 200, map[string]any{"total_count": 2, "workflow_runs": []any{runFixture(repoID, 102, 1, "failure")}})
}

func serveTriggerRun(repoID int64, w http.ResponseWriter) {
	oldRun := runFixture(repoID, 103, 1, "failure")
	oldRun["created_at"] = "2020-01-01T00:00:00Z"
	writeJSON(w, http.StatusOK, oldRun)
}

func serveRunAttempt(calls *collectorCallCounts, repoID int64, w http.ResponseWriter, path string) {
	calls.attempts.Add(1)
	parts := strings.Split(path, "/")
	runID, _ := strconv.ParseInt(parts[5], 10, 64)
	attemptNumber, _ := strconv.Atoi(parts[7])
	writeJSON(w, 200, runAttemptFixture(repoID, runID, attemptNumber, "failure"))
}

func serveJobs(calls *collectorCallCounts, w http.ResponseWriter, path string) {
	calls.jobs.Add(1)
	if strings.Contains(path, "/102/attempts/1/jobs") {
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "job data expired"})
		return
	}
	writeJSON(w, 200, map[string]any{"total_count": 0, "jobs": []any{}})
}

func TestCollectAttemptsPaginatesWorkflowsAndRunsAndReadsEveryAttempt(t *testing.T) {
	const repoID = 99
	server, calls := newPagedCollectorServer(t, repoID)
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	activatedAt := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	attempts, _, err := client.collectAttempts(context.Background(), activatedAt, now, []triggerEvent{{RunID: 103, WorkflowName: "CI"}})
	if err != nil {
		t.Fatal(err)
	}
	if calls.workflowPages.Load() != 2 || calls.runPages.Load() != 2 || calls.attempts.Load() != 4 || calls.jobs.Load() != 4 {
		t.Fatalf("API page/attempt/job calls = %d/%d/%d/%d, want 2/2/4/4", calls.workflowPages.Load(), calls.runPages.Load(), calls.attempts.Load(), calls.jobs.Load())
	}
	if len(attempts) != 4 {
		t.Fatalf("collected %d attempts, want four distinct attempts including the old event run", len(attempts))
	}
	if attempts[0].RunID != 101 || attempts[0].Attempt != 1 || !attempts[0].JobDataAvailable || len(attempts[0].FailedJobs) != 0 {
		t.Fatalf("first attempt did not preserve known-empty job data: %#v", attempts[0])
	}
	if attempts[1].RunID != 101 || attempts[1].Attempt != 2 || !attempts[1].JobDataAvailable {
		t.Fatalf("second attempt was not fetched independently: %#v", attempts[1])
	}
	if attempts[2].RunID != 102 || attempts[2].JobDataAvailable {
		t.Fatalf("unavailable job data was conflated with a known-empty result: %#v", attempts[2])
	}
	if attempts[3].RunID != 103 || attempts[3].Conclusion != "failure" {
		t.Fatalf("the triggering run outside the created-time window was not fetched: %#v", attempts[3])
	}
	pendingBody, _, err := watcher.RenderBody(watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"}, watcher.IncidentState{
		WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing,
		Failures: []watcher.RunAttempt{testFailureAttempt(101, 101, now.Add(-time.Hour))},
		Latest:   testFailureAttempt(101, 101, now.Add(-time.Hour)), PendingRunIDs: []int64{103},
	}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	pendingEvents, deferredEvents, err := pendingRunEvents(watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"}, expectedAppSlug+"[bot]", []watcher.IssueSnapshot{{State: "open", Body: pendingBody}})
	if err != nil {
		t.Fatal(err)
	}
	if len(deferredEvents) != 0 {
		t.Fatalf("deferred events = %#v, want none", deferredEvents)
	}
	pendingAttempts, _, err := client.collectAttempts(context.Background(), activatedAt, now, pendingEvents)
	if err != nil {
		t.Fatalf("collectAttempts() with a persisted pending run: %v", err)
	}
	if len(pendingAttempts) != 4 || pendingAttempts[3].RunID != 103 {
		t.Fatalf("persisted pending run was not fetched outside the created-time window: %#v", pendingAttempts)
	}
}

func TestCollectAttemptsRetainsEarlierFailureWhenLatestAttemptIsCancelled(t *testing.T) {
	const repoID = 99
	var attemptCalls atomic.Int32
	var jobCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/101/attempts/") && !strings.HasSuffix(path, "/jobs"):
			attemptCalls.Add(1)
			attemptNumber, err := strconv.Atoi(strings.TrimPrefix(path, "repos/orang-gaboets/octostate/actions/runs/101/attempts/"))
			if err != nil {
				t.Errorf("parse attempt number from %q: %v", path, err)
				return
			}
			conclusion := "failure"
			if attemptNumber == 2 {
				conclusion = "cancelled"
			}
			writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 101, attemptNumber, conclusion))
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/actions/runs/101/attempts/1/jobs"):
			jobCalls.Add(1)
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 0, "jobs": []any{}})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	workflow := &gh.Workflow{ID: gh.Ptr(int64(1)), Name: gh.Ptr("CI"), Path: gh.Ptr(expectedWorkflowPaths["CI"])}
	row := runFixture(repoID, 101, 2, "cancelled")
	rowJSON, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var listed gh.WorkflowRun
	if err := json.Unmarshal(rowJSON, &listed); err != nil {
		t.Fatal(err)
	}
	collector := attemptCollector{
		client: client, activatedAt: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC),
		seen: make(map[string]watcher.RunAttempt), seenRunNames: make(map[int64]string),
	}
	if err := collector.processRun(context.Background(), workflow, &listed); err != nil {
		t.Fatal(err)
	}
	if attemptCalls.Load() != 2 || jobCalls.Load() != 1 {
		t.Fatalf("attempt/job API calls = %d/%d, want 2/1", attemptCalls.Load(), jobCalls.Load())
	}
	if len(collector.result) != 1 || collector.result[0].RunID != 101 || collector.result[0].Attempt != 1 || collector.result[0].Conclusion != "failure" {
		t.Fatalf("collected attempts = %#v, want the earlier failed attempt only", collector.result)
	}
}

func TestCollectAttemptsRetainsCompletedFailureWhileLatestRerunIsInProgress(t *testing.T) {
	const repoID = 99
	var attemptCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/101/attempts/") && !strings.HasSuffix(path, "/jobs"):
			attemptCalls.Add(1)
			attemptNumber, err := strconv.Atoi(strings.TrimPrefix(path, "repos/orang-gaboets/octostate/actions/runs/101/attempts/"))
			if err != nil {
				t.Errorf("parse attempt number from %q: %v", path, err)
				return
			}
			conclusion := "failure"
			if attemptNumber == 2 {
				conclusion = ""
			}
			attempt := runAttemptFixture(repoID, 101, attemptNumber, conclusion)
			if attemptNumber == 2 {
				attempt["status"] = "in_progress"
				delete(attempt, "conclusion")
			}
			writeJSON(w, http.StatusOK, attempt)
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/actions/runs/101/attempts/1/jobs"):
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 0, "jobs": []any{}})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	workflow := &gh.Workflow{ID: gh.Ptr(int64(1)), Name: gh.Ptr("CI"), Path: gh.Ptr(expectedWorkflowPaths["CI"])}
	row := runFixture(repoID, 101, 2, "")
	row["status"] = "in_progress"
	delete(row, "conclusion")
	rowJSON, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var listed gh.WorkflowRun
	if err := json.Unmarshal(rowJSON, &listed); err != nil {
		t.Fatal(err)
	}
	collector := attemptCollector{
		client: client, activatedAt: time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC),
		seen: make(map[string]watcher.RunAttempt), seenRunNames: make(map[int64]string),
	}
	if err := collector.processRun(context.Background(), workflow, &listed); err != nil {
		t.Fatal(err)
	}
	if attemptCalls.Load() != 2 || len(collector.result) != 1 || collector.result[0].Attempt != 1 || collector.result[0].Conclusion != "failure" {
		t.Fatalf("attempt calls/results = %d/%#v, want both attempts read and earlier failure retained", attemptCalls.Load(), collector.result)
	}
}

func TestCollectAttemptsSkipsJobLookupBeforeActivation(t *testing.T) {
	const repoID = 99
	var oldJobCalls atomic.Int32
	var newJobCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/101/attempts/") && !strings.HasSuffix(path, "/jobs"):
			attemptNumber, err := strconv.Atoi(strings.TrimPrefix(path, "repos/orang-gaboets/octostate/actions/runs/101/attempts/"))
			if err != nil {
				t.Errorf("parse attempt number from %q: %v", path, err)
				return
			}
			writeJSON(w, http.StatusOK, runAttemptFixture(repoID, 101, attemptNumber, "failure"))
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/actions/runs/101/attempts/1/jobs"):
			oldJobCalls.Add(1)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "old job data unavailable"})
		case r.Method == http.MethodGet && strings.HasSuffix(path, "/actions/runs/101/attempts/2/jobs"):
			newJobCalls.Add(1)
			writeJSON(w, http.StatusOK, map[string]any{"total_count": 0, "jobs": []any{}})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()

	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	workflow := &gh.Workflow{ID: gh.Ptr(int64(1)), Name: gh.Ptr("CI"), Path: gh.Ptr(expectedWorkflowPaths["CI"])}
	row := runFixture(repoID, 101, 2, "failure")
	rowJSON, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	var listed gh.WorkflowRun
	if err := json.Unmarshal(rowJSON, &listed); err != nil {
		t.Fatal(err)
	}
	activation := time.Date(2026, 11, 25, 10, 1, 30, 0, time.UTC)
	collector := attemptCollector{
		client: client, activatedAt: activation,
		seen: make(map[string]watcher.RunAttempt), seenRunNames: make(map[int64]string),
	}
	if err := collector.processRun(context.Background(), workflow, &listed); err != nil {
		t.Fatal(err)
	}
	if oldJobCalls.Load() != 0 || newJobCalls.Load() != 1 {
		t.Fatalf("old/new job API calls = %d/%d, want 0/1", oldJobCalls.Load(), newJobCalls.Load())
	}
	if len(collector.result) != 1 || collector.result[0].Attempt != 2 {
		t.Fatalf("collected attempts = %#v, want the post-activation attempt only", collector.result)
	}
}

func TestCollectAttemptsRejectsInstallationScopeAndAuthorizationFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantPhrase string
	}{
		{name: "unauthorized token", status: http.StatusUnauthorized, body: `{"message":"Bad credentials"}`, wantPhrase: "verify installation-token repository scope"},
		{name: "wrong repository scope", status: http.StatusOK, body: `{"total_count":1,"repositories":[{"id":101,"full_name":"someone/else"}]}`, wantPhrase: "must be scoped only to"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRaw(w, tc.status, tc.body)
			}))
			defer server.Close()
			client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})
			_, _, err := client.collectAttempts(context.Background(), time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC), nil)
			if err == nil || !strings.Contains(err.Error(), tc.wantPhrase) {
				t.Fatalf("collectAttempts() error = %v, want phrase %q", err, tc.wantPhrase)
			}
		})
	}
}

func TestRateLimitTransportHonorsRetryAfterAndBoundsRetriesToGET(t *testing.T) {
	var calls atomic.Int32
	var waits []time.Duration
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		call := calls.Add(1)
		status := http.StatusOK
		header := make(http.Header)
		body := "{}"
		if call == 1 {
			status = http.StatusTooManyRequests
			header.Set("Retry-After", "2")
			body = `{"message":"secondary rate limit"}`
		}
		return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	transport := &rateLimitTransport{
		base: base, now: func() time.Time { return time.Unix(100, 0) }, maxRetries: 3, maxWait: time.Minute,
		wait: func(_ context.Context, delay time.Duration) error { waits = append(waits, delay); return nil },
	}
	request, _ := http.NewRequest(http.MethodGet, "https://api.github.com/test", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if calls.Load() != 2 || !reflect.DeepEqual(waits, []time.Duration{2 * time.Second}) {
		t.Fatalf("GET calls/waits = %d/%v, want 2/[2s]", calls.Load(), waits)
	}

	calls.Store(0)
	request, _ = http.NewRequest(http.MethodPost, "https://api.github.com/test", strings.NewReader("{}"))
	response, err = transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("POST was retried %d times, want one request", calls.Load())
	}
}

func TestRateLimitTransportReturnsSuccessfulResponseAtQuotaBoundary(t *testing.T) {
	var calls atomic.Int32
	var waits []time.Duration
	transport := &rateLimitTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			header := make(http.Header)
			header.Set("X-RateLimit-Remaining", "0")
			header.Set("X-RateLimit-Reset", "130")
			return &http.Response{StatusCode: http.StatusOK, Header: header, Body: io.NopCloser(strings.NewReader("valid response")), Request: req}, nil
		}),
		wait: func(_ context.Context, delay time.Duration) error { waits = append(waits, delay); return nil },
		now:  func() time.Time { return time.Unix(100, 0) }, maxRetries: 3, maxWait: time.Minute,
	}
	request, _ := http.NewRequest(http.MethodGet, "https://api.github.com/test", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || len(waits) != 0 || string(body) != "valid response" {
		t.Fatalf("successful response calls/waits/body = %d/%v/%q, want 1/[]/valid response", calls.Load(), waits, body)
	}
}

func TestRetryableRateResponseReplaysBodyAndClosesOriginal(t *testing.T) {
	original := &trackedReadCloser{Reader: strings.NewReader(`{"message":"secondary rate limit"}`)}
	response := &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: original}
	if !retryableRateResponse(response) {
		t.Fatal("retryableRateResponse() = false, want secondary rate limit")
	}
	body, err := io.ReadAll(response.Body)
	if err != nil || string(body) != `{"message":"secondary rate limit"}` {
		t.Fatalf("replayed response body = %q, %v", body, err)
	}
	if original.closeCalls.Load() != 0 {
		t.Fatal("peeking at the response closed the original body before the caller")
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if original.closeCalls.Load() != 1 {
		t.Fatalf("original body close calls = %d, want one", original.closeCalls.Load())
	}
}

func TestRetryableRateResponsePreservesBodyAndReadError(t *testing.T) {
	wantErr := fmt.Errorf("injected body read failure")
	original := &trackedReadCloser{Reader: strings.NewReader(`{"message":"ordinary forbidden response"}`), readErr: wantErr}
	response := &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header), Body: original}
	if retryableRateResponse(response) {
		t.Fatal("retryableRateResponse() = true after the body read failed")
	}
	body, err := io.ReadAll(response.Body)
	if err == nil || err.Error() != wantErr.Error() || string(body) != `{"message":"ordinary forbidden response"}` {
		t.Fatalf("response body after peek error = %q, %v; want original bytes and read error", body, err)
	}
	if err := response.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if original.closeCalls.Load() != 1 {
		t.Fatalf("original body close calls = %d, want one", original.closeCalls.Load())
	}
}

func TestRateLimitTransportWaitsAtLeastOneMinuteWithoutRetryHeaders(t *testing.T) {
	var calls atomic.Int32
	var waits []time.Duration
	base := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		call := calls.Add(1)
		status := http.StatusOK
		body := "{}"
		if call == 1 {
			status = http.StatusForbidden
			body = `{"message":"You have exceeded a secondary rate limit"}`
		}
		return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
	})
	transport := &rateLimitTransport{
		base: base, now: func() time.Time { return time.Unix(100, 0) }, maxRetries: 3, maxWait: time.Minute,
		wait: func(_ context.Context, delay time.Duration) error { waits = append(waits, delay); return nil },
	}
	request, _ := http.NewRequest(http.MethodGet, "https://api.github.com/test", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if calls.Load() != 2 || !reflect.DeepEqual(waits, []time.Duration{time.Minute}) {
		t.Fatalf("GET calls/waits = %d/%v, want 2/[1m]", calls.Load(), waits)
	}
}

func TestRateLimitTransportIgnoresPrimaryResetForSecondaryLimit(t *testing.T) {
	var calls atomic.Int32
	var waits []time.Duration
	transport := &rateLimitTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			call := calls.Add(1)
			status := http.StatusOK
			body := "{}"
			header := make(http.Header)
			if call == 1 {
				status = http.StatusForbidden
				body = `{"message":"You have exceeded a secondary rate limit"}`
				header.Set("X-RateLimit-Remaining", "100")
				header.Set("X-RateLimit-Reset", "1900")
			}
			return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body)), Request: req}, nil
		}),
		wait: func(_ context.Context, delay time.Duration) error { waits = append(waits, delay); return nil },
		now:  func() time.Time { return time.Unix(100, 0) }, maxRetries: 3, maxWait: time.Minute,
	}
	request, _ := http.NewRequest(http.MethodGet, "https://api.github.com/test", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if calls.Load() != 2 || !reflect.DeepEqual(waits, []time.Duration{time.Minute}) {
		t.Fatalf("secondary-limit GET calls/waits = %d/%v, want 2/[1m]", calls.Load(), waits)
	}
}

func TestRateLimitTransportStopsWhenRetryHeaderExceedsBound(t *testing.T) {
	var calls atomic.Int32
	transport := &rateLimitTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			calls.Add(1)
			header := make(http.Header)
			header.Set("Retry-After", "3600")
			return &http.Response{StatusCode: http.StatusTooManyRequests, Header: header, Body: io.NopCloser(strings.NewReader(`{"message":"limited"}`)), Request: req}, nil
		}),
		wait: func(context.Context, time.Duration) error {
			t.Fatal("wait should not exceed configured bound")
			return nil
		},
		now: func() time.Time { return time.Unix(0, 0) }, maxRetries: 3, maxWait: time.Minute,
	}
	request, _ := http.NewRequest(http.MethodGet, "https://api.github.com/test", nil)
	response, err := transport.RoundTrip(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("rate-limited GET calls = %d, want one bounded attempt", calls.Load())
	}
}

func TestRateRetryDelayDoesNotOverflowForLargeRetryAfter(t *testing.T) {
	header := make(http.Header)
	header.Set("Retry-After", "9223372036854775807")
	delay, ok := rateRetryDelay(header, time.Unix(0, 0), 0)
	if !ok || delay <= maxRateWait {
		t.Fatalf("rateRetryDelay() = (%s, %t), want a delay larger than %s", delay, ok, maxRateWait)
	}
}

func TestWatcherClientAllowsRateLimitWaitWithinConfiguredTimeout(t *testing.T) {
	var calls atomic.Int32
	var waits []time.Duration
	client := testWatcherClient(t, "https://api.github.com", watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})
	httpClient := client.client.Client()
	httpClient.Transport = &rateLimitTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			status := http.StatusOK
			header := make(http.Header)
			if calls.Add(1) == 1 {
				status = http.StatusTooManyRequests
				header.Set("Retry-After", "40")
			}
			return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(`{}`)), Request: req}, nil
		}),
		wait: func(ctx context.Context, delay time.Duration) error {
			deadline, ok := ctx.Deadline()
			if !ok || time.Until(deadline) <= delay {
				return fmt.Errorf("retry delay %s exceeds the configured request deadline", delay)
			}
			waits = append(waits, delay)
			return nil
		},
		now: func() time.Time { return time.Unix(0, 0) }, maxRetries: maxRateRetries, maxWait: maxRateWait,
	}

	response, err := httpClient.Get("https://api.github.com/test")
	if err != nil {
		t.Fatalf("configured watcher client GET: %v", err)
	}
	_ = response.Body.Close()
	if calls.Load() != 2 || !reflect.DeepEqual(waits, []time.Duration{40 * time.Second}) {
		t.Fatalf("GET calls/waits = %d/%v, want 2/[40s]", calls.Load(), waits)
	}
}

func TestPatchIssueReadsBackAmbiguousMutationWithoutRetry(t *testing.T) {
	issue := issueJSON(7, "open", "old title", "old body", []string{watcher.ManagedLabel, watcher.FailingLabel, "keep-me"})
	var patchCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodPatch && path == "repos/orang-gaboets/octostate/issues/7":
			patchCalls.Add(1)
			var patch struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				t.Errorf("decode patch: %v", err)
			}
			issue["title"], issue["body"], issue["labels"] = patch.Title, patch.Body, labelObjects(patch.Labels)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"message": "connection outcome uncertain"})
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, issue)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, []any{})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})
	current := watcher.IssueSnapshot{Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "old title", Body: "old body", Labels: []string{watcher.ManagedLabel, watcher.FailingLabel, "keep-me"}}
	change := watcher.IssueChange{
		Action: watcher.ActionUpdate, IssueNumber: 7, Title: "new title", Body: "new body",
		AddLabels: []string{watcher.RecoveredLabel}, RemoveLabels: []string{watcher.FailingLabel},
	}
	updated, err := client.patchIssue(context.Background(), current, change)
	if err != nil {
		t.Fatalf("patchIssue() rejected desired readback after ambiguous response: %v", err)
	}
	if patchCalls.Load() != 1 {
		t.Fatalf("PATCH calls = %d, want no blind retry", patchCalls.Load())
	}
	wantLabels := []string{watcher.ManagedLabel, watcher.RecoveredLabel, "keep-me"}
	if updated.Title != "new title" || updated.Body != "new body" || !sameLabels(updated.Labels, wantLabels) {
		t.Fatalf("readback issue = %#v, want desired state and preserved label", updated)
	}
}

func TestPatchIssueRejectsSnapshotChangedImmediatelyBeforeMutation(t *testing.T) {
	fresh := issueJSON(7, "open", "human title", "human body", []string{watcher.ManagedLabel, watcher.FailingLabel, "keep-me", "human-label"})
	var patchCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, fresh)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, []any{})
		case r.Method == http.MethodPatch && path == "repos/orang-gaboets/octostate/issues/7":
			patchCalls.Add(1)
			writeJSON(w, http.StatusOK, fresh)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})
	current := watcher.IssueSnapshot{Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "old title", Body: "old body", Labels: []string{watcher.ManagedLabel, watcher.FailingLabel, "keep-me"}}
	change := watcher.IssueChange{
		Action: watcher.ActionUpdate, IssueNumber: 7, Title: "new title", Body: "new body",
		AddLabels: []string{watcher.RecoveredLabel}, RemoveLabels: []string{watcher.FailingLabel},
	}
	if _, err := client.patchIssue(context.Background(), current, change); err == nil || !strings.Contains(err.Error(), "changed before issue update") {
		t.Fatalf("patchIssue() error = %v, want last-minute snapshot change refusal", err)
	}
	if patchCalls.Load() != 0 {
		t.Fatalf("issue PATCH calls = %d, want none after human edit", patchCalls.Load())
	}
}

func TestApplyPlanStopsBeforeWriteOnConcurrentHumanEdit(t *testing.T) {
	issue := issueJSON(7, "open", "edited by human", "human text", []string{watcher.ManagedLabel, watcher.FailingLabel})
	var writes atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7":
			writeJSON(w, http.StatusOK, issue)
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/7/comments":
			writeJSON(w, http.StatusOK, []any{})
		case r.Method == http.MethodPatch:
			writes.Add(1)
			writeJSON(w, http.StatusOK, issue)
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})
	original := watcher.IssueSnapshot{Number: 7, State: "open", AuthorLogin: expectedAppSlug + "[bot]", Title: "old title", Body: "old body", Labels: []string{watcher.ManagedLabel, watcher.FailingLabel}}
	plan := watcher.Plan{Changes: []watcher.IssueChange{{Action: watcher.ActionUpdate, IssueNumber: 7, Title: "desired", Body: "desired"}}}
	err := client.applyPlan(context.Background(), plan, []watcher.IssueSnapshot{original})
	if err == nil || !strings.Contains(err.Error(), "changed during reconciliation") {
		t.Fatalf("applyPlan() error = %v, want concurrent edit refusal", err)
	}
	if writes.Load() != 0 {
		t.Fatalf("API mutations = %d, want none", writes.Load())
	}
}

func TestApplyPlanCreatesMultipleNewIncidentsInOneReconciliation(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	started := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	secondFailure := testFailureAttempt(102, 102, started.Add(time.Minute))
	secondFailure.WorkflowID = 2
	secondFailure.WorkflowName = "CodeQL Security"
	plan, err := watcher.Reconcile(repo, expectedAppSlug+"[bot]", []watcher.RunAttempt{
		testFailureAttempt(101, 101, started), secondFailure,
	}, nil, started.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 2 {
		t.Fatalf("planned changes = %d, want two new incidents", len(plan.Changes))
	}

	issues := make(map[int]map[string]any)
	comments := make(map[int][]any)
	nextNumber := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues":
			listed := make([]any, 0, len(issues))
			for number := 1; number <= nextNumber; number++ {
				if issue, ok := issues[number]; ok {
					listed = append(listed, issue)
				}
			}
			writeJSON(w, http.StatusOK, listed)
		case r.Method == http.MethodPost && path == "repos/orang-gaboets/octostate/issues":
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode create issue: %v", err)
			}
			nextNumber++
			issues[nextNumber] = issueJSON(nextNumber, "open", request.Title, request.Body, request.Labels)
			comments[nextNumber] = []any{}
			writeJSON(w, http.StatusCreated, issues[nextNumber])
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/") && strings.HasSuffix(path, "/comments"):
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			writeJSON(w, http.StatusOK, comments[number])
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, repo)
	if err := client.applyPlan(context.Background(), plan, nil); err != nil {
		t.Fatalf("applyPlan() with two creates returned error: %v", err)
	}
	if nextNumber != 2 {
		t.Fatalf("created issues = %d, want two", nextNumber)
	}
}

func TestListManagedIssuesPaginatesIssuesAndIssueComments(t *testing.T) {
	var issuePages, commentPages atomic.Int32
	marker := `<!-- actions-watcher:v1 repo_id=99 workflow_id=1 workflow_name="CI" -->`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues":
			issuePages.Add(1)
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `</api/v3/repos/orang-gaboets/octostate/issues?per_page=100&page=2>; rel="next"`)
				writeJSON(w, http.StatusOK, []any{issueJSON(10, "open", "incident one", marker, []string{watcher.ManagedLabel})})
			} else {
				writeJSON(w, http.StatusOK, []any{issueJSON(11, "closed", "incident two", marker, []string{watcher.ManagedLabel})})
			}
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/10/comments":
			commentPages.Add(1)
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `</api/v3/repos/orang-gaboets/octostate/issues/10/comments?per_page=100&page=2>; rel="next"`)
				writeJSON(w, http.StatusOK, []any{map[string]any{"id": 1, "body": "comment 1", "user": map[string]any{"login": expectedAppSlug + "[bot]"}}})
			} else {
				writeJSON(w, http.StatusOK, []any{map[string]any{"id": 2, "body": "comment 2", "user": map[string]any{"login": "maintainer"}}})
			}
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues/11/comments":
			writeJSON(w, http.StatusOK, []any{})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})
	issues, err := client.listManagedIssues(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if issuePages.Load() != 2 || commentPages.Load() != 2 {
		t.Fatalf("issue/comment page calls = %d/%d, want 2/2", issuePages.Load(), commentPages.Load())
	}
	if len(issues) != 2 || len(issues[0].Comments) != 2 || issues[0].Comments[1].Body != "comment 2" {
		t.Fatalf("paginated issue snapshot = %#v", issues)
	}
}

func TestListManagedIssuesIgnoresPublicIssuesThatSpoofWatcherIdentity(t *testing.T) {
	spoofedTitle := issueJSON(12, "open", "ci(actions): investigate CI workflow failures on main", "not managed", nil)
	spoofedTitle["user"] = map[string]any{"login": "public-user"}
	spoofedBody := issueJSON(13, "closed", "unrelated issue", "<!-- actions-watcher:v1", nil)
	spoofedBody["user"] = map[string]any{"login": "public-user"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || strings.TrimPrefix(r.URL.Path, "/api/v3/") != "repos/orang-gaboets/octostate/issues" {
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
			return
		}
		writeJSON(w, http.StatusOK, []any{spoofedTitle, spoofedBody})
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})

	issues, err := client.listManagedIssues(context.Background())
	if err != nil {
		t.Fatalf("listManagedIssues() rejected untrusted spoof issues: %v", err)
	}
	if len(issues) != 0 {
		t.Fatalf("listManagedIssues() returned %d untrusted issues, want none", len(issues))
	}
}

func TestListManagedIssuesRejectsAppAuthoredIssueWithoutIdentityMarker(t *testing.T) {
	issue := issueJSON(14, "open", "unrelated title", "marker removed", nil)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || strings.TrimPrefix(r.URL.Path, "/api/v3/") != "repos/orang-gaboets/octostate/issues" {
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
			return
		}
		writeJSON(w, http.StatusOK, []any{issue})
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})

	issues, err := client.listManagedIssues(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no identity marker") {
		t.Fatalf("listManagedIssues() = %#v, %v; want missing identity marker error", issues, err)
	}
}

func TestFindMarkedCommentIgnoresUntrustedMarkerText(t *testing.T) {
	marker := "<!-- actions-watcher:transition:v1 expected -->"
	comments := []watcher.CommentSnapshot{{AuthorLogin: "public-user", Body: "quoted marker: " + marker}}

	found, err := findMarkedComment(comments, marker, "expected body", expectedAppSlug+"[bot]")
	if err != nil {
		t.Fatalf("findMarkedComment() rejected an untrusted marker mention: %v", err)
	}
	if found {
		t.Fatal("findMarkedComment() treated untrusted marker text as an App comment")
	}
}

//nolint:gocyclo // keeps the create, comment, and final readback sequence in one HTTP fixture.
func TestCreateIncidentWritesTransitionCommentsBeforeFinalBody(t *testing.T) {
	repo := watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"}
	closedAt := time.Date(2026, 11, 25, 8, 0, 0, 0, time.UTC)
	oldFailure := testFailureAttempt(100, 1, closedAt.Add(-time.Hour))
	oldState := watcher.IncidentState{WorkflowID: 1, WorkflowName: "CI", Status: watcher.StatusFailing, Failures: []watcher.RunAttempt{oldFailure}, Latest: oldFailure}
	oldBody, _, err := watcher.RenderBody(repo, oldState, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	closedIssue := watcher.IssueSnapshot{
		Number: 50, State: "closed", ClosedAt: &closedAt, AuthorLogin: expectedAppSlug + "[bot]",
		Title: "old incident", Body: oldBody, Labels: []string{watcher.ManagedLabel, watcher.FailingLabel},
	}
	first := testFailureAttempt(101, 1, closedAt.Add(time.Hour))
	recovery := first
	recovery.RunID, recovery.RunNumber, recovery.Conclusion = 102, 102, "success"
	recovery.StartedAt, recovery.CompletedAt, recovery.HTMLURL = closedAt.Add(2*time.Hour), closedAt.Add(2*time.Hour+time.Minute), "https://github.com/orang-gaboets/octostate/actions/runs/102"
	regression := testFailureAttempt(103, 1, closedAt.Add(3*time.Hour))
	attempts := []watcher.RunAttempt{first, recovery, regression}
	plan, err := watcher.Reconcile(repo, expectedAppSlug+"[bot]", attempts, []watcher.IssueSnapshot{closedIssue}, closedAt.Add(4*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Action != watcher.ActionCreate || plan.Changes[0].InitialBody == "" || len(plan.Changes[0].Comments) != 2 {
		t.Fatalf("create change lacks staged initial body and transitions: %#v", plan.Changes)
	}
	change := plan.Changes[0]
	change.PendingRunIDs = []int64{103}
	plan.Changes[0] = change
	apiIssue := map[int]map[string]any{50: issueSnapshotJSON(closedIssue)}
	comments := map[int][]any{50: []any{}}
	var nextNumber = 50
	var commentID int64
	var writes []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case r.Method == http.MethodGet && path == "repos/orang-gaboets/octostate/issues":
			issues := make([]any, 0, len(apiIssue))
			for number := 1; number <= nextNumber; number++ {
				if issue, ok := apiIssue[number]; ok {
					issues = append(issues, issue)
				}
			}
			writeJSON(w, http.StatusOK, issues)
		case r.Method == http.MethodPost && path == "repos/orang-gaboets/octostate/issues":
			writes = append(writes, "create")
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode create issue: %v", err)
			}
			initialState, stateErr := watcher.RenderBodyState(request.Body, repo, change.WorkflowID, change.WorkflowName)
			if request.Body == change.Body || stateErr != nil || !reflect.DeepEqual(initialState.PendingRunIDs, []int64{103}) {
				t.Errorf("create body did not persist staged state and pending run ID: state=%#v err=%v", initialState, stateErr)
			}
			nextNumber++
			apiIssue[nextNumber] = issueJSON(nextNumber, "open", request.Title, request.Body, request.Labels)
			comments[nextNumber] = []any{}
			writeJSON(w, http.StatusCreated, apiIssue[nextNumber])
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/") && strings.HasSuffix(path, "/comments"):
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			writeJSON(w, http.StatusOK, comments[number])
		case r.Method == http.MethodGet && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/"):
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			writeJSON(w, http.StatusOK, apiIssue[number])
		case r.Method == http.MethodPost && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/") && strings.HasSuffix(path, "/comments"):
			writes = append(writes, "comment")
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			var request struct {
				Body string `json:"body"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode create comment: %v", err)
			}
			commentID++
			comments[number] = append(comments[number], map[string]any{"id": commentID, "body": request.Body, "user": map[string]any{"login": expectedAppSlug + "[bot]"}})
			writeJSON(w, http.StatusCreated, map[string]any{"id": commentID, "body": request.Body, "user": map[string]any{"login": expectedAppSlug + "[bot]"}})
		case r.Method == http.MethodPatch && strings.HasPrefix(path, "repos/orang-gaboets/octostate/issues/"):
			writes = append(writes, "patch")
			parts := strings.Split(path, "/")
			number, _ := strconv.Atoi(parts[4])
			var request struct {
				Title  string   `json:"title"`
				Body   string   `json:"body"`
				Labels []string `json:"labels"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode patch issue: %v", err)
			}
			apiIssue[number]["title"], apiIssue[number]["body"], apiIssue[number]["labels"] = request.Title, request.Body, labelObjects(request.Labels)
			writeJSON(w, http.StatusOK, apiIssue[number])
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, repo)
	if err := client.applyPlan(context.Background(), plan, []watcher.IssueSnapshot{closedIssue}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(writes, []string{"create", "comment", "comment", "patch"}) {
		t.Fatalf("mutation order = %v, want create, transitions, final patch", writes)
	}
	created := apiIssue[51]
	if created["body"] != change.Body {
		t.Fatal("created incident did not reach final body")
	}
	finalState, err := watcher.RenderBodyState(created["body"].(string), repo, change.WorkflowID, change.WorkflowName)
	if err != nil || len(finalState.PendingRunIDs) != 0 {
		t.Fatalf("final created body retained pending runs: state=%#v err=%v", finalState, err)
	}
	if !sameLabels(issueLabels(created), []string{watcher.ManagedLabel, watcher.FailingLabel}) {
		t.Fatalf("final created labels = %#v", issueLabels(created))
	}
}

func testWatcherClient(t *testing.T, apiURL string, repo watcher.RepositoryIdentity) *watcherClient {
	t.Helper()
	client, err := newWatcherClient(context.Background(), "test-token", apiURL, repo, expectedAppSlug)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func workflowFixtures() []any {
	names := []string{
		"CI", "CodeQL Security", "Go vulnerability monitoring", "Go toolchain remediation",
		"Release Please", "Contributor showcase", "Trusted Live Integration",
	}
	result := make([]any, 0, len(names))
	for i, name := range names {
		result = append(result, map[string]any{"id": i + 1, "name": name, "path": expectedWorkflowPaths[name]})
	}
	return result
}

func runFixture(repoID int64, runID int64, latestAttempt int, conclusion string) map[string]any {
	return map[string]any{
		"id": runID, "name": "CI", "workflow_id": 1, "path": expectedWorkflowPaths["CI"] + "@main",
		"run_number": int(runID), "run_attempt": latestAttempt, "head_branch": "main", "event": "push",
		"conclusion": conclusion, "status": "completed", "head_sha": "0123456789abcdef", "html_url": fmt.Sprintf("https://github.com/orang-gaboets/octostate/actions/runs/%d", runID),
		"run_started_at": "2026-11-25T10:00:00Z", "updated_at": "2026-11-25T10:10:00Z",
		"repository": map[string]any{"id": repoID}, "head_repository": map[string]any{"id": repoID},
	}
}

func runAttemptFixture(repoID int64, runID int64, attemptNumber int, conclusion string) map[string]any {
	result := runFixture(repoID, runID, attemptNumber, conclusion)
	result["run_started_at"] = fmt.Sprintf("2026-11-25T10:%02d:00Z", attemptNumber)
	result["updated_at"] = fmt.Sprintf("2026-11-25T10:%02d:30Z", attemptNumber)
	return result
}

func issueJSON(number int, state, title, body string, labels []string) map[string]any {
	return map[string]any{
		"number": number, "state": state, "title": title, "body": body,
		"user": map[string]any{"login": expectedAppSlug + "[bot]"}, "labels": labelObjects(labels),
	}
}

func issueSnapshotJSON(issue watcher.IssueSnapshot) map[string]any {
	value := issueJSON(issue.Number, issue.State, issue.Title, issue.Body, issue.Labels)
	if issue.ClosedAt != nil {
		value["closed_at"] = issue.ClosedAt.Format(time.RFC3339)
	}
	return value
}

func issueLabels(issue map[string]any) []string {
	objects, _ := issue["labels"].([]any)
	labels := make([]string, 0, len(objects))
	for _, object := range objects {
		if item, ok := object.(map[string]any); ok {
			if label, ok := item["name"].(string); ok {
				labels = append(labels, label)
			}
		}
	}
	return labels
}

func testFailureAttempt(runID int64, runNumber int, started time.Time) watcher.RunAttempt {
	return watcher.RunAttempt{
		WorkflowID: 1, WorkflowName: "CI", RunID: runID, RunNumber: runNumber, Attempt: 1,
		Event: "push", HeadBranch: "main", Conclusion: "failure", HeadSHA: "0123456789abcdef",
		HTMLURL:   fmt.Sprintf("https://github.com/orang-gaboets/octostate/actions/runs/%d", runID),
		StartedAt: started, CompletedAt: started.Add(time.Minute), FailedJobs: []string{"test"}, JobDataAvailable: true,
	}
}

func labelObjects(labels []string) []any {
	objects := make([]any, 0, len(labels))
	for _, name := range labels {
		objects = append(objects, map[string]any{"name": name})
	}
	return objects
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(err)
	}
}

func writeRaw(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

type trackedReadCloser struct {
	io.Reader
	closeCalls  atomic.Int32
	readErr     error
	readErrSent bool
}

func (b *trackedReadCloser) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if b.readErr != nil && !b.readErrSent {
		b.readErrSent = true
		return n, b.readErr
	}
	return n, err
}

func (b *trackedReadCloser) Close() error {
	b.closeCalls.Add(1)
	return nil
}
