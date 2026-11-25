package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	gh "github.com/google/go-github/v88/github"
	watcher "github.com/orang-gaboets/octostate/internal/actionswatcher"
	githubclient "github.com/orang-gaboets/octostate/pkg/github/client"
)

const (
	apiPageSize             = 100
	apiResultLimit          = 1000
	maxRateRetries          = 3
	maxRateWait             = time.Minute
	rateLimitRequestTimeout = time.Duration(maxRateRetries+1)*githubclient.DefaultTimeout + time.Duration(maxRateRetries)*maxRateWait
	maxAttemptHistory       = 20000
)

var expectedWorkflowPaths = map[string]string{
	"CI":                          ".github/workflows/ci.yml",
	"CodeQL Security":             ".github/workflows/codeql.yml",
	"Go vulnerability monitoring": ".github/workflows/govulncheck.yml",
	"Go toolchain remediation":    ".github/workflows/go-toolchain-remediation.yml",
	"Release Please":              ".github/workflows/release-please.yml",
	"Contributor showcase":        ".github/workflows/contributors.yml",
	"Trusted Live Integration":    ".github/workflows/live-integration.yml",
}

type watcherClient struct {
	client   *gh.Client
	repo     watcher.RepositoryIdentity
	appLogin string
}

func newWatcherClient(ctx context.Context, token, apiURL string, repo watcher.RepositoryIdentity, appSlug string) (*watcherClient, error) {
	if token == "" {
		return nil, errors.New("installation token is empty")
	}
	if appSlug != expectedAppSlug {
		return nil, fmt.Errorf("unexpected GitHub App slug %q", appSlug)
	}
	apiURL, err := normalizeAPIURL(apiURL)
	if err != nil {
		return nil, err
	}
	pat, err := githubclient.NewPAT(ctx, token, githubclient.WithTimeout(rateLimitRequestTimeout))
	if err != nil {
		return nil, fmt.Errorf("construct authenticated GitHub client: %w", err)
	}
	httpClient := *pat.Client()
	baseTransport := httpClient.Transport
	if baseTransport == nil {
		baseTransport = http.DefaultTransport
	}
	httpClient.Transport = &rateLimitTransport{base: baseTransport, wait: waitContext, now: time.Now, maxRetries: maxRateRetries, maxWait: maxRateWait}
	client, err := gh.NewClient(gh.WithHTTPClient(&httpClient), gh.WithEnterpriseURLs(apiURL, apiURL))
	if err != nil {
		return nil, fmt.Errorf("configure GitHub API URL: %w", err)
	}
	return &watcherClient{client: client, repo: repo, appLogin: appSlug + "[bot]"}, nil
}

func normalizeAPIURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("GITHUB_API_URL is not a valid API origin: %q", raw)
	}
	if u.Scheme != "https" && (u.Scheme != "http" || !isLoopbackHost(u.Hostname())) {
		return "", errors.New("GITHUB_API_URL must use HTTPS (HTTP is allowed only for a loopback test server)")
	}
	if strings.HasSuffix(u.Path, "/api/v3") {
		u.Path += "/"
	}
	return u.String(), nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	parsed := netParseIP(host)
	return parsed != nil && parsed.IsLoopback()
}

// netParseIP is kept as a small wrapper so API URL validation remains testable
// without making callers depend on a particular URL host spelling.
func netParseIP(host string) net.IP { return net.ParseIP(host) }

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

type rateLimitTransport struct {
	base       http.RoundTripper
	wait       func(context.Context, time.Duration) error
	now        func() time.Time
	maxRetries int
	maxWait    time.Duration
}

func (t *rateLimitTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method != http.MethodGet {
		return t.base.RoundTrip(req)
	}
	for attempt := 0; ; attempt++ {
		resp, err := t.base.RoundTrip(req)
		if err != nil || !retryableRateResponse(resp) || attempt >= t.maxRetries {
			return resp, err
		}
		delay, ok := rateRetryDelay(resp.Header, t.now(), attempt)
		if !ok || delay > t.maxWait {
			return resp, nil
		}
		if resp.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			_ = resp.Body.Close()
		}
		if err := t.wait(req.Context(), delay); err != nil {
			return nil, err
		}
	}
}

func retryableRateResponse(resp *http.Response) bool {
	if resp == nil || resp.Body == nil {
		return false
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if resp.StatusCode != http.StatusForbidden {
		return false
	}
	if resp.Header.Get("Retry-After") != "" || resp.Header.Get("X-RateLimit-Remaining") == "0" {
		return true
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return false
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return bytes.Contains(bytes.ToLower(body), []byte("secondary rate limit"))
}

func rateRetryDelay(header http.Header, now time.Time, attempt int) (time.Duration, bool) {
	if raw := strings.TrimSpace(header.Get("Retry-After")); raw != "" {
		if seconds, err := strconv.ParseInt(raw, 10, 64); err == nil {
			if seconds < 0 {
				return 0, false
			}
			maxSeconds := int64((1<<63 - 1) / int64(time.Second))
			if seconds > maxSeconds {
				return time.Duration(1<<63 - 1), true
			}
			return time.Duration(seconds) * time.Second, true
		}
		if at, err := http.ParseTime(raw); err == nil {
			return max(time.Duration(0), at.Sub(now)), true
		}
		return 0, false
	}
	if header.Get("X-RateLimit-Remaining") == "0" {
		if raw := strings.TrimSpace(header.Get("X-RateLimit-Reset")); raw != "" {
			if reset, err := strconv.ParseInt(raw, 10, 64); err == nil {
				return max(time.Duration(0), time.Unix(reset, 0).Sub(now)), true
			}
		}
	}
	return time.Minute << attempt, true
}

func (c *watcherClient) verifyInstallationScope(ctx context.Context) error {
	var repos []*gh.Repository
	for page := 1; ; page++ {
		listed, resp, err := c.client.Apps.ListRepos(ctx, &gh.ListOptions{PerPage: apiPageSize, Page: page})
		if err != nil {
			return fmt.Errorf("verify installation repositories: %w", err)
		}
		if listed == nil {
			return errors.New("installation repository response is empty")
		}
		repos = append(repos, listed.Repositories...)
		next, done := nextPage(resp, len(listed.Repositories), apiPageSize, page)
		if done {
			break
		}
		page = next - 1
	}
	if len(repos) != 1 || repos[0] == nil || repos[0].GetID() != c.repo.ID || !strings.EqualFold(repos[0].GetFullName(), c.repo.Owner+"/"+c.repo.Name) {
		return fmt.Errorf("installation token must be scoped only to %s with repository ID %d", c.repo.Owner+"/"+c.repo.Name, c.repo.ID)
	}
	return nil
}

func (c *watcherClient) listWorkflows(ctx context.Context) (map[string]*gh.Workflow, error) {
	result := make(map[string]*gh.Workflow, len(expectedWorkflowPaths))
	for page := 1; ; page++ {
		listed, resp, err := c.client.Actions.ListWorkflows(ctx, c.repo.Owner, c.repo.Name, &gh.ListOptions{PerPage: apiPageSize, Page: page})
		if err != nil {
			return nil, fmt.Errorf("list repository workflows: %w", err)
		}
		if listed == nil {
			return nil, errors.New("workflow list response is empty")
		}
		for _, workflow := range listed.Workflows {
			if workflow == nil || !watcher.IsMonitoredWorkflow(workflow.GetName()) {
				continue
			}
			name := workflow.GetName()
			if _, exists := result[name]; exists {
				return nil, fmt.Errorf("GitHub returned duplicate workflow %q", name)
			}
			if workflow.GetID() <= 0 || workflowPath(workflow.GetPath()) != expectedWorkflowPaths[name] {
				return nil, fmt.Errorf("workflow %q has an unexpected API identity or path", name)
			}
			result[name] = workflow
		}
		next, done := nextPage(resp, len(listed.Workflows), apiPageSize, page)
		if done {
			break
		}
		page = next - 1
	}
	if len(result) != len(expectedWorkflowPaths) {
		return nil, fmt.Errorf("expected %d monitored workflows, found %d", len(expectedWorkflowPaths), len(result))
	}
	return result, nil
}

func workflowPath(raw string) string {
	path, _, _ := strings.Cut(raw, "@")
	return path
}

type attemptCollector struct {
	client       *watcherClient
	activatedAt  time.Time
	seen         map[string]watcher.RunAttempt
	seenRunNames map[int64]string
	result       []watcher.RunAttempt
	unavailable  []unavailableRun
}

func unavailableWorkflowRunStatus(statusCode int) bool {
	return statusCode == http.StatusNotFound || statusCode == http.StatusGone
}

func (c *watcherClient) collectAttempts(ctx context.Context, activatedAt, now time.Time, triggerEvents []triggerEvent) ([]watcher.RunAttempt, []unavailableRun, error) {
	if activatedAt.IsZero() || activatedAt.After(now) {
		return nil, nil, errors.New("watcher activation time is missing or in the future")
	}
	eventsByID := make(map[int64]triggerEvent, len(triggerEvents))
	for _, event := range triggerEvents {
		if event.RunID <= 0 || !watcher.IsMonitoredWorkflow(event.WorkflowName) {
			return nil, nil, errors.New("workflow_run trigger identity is incomplete or unmonitored")
		}
		if previous, exists := eventsByID[event.RunID]; exists {
			if previous.WorkflowName != event.WorkflowName {
				return nil, nil, fmt.Errorf("workflow run %d is pending for conflicting workflows %q and %q", event.RunID, previous.WorkflowName, event.WorkflowName)
			}
			if previous.IssueNumber == 0 && event.IssueNumber != 0 {
				eventsByID[event.RunID] = event
			}
		} else {
			eventsByID[event.RunID] = event
		}
	}
	if err := c.verifyInstallationScope(ctx); err != nil {
		return nil, nil, err
	}
	workflows, err := c.listWorkflows(ctx)
	if err != nil {
		return nil, nil, err
	}
	collector := attemptCollector{
		client: c, activatedAt: activatedAt,
		seen: make(map[string]watcher.RunAttempt), seenRunNames: make(map[int64]string),
	}
	if err := collector.collect(ctx, workflows, now.Add(-lookbackWindow), now, eventsByID); err != nil {
		return nil, nil, err
	}
	return collector.result, collector.unavailable, nil
}

func (collector *attemptCollector) collect(ctx context.Context, workflows map[string]*gh.Workflow, start, end time.Time, eventsByID map[int64]triggerEvent) error {
	names := make([]string, 0, len(workflows))
	for name := range workflows {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		workflow := workflows[name]
		rows, err := collector.client.listRunsInRange(ctx, workflow.GetID(), start, end)
		if err != nil {
			return fmt.Errorf("list runs for workflow %q: %w", name, err)
		}
		for _, row := range rows {
			if row == nil {
				continue
			}
			if pendingEvent, pending := eventsByID[row.GetID()]; pending {
				if name != pendingEvent.WorkflowName {
					return fmt.Errorf("pending workflow run %d names %q but the run list associates it with %q", row.GetID(), pendingEvent.WorkflowName, name)
				}
				continue
			}
			if err := collector.processRun(ctx, workflow, row); err != nil {
				return err
			}
		}
	}
	runIDs := make([]int64, 0, len(eventsByID))
	for runID := range eventsByID {
		runIDs = append(runIDs, runID)
	}
	sort.Slice(runIDs, func(i, j int) bool { return runIDs[i] < runIDs[j] })
	for _, runID := range runIDs {
		event := eventsByID[runID]
		workflow := workflows[event.WorkflowName]
		if workflow == nil {
			return fmt.Errorf("pending workflow run %d identifies an unavailable workflow %q", runID, event.WorkflowName)
		}
		run, response, err := collector.client.client.Actions.GetWorkflowRunByID(ctx, collector.client.repo.Owner, collector.client.repo.Name, runID)
		if err != nil {
			if response != nil && response.Response != nil && unavailableWorkflowRunStatus(response.StatusCode) {
				collector.unavailable = append(collector.unavailable, unavailableRun{Event: event, StatusCode: response.StatusCode})
				continue
			}
			return fmt.Errorf("re-fetch pending workflow run %d: %w", runID, err)
		}
		if run == nil || run.GetID() != runID {
			return fmt.Errorf("pending workflow run %d returned a conflicting identity", runID)
		}
		if err := collector.processRun(ctx, workflow, run); err != nil {
			return err
		}
	}
	return nil
}

func (collector *attemptCollector) processRun(ctx context.Context, workflow *gh.Workflow, row *gh.WorkflowRun) error {
	if row == nil {
		return nil
	}
	if previous, exists := collector.seenRunNames[row.GetID()]; exists && previous != workflow.GetName() {
		return fmt.Errorf("run ID %d is associated with conflicting workflows %q and %q", row.GetID(), previous, workflow.GetName())
	}
	collector.seenRunNames[row.GetID()] = workflow.GetName()
	if err := validateWorkflowRunIdentity(row, collector.client.repo, workflow); err != nil {
		return fmt.Errorf("validate workflow run for %q: %w", workflow.GetName(), err)
	}
	eligible, err := eligibleWorkflowRun(row, collector.client.repo)
	if err != nil {
		return fmt.Errorf("validate source repository for run %d: %w", row.GetID(), err)
	}
	if !eligible {
		return nil
	}
	attempts, err := collector.client.readRunAttempts(ctx, workflow, row, collector.activatedAt)
	if err != nil {
		return fmt.Errorf("read attempts for run %d: %w", row.GetID(), err)
	}
	for _, attempt := range attempts {
		key := fmt.Sprintf("%d/%d/%d", attempt.WorkflowID, attempt.RunID, attempt.Attempt)
		if old, exists := collector.seen[key]; exists {
			if !reflect.DeepEqual(old, attempt) {
				return fmt.Errorf("API returned conflicting duplicate attempt %s", key)
			}
			continue
		}
		collector.seen[key] = attempt
		collector.result = append(collector.result, attempt)
		if len(collector.result) > maxAttemptHistory {
			return fmt.Errorf("lookback returned more than %d workflow attempts; refusing incomplete history", maxAttemptHistory)
		}
	}
	return nil
}

func (c *watcherClient) listRunsInRange(ctx context.Context, workflowID int64, start, end time.Time) ([]*gh.WorkflowRun, error) {
	if !start.Before(end) {
		return nil, nil
	}
	created := start.UTC().Format(time.RFC3339Nano) + ".." + end.UTC().Format(time.RFC3339Nano)
	var rows []*gh.WorkflowRun
	for page := 1; ; page++ {
		listed, resp, err := c.client.Actions.ListWorkflowRunsByID(ctx, c.repo.Owner, c.repo.Name, workflowID, &gh.ListWorkflowRunsOptions{
			Created: created, ExcludePullRequests: true, ListOptions: gh.ListOptions{PerPage: apiPageSize, Page: page},
		})
		if err != nil {
			return nil, err
		}
		if listed == nil {
			return nil, errors.New("workflow run response is empty")
		}
		if page == 1 && listed.GetTotalCount() > apiResultLimit {
			mid := start.Add(end.Sub(start) / 2)
			if mid.Equal(start) || mid.Equal(end) {
				return nil, fmt.Errorf("workflow run interval exceeds API result cap at %s", start.Format(time.RFC3339))
			}
			first, err := c.listRunsInRange(ctx, workflowID, start, mid)
			if err != nil {
				return nil, err
			}
			second, err := c.listRunsInRange(ctx, workflowID, mid, end)
			if err != nil {
				return nil, err
			}
			return append(first, second...), nil
		}
		rows = append(rows, listed.WorkflowRuns...)
		next, done := nextPage(resp, len(listed.WorkflowRuns), apiPageSize, page)
		if done {
			break
		}
		page = next - 1
	}
	return rows, nil
}

func validateWorkflowRunIdentity(run *gh.WorkflowRun, repo watcher.RepositoryIdentity, workflow *gh.Workflow) error {
	return validateWorkflowRunIdentityFields(run, repo, workflow.GetID(), workflow.GetName())
}

func validateWorkflowRunIdentityFields(run *gh.WorkflowRun, repo watcher.RepositoryIdentity, workflowID int64, workflowName string) error {
	if run.GetID() <= 0 {
		return errors.New("workflow run is missing a positive run ID")
	}
	if run.GetWorkflowID() != workflowID || run.GetName() != workflowName || workflowPath(run.GetPath()) != expectedWorkflowPaths[workflowName] {
		return fmt.Errorf("API workflow ID, name, or path does not match %q", workflowName)
	}
	if run.Repository == nil || run.Repository.GetID() != repo.ID {
		return fmt.Errorf("run repository does not match repository ID %d", repo.ID)
	}
	return nil
}

func (c *watcherClient) verifyOpenIncidentCursors(ctx context.Context, issues []watcher.IssueSnapshot) error {
	for _, issue := range issues {
		if issue.State != "open" {
			continue
		}
		state, err := watcher.RenderManagedBodyState(issue.Body, c.repo)
		if err != nil {
			return fmt.Errorf("read latest attempt from issue #%d: %w", issue.Number, err)
		}
		var verifyErr error
		if isRunFailure(state.Latest.Conclusion) {
			verifyErr = c.verifyPersistedFailure(ctx, state.Latest)
		} else {
			verifyErr = c.verifyPersistedAttempt(ctx, state.Latest)
		}
		if verifyErr != nil {
			return fmt.Errorf("verify persisted latest attempt for issue #%d: %w", issue.Number, verifyErr)
		}
	}
	return nil
}

func (c *watcherClient) verifyPersistedAttempt(ctx context.Context, expected watcher.RunAttempt) error {
	run, _, err := c.client.Actions.GetWorkflowRunAttempt(ctx, c.repo.Owner, c.repo.Name, expected.RunID, expected.Attempt, nil)
	if err != nil {
		return err
	}
	if run == nil {
		return errors.New("workflow API returned an empty attempt")
	}
	if err := validateWorkflowRunIdentityFields(run, c.repo, expected.WorkflowID, expected.WorkflowName); err != nil {
		return err
	}
	eligible, err := eligibleWorkflowRun(run, c.repo)
	if err != nil {
		return err
	}
	if !eligible {
		return errors.New("persisted attempt is not eligible")
	}
	started := run.RunStartedAt.GetTime()
	completed := run.UpdatedAt.GetTime()
	if started == nil || completed == nil || run.GetStatus() != "completed" ||
		run.GetID() != expected.RunID || run.GetRunNumber() != expected.RunNumber || run.GetRunAttempt() != expected.Attempt ||
		run.GetEvent() != expected.Event || run.GetHeadBranch() != expected.HeadBranch || run.GetConclusion() != expected.Conclusion ||
		run.GetHeadSHA() != expected.HeadSHA || run.GetHTMLURL() != expected.HTMLURL ||
		!started.Equal(expected.StartedAt) || !completed.Equal(expected.CompletedAt) {
		return errors.New("workflow API metadata does not match the persisted attempt")
	}
	return nil
}

func (c *watcherClient) verifyPersistedFailure(ctx context.Context, expected watcher.RunAttempt) error {
	if !isRunFailure(expected.Conclusion) {
		return errors.New("archive contains a non-failure attempt")
	}
	if !expected.JobDataAvailable && len(expected.FailedJobs) != 0 {
		return errors.New("workflow API job metadata does not match the persisted failure")
	}
	if err := c.verifyPersistedAttempt(ctx, expected); err != nil {
		return err
	}
	failedJobs, available, err := c.listFailedJobs(ctx, expected.RunID, expected.Attempt)
	if err != nil {
		return err
	}
	if !expected.JobDataAvailable {
		// A checkpoint can record an unavailable failed-job read. If a later
		// read finds jobs for the same verified attempt, reconciliation can
		// enrich the persisted failure record with that evidence.
		return nil
	}
	expectedJobs := append([]string(nil), expected.FailedJobs...)
	sort.Strings(expectedJobs)
	if !available || !reflect.DeepEqual(failedJobs, expectedJobs) {
		return errors.New("workflow API job metadata does not match the persisted failure")
	}
	return nil
}

func eligibleWorkflowRun(run *gh.WorkflowRun, repo watcher.RepositoryIdentity) (bool, error) {
	if run.GetHeadBranch() != "main" {
		return false, nil
	}
	if run.GetEvent() != "push" && run.GetEvent() != "schedule" && run.GetEvent() != "workflow_dispatch" {
		return false, nil
	}
	if run.HeadRepository == nil || run.HeadRepository.GetID() != repo.ID {
		return false, fmt.Errorf("eligible %s run on main has an unexpected head repository", run.GetEvent())
	}
	return true, nil
}

func (c *watcherClient) readRunAttempts(ctx context.Context, workflow *gh.Workflow, listed *gh.WorkflowRun, activatedAt time.Time) ([]watcher.RunAttempt, error) {
	latest := listed.GetRunAttempt()
	if latest < 1 || latest > 1000 {
		return nil, fmt.Errorf("invalid latest run attempt %d", latest)
	}
	result := make([]watcher.RunAttempt, 0, latest)
	for attemptNumber := 1; attemptNumber <= latest; attemptNumber++ {
		run, _, err := c.client.Actions.GetWorkflowRunAttempt(ctx, c.repo.Owner, c.repo.Name, listed.GetID(), attemptNumber, nil)
		if err != nil {
			return nil, err
		}
		if run == nil || run.GetID() != listed.GetID() || run.GetRunAttempt() != attemptNumber {
			return nil, fmt.Errorf("attempt %d returned conflicting workflow identity", attemptNumber)
		}
		if err := validateWorkflowRunIdentity(run, c.repo, workflow); err != nil {
			return nil, err
		}
		eligible, err := eligibleWorkflowRun(run, c.repo)
		if err != nil {
			return nil, err
		}
		if !eligible {
			continue
		}
		conclusion := run.GetConclusion()
		if run.GetStatus() != "completed" || (!isRunFailure(conclusion) && conclusion != "success") {
			continue
		}
		started := run.RunStartedAt.GetTime()
		if started == nil || started.IsZero() {
			return nil, fmt.Errorf("attempt %d is missing valid run start or completion timestamps", attemptNumber)
		}
		if started.Before(activatedAt) {
			continue
		}
		completed := run.UpdatedAt.GetTime()
		if completed == nil || completed.IsZero() || completed.Before(*started) {
			return nil, fmt.Errorf("attempt %d is missing valid run start or completion timestamps", attemptNumber)
		}
		failedJobs := []string(nil)
		jobsAvailable := true
		if isRunFailure(conclusion) {
			failedJobs, jobsAvailable, err = c.listFailedJobs(ctx, listed.GetID(), attemptNumber)
			if err != nil {
				return nil, err
			}
		}
		result = append(result, watcher.RunAttempt{
			WorkflowID: workflow.GetID(), WorkflowName: workflow.GetName(), RunID: run.GetID(), RunNumber: run.GetRunNumber(), Attempt: attemptNumber,
			Event: run.GetEvent(), HeadBranch: run.GetHeadBranch(), Conclusion: conclusion, HeadSHA: run.GetHeadSHA(), HTMLURL: run.GetHTMLURL(),
			StartedAt: *started, CompletedAt: *completed, FailedJobs: failedJobs, JobDataAvailable: jobsAvailable,
		})
	}
	return result, nil
}

func (c *watcherClient) listFailedJobs(ctx context.Context, runID int64, attemptNumber int) ([]string, bool, error) {
	var names []string
	for page := 1; ; page++ {
		jobs, resp, err := c.client.Actions.ListWorkflowJobsAttempt(ctx, c.repo.Owner, c.repo.Name, runID, int64(attemptNumber), &gh.ListOptions{PerPage: apiPageSize, Page: page})
		if err != nil {
			if responseStatus(err) == http.StatusNotFound || responseStatus(err) == http.StatusGone {
				return nil, false, nil
			}
			return nil, false, err
		}
		if jobs == nil {
			return nil, false, errors.New("workflow jobs response is empty")
		}
		for _, job := range jobs.Jobs {
			if job == nil || (job.GetConclusion() != "failure" && job.GetConclusion() != "timed_out" && job.GetConclusion() != "startup_failure") {
				continue
			}
			name := strings.TrimSpace(job.GetName())
			if name == "" {
				return nil, false, errors.New("failed workflow job has no name")
			}
			names = append(names, name)
		}
		next, done := nextPage(resp, len(jobs.Jobs), apiPageSize, page)
		if done {
			break
		}
		page = next - 1
	}
	sort.Strings(names)
	return slices.Compact(names), true, nil
}

func isRunFailure(conclusion string) bool {
	return conclusion == "failure" || conclusion == "timed_out" || conclusion == "startup_failure"
}

func responseStatus(err error) int {
	var response *gh.ErrorResponse
	if errors.As(err, &response) && response.Response != nil {
		return response.Response.StatusCode
	}
	return 0
}

func nextPage(resp *gh.Response, count, pageSize, currentPage int) (int, bool) {
	if resp != nil && resp.NextPage > currentPage {
		return resp.NextPage, false
	}
	if count >= pageSize {
		return currentPage + 1, false
	}
	return 0, true
}

func (c *watcherClient) listManagedIssues(ctx context.Context) ([]watcher.IssueSnapshot, error) {
	var all []*gh.Issue
	for page := 1; ; page++ {
		issues, resp, err := c.client.Issues.ListByRepo(ctx, c.repo.Owner, c.repo.Name, &gh.IssueListByRepoOptions{
			State: "all", ListOptions: gh.ListOptions{PerPage: apiPageSize, Page: page},
		})
		if err != nil {
			return nil, err
		}
		all = append(all, issues...)
		next, done := nextPage(resp, len(issues), apiPageSize, page)
		if done {
			break
		}
		page = next - 1
	}
	result := make([]watcher.IssueSnapshot, 0)
	for _, issue := range all {
		if issue == nil || issue.IsPullRequest() {
			continue
		}
		// Public users can spoof watcher titles or body markers in this repo.
		// Only inspect candidates authored by the app; malformed app-owned
		// incidents still fail closed below.
		if issue.GetUser() == nil || issue.GetUser().GetLogin() != c.appLogin {
			continue
		}
		candidate := watcherCandidate(issue)
		if !candidate {
			continue
		}
		if !strings.Contains(issue.GetBody(), "<!-- actions-watcher:v1 ") {
			return nil, fmt.Errorf("issue #%d carries a watcher label but has no identity marker", issue.GetNumber())
		}
		snapshot, err := c.issueSnapshot(ctx, issue)
		if err != nil {
			return nil, fmt.Errorf("read watcher issue #%d: %w", issue.GetNumber(), err)
		}
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Number < result[j].Number })
	return result, nil
}

func watcherCandidate(issue *gh.Issue) bool {
	if strings.Contains(issue.GetBody(), "<!-- actions-watcher:v1") {
		return true
	}
	for name := range expectedWorkflowPaths {
		if issue.GetTitle() == "ci(actions): investigate "+name+" workflow failures on main" {
			return true
		}
	}
	for _, label := range issue.Labels {
		if label != nil && (label.GetName() == watcher.ManagedLabel || label.GetName() == watcher.FailingLabel || label.GetName() == watcher.RecoveredLabel) {
			return true
		}
	}
	return false
}

func (c *watcherClient) issueSnapshot(ctx context.Context, issue *gh.Issue) (watcher.IssueSnapshot, error) {
	comments := make([]watcher.CommentSnapshot, 0)
	for page := 1; ; page++ {
		listed, resp, err := c.client.Issues.ListComments(ctx, c.repo.Owner, c.repo.Name, issue.GetNumber(), &gh.IssueListCommentsOptions{ListOptions: gh.ListOptions{PerPage: apiPageSize, Page: page}})
		if err != nil {
			return watcher.IssueSnapshot{}, err
		}
		for _, comment := range listed {
			if comment == nil {
				continue
			}
			author := ""
			if comment.User != nil {
				author = comment.User.GetLogin()
			}
			comments = append(comments, watcher.CommentSnapshot{ID: comment.GetID(), AuthorLogin: author, Body: comment.GetBody()})
		}
		next, done := nextPage(resp, len(listed), apiPageSize, page)
		if done {
			break
		}
		page = next - 1
	}
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		if label != nil {
			labels = append(labels, label.GetName())
		}
	}
	sort.Strings(labels)
	closedAt := issue.ClosedAt.GetTime()
	author := ""
	if issue.User != nil {
		author = issue.User.GetLogin()
	}
	return watcher.IssueSnapshot{
		Number: issue.GetNumber(), State: issue.GetState(), ClosedAt: closedAt, AuthorLogin: author,
		Title: issue.GetTitle(), Body: issue.GetBody(), Labels: labels, Comments: comments,
	}, nil
}

func (c *watcherClient) getIssueSnapshot(ctx context.Context, number int) (watcher.IssueSnapshot, error) {
	issue, _, err := c.client.Issues.Get(ctx, c.repo.Owner, c.repo.Name, number)
	if err != nil {
		return watcher.IssueSnapshot{}, err
	}
	if issue == nil || issue.IsPullRequest() {
		return watcher.IssueSnapshot{}, fmt.Errorf("issue #%d is missing or is a pull request", number)
	}
	return c.issueSnapshot(ctx, issue)
}

func (c *watcherClient) requireLabels(ctx context.Context, plan watcher.Plan) error {
	required := make(map[string]bool)
	for _, change := range plan.Changes {
		if change.Action == watcher.ActionCreate {
			required[watcher.ManagedLabel] = true
		}
		for _, label := range change.InitialLabels {
			required[label] = true
		}
		for _, label := range change.AddLabels {
			required[label] = true
		}
	}
	if len(required) == 0 {
		return nil
	}
	labels := make(map[string]bool)
	for page := 1; ; page++ {
		listed, resp, err := c.client.Issues.ListLabels(ctx, c.repo.Owner, c.repo.Name, &gh.ListOptions{PerPage: apiPageSize, Page: page})
		if err != nil {
			return fmt.Errorf("verify watcher label configuration: %w", err)
		}
		for _, label := range listed {
			if label != nil {
				labels[label.GetName()] = true
			}
		}
		next, done := nextPage(resp, len(listed), apiPageSize, page)
		if done {
			break
		}
		page = next - 1
	}
	requiredNames := make([]string, 0, len(required))
	for name := range required {
		requiredNames = append(requiredNames, name)
	}
	sort.Strings(requiredNames)
	for _, name := range requiredNames {
		if !labels[name] {
			return fmt.Errorf("required issue label %q is missing; maintainer provisioning is required", name)
		}
	}
	return nil
}

func (c *watcherClient) applyPlan(ctx context.Context, plan watcher.Plan, initial []watcher.IssueSnapshot) error {
	if err := c.verifyArchiveAttempts(ctx, plan); err != nil {
		return err
	}
	expected := append([]watcher.IssueSnapshot(nil), initial...)
	for _, change := range plan.Changes {
		switch change.Action {
		case watcher.ActionCreate:
			createChange := change
			staged := change.InitialBody != ""
			if len(change.PendingRunIDs) > 0 {
				checkpointBase := change.Body
				var archiveComments []watcher.CommentSnapshot
				if change.InitialBody != "" {
					checkpointBase = change.InitialBody
				} else {
					for _, archive := range change.Archives {
						archiveComments = append(archiveComments, watcher.CommentSnapshot{AuthorLogin: c.appLogin, Body: archive.Body})
					}
				}
				checkpointBody, checkpointArchives, err := watcher.RenderCheckpointBody(checkpointBase, c.repo, change.PendingRunIDs, archiveComments)
				if err != nil {
					return fmt.Errorf("checkpoint pending runs for %q: %w", change.WorkflowName, err)
				}
				if err := c.verifyCheckpointArchives(ctx, change.Archives, checkpointArchives); err != nil {
					return fmt.Errorf("verify checkpoint archives for %q: %w", change.WorkflowName, err)
				}
				createChange.Archives, err = mergeArchiveComments(change.Archives, checkpointArchives)
				if err != nil {
					return fmt.Errorf("merge checkpoint archives for %q: %w", change.WorkflowName, err)
				}
				createChange.InitialBody = checkpointBody
				if change.InitialBody == "" {
					createChange.InitialLabels = append([]string(nil), change.AddLabels...)
				}
				staged = true
			}
			created, err := c.createIncident(ctx, createChange, expected)
			if err != nil {
				return err
			}
			if staged {
				created, err = c.ensureArchiveComments(ctx, created, createChange.Archives)
				if err != nil {
					return fmt.Errorf("issue #%d: %w", created.Number, err)
				}
				created, err = c.ensureTransitionComments(ctx, created, change.Comments)
				if err != nil {
					return fmt.Errorf("issue #%d: %w", created.Number, err)
				}
				finalize := change
				finalize.AddLabels, finalize.RemoveLabels = labelDeltaToTarget(created.Labels, change.AddLabels)
				finalized, err := c.patchIssue(ctx, created, finalize)
				if err != nil {
					return fmt.Errorf("finalize created issue #%d: %w", created.Number, err)
				}
				created = finalized
			} else {
				issueNumber := created.Number
				created, err = c.ensureComments(ctx, created, change.Archives, change.Comments)
				if err != nil {
					return fmt.Errorf("issue #%d: %w", issueNumber, err)
				}
			}
			expected = append(expected, created)
		case watcher.ActionUpdate:
			original, ok := findIssue(expected, change.IssueNumber)
			if !ok {
				return fmt.Errorf("planned issue #%d was absent from the authoritative snapshot", change.IssueNumber)
			}
			current, err := c.getIssueSnapshot(ctx, change.IssueNumber)
			if err != nil {
				return fmt.Errorf("preflight issue #%d: %w", change.IssueNumber, err)
			}
			if !sameIssueSnapshot(original, current) {
				return fmt.Errorf("issue #%d changed during reconciliation; refusing to overwrite a concurrent edit", change.IssueNumber)
			}
			state, err := watcher.RenderManagedBodyState(current.Body, c.repo)
			if err != nil {
				return fmt.Errorf("read pending run state from issue #%d: %w", current.Number, err)
			}
			pendingRunIDs := append(append([]int64(nil), state.PendingRunIDs...), change.PendingRunIDs...)
			if len(pendingRunIDs) > 0 {
				checkpointBody, checkpointArchives, err := watcher.RenderCheckpointBody(current.Body, c.repo, pendingRunIDs, watcherAuthoredComments(current.Comments, c.appLogin))
				if err != nil {
					return fmt.Errorf("checkpoint pending runs for issue #%d: %w", current.Number, err)
				}
				if err := c.verifyCheckpointArchives(ctx, change.Archives, checkpointArchives); err != nil {
					return fmt.Errorf("verify checkpoint archives for issue #%d: %w", current.Number, err)
				}
				current, err = c.ensureArchiveComments(ctx, current, checkpointArchives)
				if err != nil {
					return fmt.Errorf("preserve checkpoint history for issue #%d: %w", current.Number, err)
				}
				checkpoint := watcher.IssueChange{Title: current.Title, Body: checkpointBody}
				current, err = c.patchIssue(ctx, current, checkpoint)
				if err != nil {
					return fmt.Errorf("checkpoint issue #%d: %w", change.IssueNumber, err)
				}
			}
			current, err = c.ensureArchiveComments(ctx, current, change.Archives)
			if err != nil {
				return fmt.Errorf("issue #%d: %w", change.IssueNumber, err)
			}
			current, err = c.ensureTransitionComments(ctx, current, change.Comments)
			if err != nil {
				return fmt.Errorf("issue #%d: %w", change.IssueNumber, err)
			}
			updated, err := c.patchIssue(ctx, current, change)
			if err != nil {
				return fmt.Errorf("update issue #%d: %w", change.IssueNumber, err)
			}
			expected = replaceIssue(expected, updated)
		default:
			return fmt.Errorf("unsupported planned action %q", change.Action)
		}
	}
	return nil
}

func (c *watcherClient) verifyArchiveAttempts(ctx context.Context, plan watcher.Plan) error {
	for _, change := range plan.Changes {
		if change.Action != watcher.ActionUpdate {
			continue
		}
		for _, archive := range change.Archives {
			if len(archive.Attempts) == 0 {
				return fmt.Errorf("issue #%d has an archive with no verifiable failure attempts", change.IssueNumber)
			}
			for _, attempt := range archive.Attempts {
				if err := c.verifyPersistedFailure(ctx, attempt); err != nil {
					return fmt.Errorf("verify failure run %d attempt %d before archiving issue #%d: %w", attempt.RunID, attempt.Attempt, change.IssueNumber, err)
				}
			}
		}
	}
	return nil
}

func (c *watcherClient) verifyCheckpointArchives(ctx context.Context, planned, generated []watcher.ArchiveComment) error {
	plannedMarkers := make(map[string]bool, len(planned))
	for _, archive := range planned {
		plannedMarkers[archive.Marker] = true
	}
	for _, archive := range generated {
		if plannedMarkers[archive.Marker] {
			continue
		}
		if len(archive.Attempts) == 0 {
			return errors.New("checkpoint archive has no verifiable failure attempts")
		}
		for _, attempt := range archive.Attempts {
			if err := c.verifyPersistedFailure(ctx, attempt); err != nil {
				return fmt.Errorf("verify failure run %d attempt %d before checkpoint archive: %w", attempt.RunID, attempt.Attempt, err)
			}
		}
	}
	return nil
}

func watcherAuthoredComments(comments []watcher.CommentSnapshot, appLogin string) []watcher.CommentSnapshot {
	trusted := make([]watcher.CommentSnapshot, 0, len(comments))
	for _, comment := range comments {
		if comment.AuthorLogin == appLogin {
			trusted = append(trusted, comment)
		}
	}
	return trusted
}

func mergeArchiveComments(existing, additional []watcher.ArchiveComment) ([]watcher.ArchiveComment, error) {
	merged := append([]watcher.ArchiveComment(nil), existing...)
	byMarker := make(map[string]string, len(existing)+len(additional))
	for _, archive := range existing {
		if previous, ok := byMarker[archive.Marker]; ok && previous != archive.Body {
			return nil, fmt.Errorf("archive marker %q has conflicting bodies", archive.Marker)
		}
		byMarker[archive.Marker] = archive.Body
	}
	for _, archive := range additional {
		if previous, ok := byMarker[archive.Marker]; ok {
			if previous != archive.Body {
				return nil, fmt.Errorf("archive marker %q has conflicting bodies", archive.Marker)
			}
			continue
		}
		merged = append(merged, archive)
		byMarker[archive.Marker] = archive.Body
	}
	return merged, nil
}

func replaceIssue(issues []watcher.IssueSnapshot, updated watcher.IssueSnapshot) []watcher.IssueSnapshot {
	for i := range issues {
		if issues[i].Number == updated.Number {
			issues[i] = updated
			return issues
		}
	}
	return append(issues, updated)
}

func (c *watcherClient) createIncident(ctx context.Context, change watcher.IssueChange, initial []watcher.IssueSnapshot) (watcher.IssueSnapshot, error) {
	current, err := c.listManagedIssues(ctx)
	if err != nil {
		return watcher.IssueSnapshot{}, fmt.Errorf("recheck incidents before creating %q: %w", change.WorkflowName, err)
	}
	if !sameSnapshotSets(initial, current) {
		return watcher.IssueSnapshot{}, errors.New("managed issue set changed during reconciliation; refusing to create a duplicate incident")
	}
	labels := append([]string(nil), change.AddLabels...)
	body := change.Body
	title := change.Title
	if change.InitialBody != "" {
		labels = append([]string(nil), change.InitialLabels...)
		body = change.InitialBody
	}
	sort.Strings(labels)
	_, _, writeErr := c.client.Issues.Create(ctx, c.repo.Owner, c.repo.Name, &gh.IssueRequest{
		Title: &title, Body: &body, Labels: &labels,
	})
	readback, readErr := c.listManagedIssues(ctx)
	if readErr != nil {
		if writeErr != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("create issue outcome is uncertain (%v), and readback failed: %w", writeErr, readErr)
		}
		return watcher.IssueSnapshot{}, fmt.Errorf("read back created incident: %w", readErr)
	}
	if len(readback) != len(initial)+1 {
		return watcher.IssueSnapshot{}, fmt.Errorf("create readback contains %d managed issues; expected the prior %d plus one new incident", len(readback), len(initial))
	}
	var match *watcher.IssueSnapshot
	for i := range readback {
		if _, existed := findIssue(initial, readback[i].Number); existed {
			continue
		}
		state, stateErr := watcher.RenderBodyState(readback[i].Body, c.repo, change.WorkflowID, change.WorkflowName)
		if stateErr != nil {
			continue
		}
		if state.WorkflowID != change.WorkflowID {
			continue
		}
		if readback[i].AuthorLogin != c.appLogin {
			return watcher.IssueSnapshot{}, fmt.Errorf("incident identity appeared on issue #%d authored by an unexpected actor", readback[i].Number)
		}
		if match != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("multiple issues now identify workflow %q", change.WorkflowName)
		}
		snapshot := readback[i]
		match = &snapshot
	}
	if match == nil {
		if writeErr != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("create incident outcome is uncertain (%w); readback found no matching issue", writeErr)
		}
		return watcher.IssueSnapshot{}, errors.New("create incident succeeded without a matching readback issue")
	}
	remaining := make([]watcher.IssueSnapshot, 0, len(readback)-1)
	for _, issue := range readback {
		if issue.Number != match.Number {
			remaining = append(remaining, issue)
		}
	}
	if !sameSnapshotSets(initial, remaining) {
		return watcher.IssueSnapshot{}, errors.New("managed issue set changed while the incident was being created")
	}
	if match.State != "open" || match.Title != change.Title || match.Body != body || !sameLabels(match.Labels, labels) {
		return watcher.IssueSnapshot{}, fmt.Errorf("created incident #%d did not match its requested state", match.Number)
	}
	return *match, nil
}

func (c *watcherClient) patchIssue(ctx context.Context, current watcher.IssueSnapshot, change watcher.IssueChange) (watcher.IssueSnapshot, error) {
	if current.State != "open" {
		return watcher.IssueSnapshot{}, fmt.Errorf("issue #%d is no longer open", current.Number)
	}
	labels := applyLabelDelta(current.Labels, change.AddLabels, change.RemoveLabels)
	body := change.Body
	title := change.Title
	if current.Body == body && current.Title == title && sameLabels(current.Labels, labels) {
		return current, nil
	}
	request := &gh.IssueRequest{Body: &body, Title: &title, Labels: &labels}
	_, _, writeErr := c.client.Issues.Edit(ctx, c.repo.Owner, c.repo.Name, current.Number, request)
	readback, readErr := c.getIssueSnapshot(ctx, current.Number)
	if readErr != nil {
		if writeErr != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("issue update outcome is uncertain (%v), and readback failed: %w", writeErr, readErr)
		}
		return watcher.IssueSnapshot{}, fmt.Errorf("read back updated issue: %w", readErr)
	}
	if readback.State != "open" || readback.Title != title || readback.Body != body || !sameLabels(readback.Labels, labels) || !sameComments(readback.Comments, current.Comments) {
		if writeErr != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("issue update outcome is uncertain (%w); desired state was not observed", writeErr)
		}
		return watcher.IssueSnapshot{}, fmt.Errorf("issue update readback mismatch (open=%t title=%t body=%t labels=%t comments=%t; expected comment IDs %v, observed %v)",
			readback.State == "open", readback.Title == title, readback.Body == body, sameLabels(readback.Labels, labels), sameComments(readback.Comments, current.Comments),
			commentIDs(current.Comments), commentIDs(readback.Comments))
	}
	return readback, nil
}

func (c *watcherClient) ensureComments(ctx context.Context, initial watcher.IssueSnapshot, archives []watcher.ArchiveComment, comments []watcher.ManagedComment) (watcher.IssueSnapshot, error) {
	current := initial
	var err error
	current, err = c.ensureArchiveComments(ctx, current, archives)
	if err != nil {
		return watcher.IssueSnapshot{}, err
	}
	return c.ensureTransitionComments(ctx, current, comments)
}

func (c *watcherClient) ensureArchiveComments(ctx context.Context, initial watcher.IssueSnapshot, archives []watcher.ArchiveComment) (watcher.IssueSnapshot, error) {
	current := initial
	for _, archive := range archives {
		var err error
		current, err = c.ensureMarkedComment(ctx, current, archive.Marker, archive.Body)
		if err != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("preserve overflow history: %w", err)
		}
	}
	return current, nil
}

func (c *watcherClient) ensureTransitionComments(ctx context.Context, initial watcher.IssueSnapshot, comments []watcher.ManagedComment) (watcher.IssueSnapshot, error) {
	current := initial
	for _, comment := range comments {
		var err error
		current, err = c.ensureMarkedComment(ctx, current, comment.Marker, comment.Body)
		if err != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("record status transition: %w", err)
		}
	}
	return current, nil
}

func (c *watcherClient) ensureMarkedComment(ctx context.Context, expected watcher.IssueSnapshot, marker, body string) (watcher.IssueSnapshot, error) {
	current, err := c.getIssueSnapshot(ctx, expected.Number)
	if err != nil {
		return watcher.IssueSnapshot{}, fmt.Errorf("preflight comment on issue #%d: %w", expected.Number, err)
	}
	if !sameIssueSnapshot(expected, current) {
		return watcher.IssueSnapshot{}, fmt.Errorf("issue #%d changed before comment write; refusing to overwrite a concurrent edit", expected.Number)
	}
	found, err := findMarkedComment(current.Comments, marker, body, c.appLogin)
	if err != nil {
		return watcher.IssueSnapshot{}, err
	}
	if found {
		return current, nil
	}
	commentBody := body
	_, _, writeErr := c.client.Issues.CreateComment(ctx, c.repo.Owner, c.repo.Name, expected.Number, &gh.IssueComment{Body: &commentBody})
	readback, readErr := c.getIssueSnapshot(ctx, expected.Number)
	if readErr != nil {
		if writeErr != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("comment outcome is uncertain (%v), and readback failed: %w", writeErr, readErr)
		}
		return watcher.IssueSnapshot{}, fmt.Errorf("read back marked comment: %w", readErr)
	}
	if readback.State != expected.State || readback.Title != expected.Title || readback.Body != expected.Body || !sameLabels(readback.Labels, expected.Labels) {
		return watcher.IssueSnapshot{}, fmt.Errorf("issue #%d changed during comment write", expected.Number)
	}
	found, markerErr := findMarkedComment(readback.Comments, marker, body, c.appLogin)
	if markerErr != nil {
		return watcher.IssueSnapshot{}, markerErr
	}
	if !found {
		if writeErr != nil {
			return watcher.IssueSnapshot{}, fmt.Errorf("comment outcome is uncertain (%w); marker was not observed", writeErr)
		}
		return watcher.IssueSnapshot{}, errors.New("created comment did not appear in readback")
	}
	if !exactlyOneNewComment(expected.Comments, readback.Comments, marker) {
		return watcher.IssueSnapshot{}, fmt.Errorf("issue #%d changed during comment write", expected.Number)
	}
	return readback, nil
}

func exactlyOneNewComment(before, after []watcher.CommentSnapshot, marker string) bool {
	if len(after) != len(before)+1 {
		return false
	}
	var added *watcher.CommentSnapshot
	beforeByID := make(map[int64]watcher.CommentSnapshot, len(before))
	for _, comment := range before {
		beforeByID[comment.ID] = comment
	}
	for i := range after {
		if old, exists := beforeByID[after[i].ID]; exists {
			if old != after[i] {
				return false
			}
			delete(beforeByID, after[i].ID)
			continue
		}
		if added != nil || !strings.Contains(after[i].Body, marker) {
			return false
		}
		added = &after[i]
	}
	return len(beforeByID) == 0 && added != nil
}

func findMarkedComment(comments []watcher.CommentSnapshot, marker, body, appLogin string) (bool, error) {
	count := 0
	for _, comment := range comments {
		if comment.AuthorLogin != appLogin {
			continue
		}
		if !strings.Contains(comment.Body, marker) {
			continue
		}
		count++
		if comment.Body != body {
			return false, errors.New("existing watcher comment marker has conflicting content")
		}
	}
	if count > 1 {
		return false, errors.New("duplicate watcher comment markers exist")
	}
	return count == 1, nil
}

func applyLabelDelta(current, add, remove []string) []string {
	labels := make(map[string]bool, len(current)+len(add))
	for _, label := range current {
		labels[label] = true
	}
	for _, label := range remove {
		delete(labels, label)
	}
	for _, label := range add {
		labels[label] = true
	}
	result := make([]string, 0, len(labels))
	for label := range labels {
		result = append(result, label)
	}
	sort.Strings(result)
	return result
}

func labelDeltaToTarget(current, target []string) (add, remove []string) {
	currentSet := make(map[string]bool, len(current))
	targetSet := make(map[string]bool, len(target))
	for _, label := range current {
		currentSet[label] = true
	}
	for _, label := range target {
		targetSet[label] = true
	}
	for _, label := range current {
		if !targetSet[label] {
			remove = append(remove, label)
		}
	}
	for _, label := range target {
		if !currentSet[label] {
			add = append(add, label)
		}
	}
	sort.Strings(add)
	sort.Strings(remove)
	return add, remove
}

func sameIssueSnapshot(a, b watcher.IssueSnapshot) bool {
	return a.Number == b.Number && a.State == b.State && sameOptionalTime(a.ClosedAt, b.ClosedAt) &&
		a.AuthorLogin == b.AuthorLogin && a.Title == b.Title && a.Body == b.Body && sameLabels(a.Labels, b.Labels) && sameComments(a.Comments, b.Comments)
}

func sameSnapshotSets(a, b []watcher.IssueSnapshot) bool {
	if len(a) != len(b) {
		return false
	}
	left := append([]watcher.IssueSnapshot(nil), a...)
	right := append([]watcher.IssueSnapshot(nil), b...)
	sort.Slice(left, func(i, j int) bool { return left[i].Number < left[j].Number })
	sort.Slice(right, func(i, j int) bool { return right[i].Number < right[j].Number })
	for i := range left {
		if !sameIssueSnapshot(left[i], right[i]) {
			return false
		}
	}
	return true
}

func sameOptionalTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

func sameLabels(a, b []string) bool {
	left, right := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(left)
	sort.Strings(right)
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	return reflect.DeepEqual(left, right)
}

func sameComments(a, b []watcher.CommentSnapshot) bool {
	left, right := append([]watcher.CommentSnapshot(nil), a...), append([]watcher.CommentSnapshot(nil), b...)
	sort.Slice(left, func(i, j int) bool { return left[i].ID < left[j].ID })
	sort.Slice(right, func(i, j int) bool { return right[i].ID < right[j].ID })
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	return reflect.DeepEqual(left, right)
}

func commentIDs(comments []watcher.CommentSnapshot) []int64 {
	ids := make([]int64, 0, len(comments))
	for _, comment := range comments {
		ids = append(ids, comment.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func findIssue(issues []watcher.IssueSnapshot, number int) (watcher.IssueSnapshot, bool) {
	for _, issue := range issues {
		if issue.Number == number {
			return issue, true
		}
	}
	return watcher.IssueSnapshot{}, false
}
