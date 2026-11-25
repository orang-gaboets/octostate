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
	"sort"
	"strconv"
	"strings"
	"time"

	watcher "github.com/orang-gaboets/octostate/internal/actionswatcher"
)

const (
	expectedRepository  = "orang-gaboets/octostate"
	expectedAppSlug     = "orang-gaboets-actions-watcher"
	lookbackWindow      = 7 * 24 * time.Hour
	maxPendingRunEvents = 50
)

type commandConfig struct {
	Repository          watcher.RepositoryIdentity
	EventName           string
	EventPath           string
	EventRef            string
	TriggerRunID        int64
	TriggerWorkflowName string
	APIURL              string
	Token               string
	AppSlug             string
	ActivatedAt         time.Time
	ReconcileAt         time.Time
}

type commandResult struct {
	Repository string         `json:"repository"`
	Attempts   int            `json:"attempts"`
	Changes    []resultChange `json:"changes"`
}

type resultChange struct {
	Action   watcher.IssueAction    `json:"action"`
	Workflow string                 `json:"workflow"`
	Issue    int                    `json:"issue,omitempty"`
	Previous int                    `json:"previous_issue,omitempty"`
	Status   watcher.IncidentStatus `json:"status"`
	Failures int                    `json:"failures"`
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
	triggerEvents, err := pendingRunEvents(cfg.Repository, issues)
	if err != nil {
		return fmt.Errorf("read pending workflow runs: %w", err)
	}
	if cfg.TriggerRunID > 0 {
		triggerEvents = append(triggerEvents, triggerEvent{RunID: cfg.TriggerRunID, WorkflowName: cfg.TriggerWorkflowName})
	}
	attempts, err := client.collectAttempts(ctx, cfg.ActivatedAt, cfg.ReconcileAt, triggerEvents)
	if err != nil {
		return fmt.Errorf("collect workflow attempts: %w", err)
	}
	plan, err := watcher.Reconcile(cfg.Repository, client.appLogin, attempts, issues, cfg.ReconcileAt)
	if err != nil {
		return fmt.Errorf("plan incident reconciliation: %w", err)
	}
	if len(plan.Changes) > 0 {
		for i := range plan.Changes {
			change := &plan.Changes[i]
			for _, event := range triggerEvents {
				if event.WorkflowName == change.WorkflowName {
					change.PendingRunIDs = append(change.PendingRunIDs, event.RunID)
				}
			}
		}
		if err := client.requireLabels(ctx, plan); err != nil {
			return err
		}
		if err := client.applyPlan(ctx, plan, issues); err != nil {
			return fmt.Errorf("apply incident reconciliation: %w", err)
		}
	}
	result := resultFor(len(attempts), plan)
	if err := json.NewEncoder(stdout).Encode(result); err != nil {
		return fmt.Errorf("write reconciliation result: %w", err)
	}
	return nil
}

func pendingRunEvents(repo watcher.RepositoryIdentity, issues []watcher.IssueSnapshot) ([]triggerEvent, error) {
	eventsByID := make(map[int64]string)
	for _, issue := range issues {
		if issue.State != "open" {
			continue
		}
		state, err := watcher.RenderManagedBodyState(issue.Body, repo)
		if err != nil {
			return nil, fmt.Errorf("issue #%d: %w", issue.Number, err)
		}
		for _, runID := range state.PendingRunIDs {
			if previous, exists := eventsByID[runID]; exists && previous != state.WorkflowName {
				return nil, fmt.Errorf("pending workflow run %d identifies conflicting workflows %q and %q", runID, previous, state.WorkflowName)
			}
			eventsByID[runID] = state.WorkflowName
			if len(eventsByID) > maxPendingRunEvents {
				return nil, fmt.Errorf("managed issues contain more than %d pending workflow runs", maxPendingRunEvents)
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
		events = append(events, triggerEvent{RunID: runID, WorkflowName: eventsByID[runID]})
	}
	return events, nil
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
		Repository: watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"},
		EventName:  eventName, EventPath: eventPath, EventRef: eventRef,
		TriggerRunID: trigger.RunID, TriggerWorkflowName: trigger.WorkflowName,
		APIURL: apiURL, Token: token, AppSlug: appSlug, ActivatedAt: activated.UTC(), ReconcileAt: now.UTC(),
	}, nil
}

type triggerEvent struct {
	RunID        int64
	WorkflowName string
}

func validateTriggerEvent(eventName, eventPath string) error {
	_, err := readTriggerEvent(eventName, eventPath)
	return err
}

func readTriggerEvent(eventName, eventPath string) (triggerEvent, error) {
	var trigger triggerEvent
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
	if eventName == "workflow_run" {
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
	}
	return trigger, nil
}

func resultFor(attemptCount int, plan watcher.Plan) commandResult {
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
	return result
}
