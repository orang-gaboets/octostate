// Command actions-watcher reconciles GitHub Actions failures into incident
// issues for the repository's explicitly monitored workflows.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	watcher "github.com/orang-gaboets/octostate/internal/actionswatcher"
)

const (
	expectedRepository      = "orang-gaboets/octostate"
	expectedAppSlug         = "orang-gaboets-actions-watcher"
	lookbackWindow          = 7 * 24 * time.Hour
	pendingCheckpointPrefix = "<!-- actions-watcher:pending:v1 "
)

type commandConfig struct {
	Repository          watcher.RepositoryIdentity
	TriggerRunID        int64
	TriggerWorkflowName string
	APIURL              string
	Token               string
	AppSlug             string
	ActivatedAt         time.Time
	ReconcileAt         time.Time
}

type commandResult struct {
	Repository      string                 `json:"repository"`
	Attempts        int                    `json:"attempts"`
	Changes         []resultChange         `json:"changes"`
	UnavailableRuns []resultUnavailableRun `json:"unavailable_runs,omitempty"`
}

type resultChange struct {
	Action   watcher.IssueAction    `json:"action"`
	Workflow string                 `json:"workflow"`
	Issue    int                    `json:"issue,omitempty"`
	Previous int                    `json:"previous_issue,omitempty"`
	Status   watcher.IncidentStatus `json:"status"`
	Failures int                    `json:"failures"`
}

type resultUnavailableRun struct {
	Workflow   string `json:"workflow"`
	Issue      int    `json:"issue,omitempty"`
	RunID      int64  `json:"run_id"`
	StatusCode int    `json:"status_code"`
}

func main() {
	if err := run(context.Background(), os.Getenv, os.Stdout, time.Now); err != nil {
		fmt.Fprintf(os.Stderr, "actions-watcher: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, getenv func(string) string, stdout io.Writer, now func() time.Time) error {
	cfg, err := readConfig(getenv, now())
	if err != nil {
		return err
	}
	client, err := newWatcherClient(ctx, cfg.Token, cfg.APIURL, cfg.Repository, cfg.AppSlug)
	if err != nil {
		return err
	}
	issues, err := client.listManagedIssues(ctx)
	if err != nil {
		return fmt.Errorf("read watcher incidents: %w", err)
	}
	if err := client.verifyOpenIncidentCursors(ctx, issues); err != nil {
		return fmt.Errorf("verify watcher incident state: %w", err)
	}
	triggerEvents, err := pendingRunEvents(cfg.Repository, client.appLogin, issues)
	if err != nil {
		return fmt.Errorf("read pending workflow runs: %w", err)
	}
	if cfg.TriggerRunID > 0 {
		triggerEvents = append(triggerEvents, triggerEvent{RunID: cfg.TriggerRunID, WorkflowName: cfg.TriggerWorkflowName})
	}
	attempts, unavailableRuns, err := client.collectAttempts(ctx, cfg.ActivatedAt, cfg.ReconcileAt, triggerEvents)
	if err != nil {
		return fmt.Errorf("collect workflow attempts: %w", err)
	}
	plan, err := watcher.Reconcile(cfg.Repository, client.appLogin, attempts, issues, cfg.ReconcileAt)
	if err != nil {
		return fmt.Errorf("plan incident reconciliation: %w", err)
	}
	if err := recordRunEvents(&plan, triggerEvents, unavailableRuns); err != nil {
		return fmt.Errorf("record workflow run events: %w", err)
	}
	attemptCount := len(attempts)
	refreshed, err := client.refreshUnavailableArchiveAttempts(ctx, plan, issues)
	if err != nil {
		return fmt.Errorf("refresh archived workflow attempts: %w", err)
	}
	if len(refreshed) > 0 {
		attempts = mergeArchiveRefreshes(attempts, refreshed)
		plan, err = watcher.Reconcile(cfg.Repository, client.appLogin, attempts, issues, cfg.ReconcileAt)
		if err != nil {
			return fmt.Errorf("replan incident reconciliation: %w", err)
		}
		if err := recordRunEvents(&plan, triggerEvents, unavailableRuns); err != nil {
			return fmt.Errorf("record workflow run events: %w", err)
		}
	}
	if len(plan.Changes) > 0 {
		if err := client.requireLabels(ctx, plan); err != nil {
			return err
		}
		if err := client.applyPlan(ctx, plan, issues); err != nil {
			return fmt.Errorf("apply incident reconciliation: %w", err)
		}
	}
	if err := client.completeClosedPendingRuns(ctx, issues, triggerEvents, unavailableRuns); err != nil {
		return fmt.Errorf("complete closed-incident checkpoints: %w", err)
	}
	result := resultFor(attemptCount, plan, unavailableRuns...)
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return fmt.Errorf("write reconciliation result: %w", err)
	}
	return nil
}

func mergeArchiveRefreshes(attempts, refreshed []watcher.RunAttempt) []watcher.RunAttempt {
	updates := make(map[string]watcher.RunAttempt, len(refreshed))
	for _, attempt := range refreshed {
		updates[fmt.Sprintf("%d/%d/%d", attempt.WorkflowID, attempt.RunID, attempt.Attempt)] = attempt
	}
	result := append([]watcher.RunAttempt(nil), attempts...)
	for i, attempt := range result {
		key := fmt.Sprintf("%d/%d/%d", attempt.WorkflowID, attempt.RunID, attempt.Attempt)
		if update, ok := updates[key]; ok {
			result[i] = update
			delete(updates, key)
		}
	}
	for _, attempt := range refreshed {
		key := fmt.Sprintf("%d/%d/%d", attempt.WorkflowID, attempt.RunID, attempt.Attempt)
		if _, ok := updates[key]; ok {
			result = append(result, attempt)
			delete(updates, key)
		}
	}
	return result
}

func pendingRunEvents(repo watcher.RepositoryIdentity, appLogin string, issues []watcher.IssueSnapshot) ([]triggerEvent, error) {
	type workflowKey struct {
		id   int64
		name string
	}
	type issueState struct {
		issue watcher.IssueSnapshot
		state watcher.IncidentState
	}
	byWorkflow := make(map[workflowKey][]issueState)
	for _, issue := range issues {
		if issue.State != "open" && issue.State != "closed" {
			return nil, fmt.Errorf("issue #%d has unknown state %q", issue.Number, issue.State)
		}
		state, err := watcher.RenderManagedBodyState(issue.Body, repo)
		if err != nil {
			return nil, fmt.Errorf("issue #%d: %w", issue.Number, err)
		}
		key := workflowKey{id: state.WorkflowID, name: state.WorkflowName}
		byWorkflow[key] = append(byWorkflow[key], issueState{issue: issue, state: state})
	}

	eventsByID := make(map[int64]triggerEvent)
	maxPendingRunEvents := 2 * len(expectedWorkflowPaths) * watcher.MaxPendingRunIDs
	addRunID := func(runID int64, workflowName string, issueNumber int) error {
		if previous, exists := eventsByID[runID]; exists {
			if previous.WorkflowName != workflowName {
				return fmt.Errorf("pending workflow run %d identifies conflicting workflows %q and %q", runID, previous.WorkflowName, workflowName)
			}
			if previous.IssueNumber != issueNumber {
				return fmt.Errorf("pending workflow run %d is stored on multiple incidents", runID)
			}
			return nil
		}
		eventsByID[runID] = triggerEvent{RunID: runID, WorkflowName: workflowName, IssueNumber: issueNumber}
		if len(eventsByID) > maxPendingRunEvents {
			return fmt.Errorf("managed issues contain more than %d pending workflow runs", maxPendingRunEvents)
		}
		return nil
	}
	addIssuePendingRuns := func(source issueState, skip map[int64]bool) (map[int64]bool, error) {
		seen := make(map[int64]bool)
		add := func(runID int64) error {
			seen[runID] = true
			if skip[runID] {
				return nil
			}
			return addRunID(runID, source.state.WorkflowName, source.issue.Number)
		}
		for _, runID := range source.state.PendingRunIDs {
			if err := add(runID); err != nil {
				return nil, err
			}
		}
		for _, comment := range source.issue.Comments {
			if comment.AuthorLogin != appLogin {
				continue
			}
			workflowID, runIDs, found, err := parsePendingCheckpoint(comment.Body)
			if err != nil {
				return nil, fmt.Errorf("issue #%d has an invalid pending-run checkpoint: %w", source.issue.Number, err)
			}
			if !found {
				continue
			}
			if workflowID != source.state.WorkflowID {
				return nil, fmt.Errorf("issue #%d pending-run checkpoint identifies workflow ID %d, want %d", source.issue.Number, workflowID, source.state.WorkflowID)
			}
			for _, runID := range runIDs {
				if err := add(runID); err != nil {
					return nil, err
				}
			}
		}
		return seen, nil
	}
	keys := make([]workflowKey, 0, len(byWorkflow))
	for key := range byWorkflow {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].id < keys[j].id
	})
	for _, key := range keys {
		incidents := byWorkflow[key]
		var open, latestClosed *issueState
		for i := range incidents {
			candidate := &incidents[i]
			if candidate.issue.State == "open" {
				if open != nil {
					return nil, fmt.Errorf("workflow %q has multiple open watcher incidents", candidate.state.WorkflowName)
				}
				open = candidate
				continue
			}
			if candidate.issue.ClosedAt == nil {
				return nil, fmt.Errorf("closed issue #%d is missing its closure timestamp", candidate.issue.Number)
			}
			if latestClosed == nil || candidate.issue.ClosedAt.After(*latestClosed.issue.ClosedAt) ||
				(candidate.issue.ClosedAt.Equal(*latestClosed.issue.ClosedAt) && candidate.issue.Number > latestClosed.issue.Number) {
				latestClosed = candidate
			}
		}
		if open == nil && latestClosed == nil {
			continue
		}
		var closedPending map[int64]bool
		if latestClosed != nil {
			var err error
			closedPending, err = addIssuePendingRuns(*latestClosed, nil)
			if err != nil {
				return nil, err
			}
		}
		if open != nil {
			if _, err := addIssuePendingRuns(*open, closedPending); err != nil {
				return nil, err
			}
		}
	}
	runIDs := make([]int64, 0, len(eventsByID))
	for runID := range eventsByID {
		runIDs = append(runIDs, runID)
	}
	sort.Slice(runIDs, func(i, j int) bool { return runIDs[i] < runIDs[j] })
	events := make([]triggerEvent, 0, len(runIDs))
	for _, runID := range runIDs {
		events = append(events, eventsByID[runID])
	}
	return events, nil
}

func pendingCheckpointMarker(workflowID int64, runIDs []int64) (string, error) {
	if workflowID <= 0 || len(runIDs) == 0 {
		return "", errors.New("pending-run checkpoint identity is incomplete")
	}
	ids := append([]int64(nil), runIDs...)
	for _, runID := range ids {
		if runID <= 0 {
			return "", errors.New("pending-run checkpoint IDs must be positive")
		}
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	if len(ids) > watcher.MaxPendingRunIDs {
		return "", fmt.Errorf("pending-run checkpoint exceeds %d IDs", watcher.MaxPendingRunIDs)
	}
	values := make([]string, len(ids))
	for i, runID := range ids {
		values[i] = strconv.FormatInt(runID, 10)
	}
	return fmt.Sprintf("%sworkflow_id=%d run_ids=%s -->", pendingCheckpointPrefix, workflowID, strings.Join(values, ",")), nil
}

func parsePendingCheckpoint(body string) (int64, []int64, bool, error) {
	if !strings.Contains(body, pendingCheckpointPrefix) {
		return 0, nil, false, nil
	}
	if strings.Count(body, pendingCheckpointPrefix) != 1 || !strings.HasPrefix(body, pendingCheckpointPrefix) || !strings.HasSuffix(body, " -->") {
		return 0, nil, false, errors.New("marker is malformed")
	}
	fields := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(body, pendingCheckpointPrefix), " -->"))
	if len(fields) != 2 || !strings.HasPrefix(fields[0], "workflow_id=") || !strings.HasPrefix(fields[1], "run_ids=") {
		return 0, nil, false, errors.New("marker fields are malformed")
	}
	workflowID, err := strconv.ParseInt(strings.TrimPrefix(fields[0], "workflow_id="), 10, 64)
	if err != nil || workflowID <= 0 {
		return 0, nil, false, errors.New("marker workflow ID is invalid")
	}
	rawIDs := strings.TrimPrefix(fields[1], "run_ids=")
	if rawIDs == "" {
		return 0, nil, false, errors.New("marker run IDs are empty")
	}
	parts := strings.Split(rawIDs, ",")
	if len(parts) > watcher.MaxPendingRunIDs {
		return 0, nil, false, fmt.Errorf("marker exceeds %d run IDs", watcher.MaxPendingRunIDs)
	}
	runIDs := make([]int64, len(parts))
	for i, part := range parts {
		runID, parseErr := strconv.ParseInt(part, 10, 64)
		if parseErr != nil || runID <= 0 || (i > 0 && runID <= runIDs[i-1]) {
			return 0, nil, false, errors.New("marker run IDs are invalid or unordered")
		}
		runIDs[i] = runID
	}
	return workflowID, runIDs, true, nil
}

func readConfig(getenv func(string) string, now time.Time) (commandConfig, error) {
	var cfg commandConfig
	repository := strings.TrimSpace(getenv("GITHUB_REPOSITORY"))
	if repository != expectedRepository {
		return cfg, fmt.Errorf("GITHUB_REPOSITORY must be %q", expectedRepository)
	}
	repoID, err := strconv.ParseInt(strings.TrimSpace(getenv("GITHUB_REPOSITORY_ID")), 10, 64)
	if err != nil || repoID <= 0 {
		return cfg, errors.New("GITHUB_REPOSITORY_ID must be a positive integer")
	}
	eventName := strings.TrimSpace(getenv("GITHUB_EVENT_NAME"))
	if eventName != "workflow_run" && eventName != "push" && eventName != "schedule" {
		return cfg, fmt.Errorf("unsupported GITHUB_EVENT_NAME %q", eventName)
	}
	eventRef := strings.TrimSpace(getenv("GITHUB_REF"))
	if eventRef != "refs/heads/main" {
		return cfg, errors.New("actions-watcher must run from refs/heads/main")
	}
	eventPath := strings.TrimSpace(getenv("GITHUB_EVENT_PATH"))
	if eventPath == "" {
		return cfg, errors.New("GITHUB_EVENT_PATH is required")
	}
	trigger, err := readTriggerEvent(eventName, eventPath)
	if err != nil {
		return cfg, err
	}
	apiURL := strings.TrimSpace(getenv("GITHUB_API_URL"))
	if apiURL == "" {
		apiURL = "https://api.github.com"
	}
	token := strings.TrimSpace(getenv("ACTIONS_WATCHER_TOKEN"))
	if token == "" {
		return cfg, errors.New("ACTIONS_WATCHER_TOKEN is required; no credential fallback is permitted")
	}
	appSlug := strings.TrimSpace(getenv("ACTIONS_WATCHER_APP_SLUG"))
	if appSlug != expectedAppSlug {
		return cfg, fmt.Errorf("ACTIONS_WATCHER_APP_SLUG must be %q", expectedAppSlug)
	}
	activated, err := time.Parse(time.RFC3339, strings.TrimSpace(getenv("ACTIONS_WATCHER_ACTIVATED_AT")))
	if err != nil {
		return cfg, fmt.Errorf("ACTIONS_WATCHER_ACTIVATED_AT must be RFC3339: %w", err)
	}
	if now.IsZero() {
		return cfg, errors.New("reconciliation time is unavailable")
	}
	return commandConfig{
		Repository:   watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"},
		TriggerRunID: trigger.RunID, TriggerWorkflowName: trigger.WorkflowName,
		APIURL: apiURL, Token: token, AppSlug: appSlug, ActivatedAt: activated.UTC(), ReconcileAt: now.UTC(),
	}, nil
}

type triggerEvent struct {
	RunID        int64
	WorkflowName string
	IssueNumber  int
}

type unavailableRun struct {
	Event      triggerEvent
	StatusCode int
}

func readTriggerEvent(eventName, eventPath string) (triggerEvent, error) {
	var trigger triggerEvent
	if eventName != "workflow_run" {
		return trigger, nil
	}
	file, err := os.Open(eventPath)
	if err != nil {
		return trigger, fmt.Errorf("read GITHUB_EVENT_PATH: %w", err)
	}
	var payload struct {
		Action      string `json:"action"`
		WorkflowRun struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"workflow_run"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(file, 1<<20)).Decode(&payload)
	closeErr := file.Close()
	if decodeErr != nil {
		return trigger, fmt.Errorf("decode GITHUB_EVENT_PATH: %w", decodeErr)
	}
	if closeErr != nil {
		return trigger, fmt.Errorf("close GITHUB_EVENT_PATH: %w", closeErr)
	}
	if payload.Action != "completed" {
		return trigger, fmt.Errorf("workflow_run action must be completed, got %q", payload.Action)
	}
	if !watcher.IsMonitoredWorkflow(payload.WorkflowRun.Name) {
		return trigger, fmt.Errorf("workflow_run event is for an unmonitored workflow %q", payload.WorkflowRun.Name)
	}
	if payload.WorkflowRun.ID <= 0 {
		return trigger, errors.New("workflow_run event is missing a positive run ID")
	}
	trigger.RunID = payload.WorkflowRun.ID
	trigger.WorkflowName = payload.WorkflowRun.Name
	return trigger, nil
}

func resultFor(attemptCount int, plan watcher.Plan, unavailable ...unavailableRun) commandResult {
	result := commandResult{
		Repository: expectedRepository,
		Attempts:   attemptCount,
		Changes:    make([]resultChange, 0, len(plan.Changes)),
	}
	for _, change := range plan.Changes {
		result.Changes = append(result.Changes, resultChange{
			Action: change.Action, Workflow: change.WorkflowName, Issue: change.IssueNumber,
			Previous: change.PreviousIssueNumber, Status: change.CurrentStatus, Failures: change.FailureCount,
		})
	}
	for _, missing := range unavailable {
		result.UnavailableRuns = append(result.UnavailableRuns, resultUnavailableRun{
			Workflow: missing.Event.WorkflowName, Issue: missing.Event.IssueNumber,
			RunID: missing.Event.RunID, StatusCode: missing.StatusCode,
		})
	}
	return result
}

func recordRunEvents(plan *watcher.Plan, events []triggerEvent, unavailable []unavailableRun) error {
	unavailableIDs := make(map[int64]bool, len(unavailable))
	for _, missing := range unavailable {
		unavailableIDs[missing.Event.RunID] = true
	}
	for i := range plan.Changes {
		change := &plan.Changes[i]
		for _, event := range events {
			if event.WorkflowName == change.WorkflowName && !unavailableIDs[event.RunID] {
				change.PendingRunIDs = append(change.PendingRunIDs, event.RunID)
			}
		}
		for _, missing := range unavailable {
			if missing.Event.WorkflowName != change.WorkflowName || (missing.Event.IssueNumber != 0 && missing.Event.IssueNumber != change.IssueNumber) {
				continue
			}
			comment := unavailableRunComment(change.WorkflowID, missing)
			marker := comment.Marker
			found := false
			for _, existing := range change.Comments {
				if existing.Marker != marker {
					continue
				}
				if existing.Body != comment.Body {
					return fmt.Errorf("unavailable run marker for run %d has conflicting content", missing.Event.RunID)
				}
				found = true
				break
			}
			if !found {
				change.Comments = append(change.Comments, comment)
			}
		}
	}
	return nil
}

func unavailableRunComment(workflowID int64, missing unavailableRun) watcher.ManagedComment {
	marker := fmt.Sprintf("<!-- actions-watcher:unavailable:v1 workflow_id=%d run_id=%d status=%d -->", workflowID, missing.Event.RunID, missing.StatusCode)
	return watcher.ManagedComment{
		Marker: marker,
		Body:   fmt.Sprintf("%s\nWorkflow run ID %d is unavailable because GitHub returned HTTP %d. Its attempt evidence could not be retrieved; this does not indicate a successful run.", marker, missing.Event.RunID, missing.StatusCode),
	}
}
