package main

import (
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

	watcher "github.com/orang-gaboets/octostate/internal/actionswatcher"
)

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

func TestValidateTriggerEventRequiresCompletedAllowlistedWorkflowRun(t *testing.T) {
	path := t.TempDir() + "/event.json"
	for _, tc := range []struct {
		name    string
		payload string
		wantErr bool
	}{
		{name: "completed allowlisted", payload: `{"action":"completed","workflow_run":{"name":"CI"}}`},
		{name: "wrong action", payload: `{"action":"requested","workflow_run":{"name":"CI"}}`, wantErr: true},
		{name: "unknown workflow", payload: `{"action":"completed","workflow_run":{"name":"Untrusted"}}`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.payload), 0o600); err != nil {
				t.Fatal(err)
			}
			err := validateTriggerEvent("workflow_run", path)
			if (err != nil) != tc.wantErr {
				t.Fatalf("validateTriggerEvent() error = %v, wantErr=%v", err, tc.wantErr)
			}
		})
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

func TestCollectAttemptsPaginatesWorkflowsAndRunsAndReadsEveryAttempt(t *testing.T) {
	const repoID = 99
	var workflowPageCalls, runPageCalls, attemptCalls, jobCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want installation bearer token", got)
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v3/")
		switch {
		case path == "installation/repositories":
			writeJSON(w, 200, map[string]any{"total_count": 1, "repositories": []any{map[string]any{"id": repoID, "full_name": expectedRepository}}})
		case path == "repos/orang-gaboets/octostate/actions/workflows":
			workflowPageCalls.Add(1)
			workflows := workflowFixtures()
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `</api/v3/repos/orang-gaboets/octostate/actions/workflows?per_page=100&page=2>; rel="next"`)
				writeJSON(w, 200, map[string]any{"total_count": 7, "workflows": workflows[:6]})
			} else {
				writeJSON(w, 200, map[string]any{"total_count": 7, "workflows": workflows[6:]})
			}
		case strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/workflows/") && strings.HasSuffix(path, "/runs"):
			id := strings.TrimSuffix(strings.TrimPrefix(path, "repos/orang-gaboets/octostate/actions/workflows/"), "/runs")
			if id != "1" {
				writeJSON(w, 200, map[string]any{"total_count": 0, "workflow_runs": []any{}})
				return
			}
			runPageCalls.Add(1)
			if r.URL.Query().Get("page") == "1" {
				w.Header().Set("Link", `</api/v3/repos/orang-gaboets/octostate/actions/workflows/1/runs?per_page=100&page=2>; rel="next"`)
				writeJSON(w, 200, map[string]any{"total_count": 2, "workflow_runs": []any{runFixture(repoID, 101, 2, "failure")}})
			} else {
				writeJSON(w, 200, map[string]any{"total_count": 2, "workflow_runs": []any{runFixture(repoID, 102, 1, "failure")}})
			}
		case strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/") && strings.Contains(path, "/attempts/") && !strings.HasSuffix(path, "/jobs"):
			attemptCalls.Add(1)
			parts := strings.Split(path, "/")
			runID, _ := strconv.ParseInt(parts[5], 10, 64)
			attemptNumber, _ := strconv.Atoi(parts[7])
			writeJSON(w, 200, runAttemptFixture(repoID, runID, attemptNumber, "failure"))
		case strings.HasPrefix(path, "repos/orang-gaboets/octostate/actions/runs/") && strings.HasSuffix(path, "/jobs"):
			jobCalls.Add(1)
			if strings.Contains(path, "/102/attempts/1/jobs") {
				writeJSON(w, http.StatusNotFound, map[string]any{"message": "job data expired"})
				return
			}
			writeJSON(w, 200, map[string]any{"total_count": 0, "jobs": []any{}})
		default:
			t.Errorf("unexpected API request: %s %s", r.Method, r.URL.String())
			writeJSON(w, 404, map[string]any{"message": "unexpected route"})
		}
	}))
	defer server.Close()
	client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: repoID, Owner: "orang-gaboets", Name: "octostate"})
	activatedAt := time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	attempts, err := client.collectAttempts(context.Background(), activatedAt, now)
	if err != nil {
		t.Fatal(err)
	}
	if workflowPageCalls.Load() != 2 || runPageCalls.Load() != 2 || attemptCalls.Load() != 3 || jobCalls.Load() != 3 {
		t.Fatalf("API page/attempt/job calls = %d/%d/%d/%d, want 2/2/3/3", workflowPageCalls.Load(), runPageCalls.Load(), attemptCalls.Load(), jobCalls.Load())
	}
	if len(attempts) != 3 {
		t.Fatalf("collected %d attempts, want three distinct attempts", len(attempts))
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
}

func TestCollectAttemptsRejectsInstallationScopeAndAuthorizationFailures(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body       string
		wantPhrase string
	}{
		{name: "unauthorized token", status: http.StatusUnauthorized, body: `{"message":"Bad credentials"}`, wantPhrase: "verify installation repositories"},
		{name: "wrong repository scope", status: http.StatusOK, body: `{"total_count":1,"repositories":[{"id":101,"full_name":"someone/else"}]}`, wantPhrase: "must be scoped only to"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				writeRaw(w, tc.status, tc.body)
			}))
			defer server.Close()
			client := testWatcherClient(t, server.URL, watcher.RepositoryIdentity{ID: 99, Owner: "orang-gaboets", Name: "octostate"})
			_, err := client.collectAttempts(context.Background(), time.Date(2026, 11, 24, 0, 0, 0, 0, time.UTC), time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC))
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
			if request.Body != change.InitialBody || request.Body == change.Body {
				t.Errorf("create body = initial? %v final? %v", request.Body == change.InitialBody, request.Body == change.Body)
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
	client.now = func() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC) }
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
