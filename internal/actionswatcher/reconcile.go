package actionswatcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

var monitoredWorkflowNames = map[string]struct{}{
	"CI":                          {},
	"CodeQL Security":             {},
	"Go vulnerability monitoring": {},
	"Go toolchain remediation":    {},
	"Release Please":              {},
	"Contributor showcase":        {},
	"Trusted Live Integration":    {},
}

// IsMonitoredWorkflow reports whether a workflow is part of the fixed
// repository-level watcher allowlist.
func IsMonitoredWorkflow(name string) bool {
	_, ok := monitoredWorkflowNames[name]
	return ok
}

type workflowKey struct {
	id   int64
	name string
}

// Reconcile derives issue changes from authoritative snapshots. It performs
// no API calls and returns no change for duplicate or irrelevant deliveries.
func Reconcile(repo RepositoryIdentity, appLogin string, attempts []RunAttempt, issues []IssueSnapshot, now time.Time) (Plan, error) {
	if err := validateReconcileInput(repo, appLogin, now); err != nil {
		return Plan{}, err
	}
	eligible, err := normalizeEligibleAttempts(attempts)
	if err != nil {
		return Plan{}, err
	}
	byWorkflow, namesByID, idsByName, err := groupAttempts(eligible)
	if err != nil {
		return Plan{}, err
	}
	managedIssues, err := indexManagedIssues(repo, appLogin, issues)
	if err != nil {
		return Plan{}, err
	}
	if err := validateWorkflowIdentityLinks(namesByID, idsByName, managedIssues); err != nil {
		return Plan{}, err
	}
	keys := workflowKeys(byWorkflow, managedIssues)
	plan := Plan{Changes: make([]IssueChange, 0)}
	for _, key := range keys {
		change, err := reconcileWorkflow(repo, key, byWorkflow[key], managedIssues[key])
		if err != nil {
			return Plan{}, fmt.Errorf("workflow %q: %w", key.name, err)
		}
		if change.Action != "" && change.Action != ActionNoop {
			plan.Changes = append(plan.Changes, change)
		}
	}
	return plan, nil
}

func validateReconcileInput(repo RepositoryIdentity, appLogin string, now time.Time) error {
	if repo.ID <= 0 || repo.Owner == "" || repo.Name == "" || strings.Contains(repo.Owner+repo.Name, "\n") {
		return errors.New("repository identity is incomplete")
	}
	if !strings.HasSuffix(appLogin, "[bot]") || strings.TrimSpace(appLogin) != appLogin {
		return errors.New("expected GitHub App bot login is invalid")
	}
	if now.IsZero() {
		return errors.New("reconciliation time is required")
	}
	return nil
}

func groupAttempts(eligible []RunAttempt) (map[workflowKey][]RunAttempt, map[int64]string, map[string]int64, error) {
	byWorkflow := make(map[workflowKey][]RunAttempt)
	namesByID := make(map[int64]string)
	idsByName := make(map[string]int64)
	for _, attempt := range eligible {
		if previous, ok := namesByID[attempt.WorkflowID]; ok && previous != attempt.WorkflowName {
			return nil, nil, nil, fmt.Errorf("workflow ID %d is associated with conflicting names %q and %q", attempt.WorkflowID, previous, attempt.WorkflowName)
		}
		if previous, ok := idsByName[attempt.WorkflowName]; ok && previous != attempt.WorkflowID {
			return nil, nil, nil, fmt.Errorf("workflow %q is associated with conflicting IDs %d and %d", attempt.WorkflowName, previous, attempt.WorkflowID)
		}
		namesByID[attempt.WorkflowID] = attempt.WorkflowName
		idsByName[attempt.WorkflowName] = attempt.WorkflowID
		key := workflowKey{id: attempt.WorkflowID, name: attempt.WorkflowName}
		byWorkflow[key] = append(byWorkflow[key], attempt)
	}
	return byWorkflow, namesByID, idsByName, nil
}

func validateWorkflowIdentityLinks(namesByID map[int64]string, idsByName map[string]int64, managedIssues map[workflowKey][]IssueSnapshot) error {
	for key := range managedIssues {
		if name, ok := namesByID[key.id]; ok && name != key.name {
			return fmt.Errorf("workflow ID %d has conflicting issue and run identities", key.id)
		}
		if id, ok := idsByName[key.name]; ok && id != key.id {
			return fmt.Errorf("workflow %q has conflicting issue and run identities", key.name)
		}
		namesByID[key.id] = key.name
		idsByName[key.name] = key.id
	}
	return nil
}

func workflowKeys(byWorkflow map[workflowKey][]RunAttempt, managedIssues map[workflowKey][]IssueSnapshot) []workflowKey {
	keys := make([]workflowKey, 0, len(byWorkflow)+len(managedIssues))
	seen := make(map[workflowKey]bool)
	for key := range byWorkflow {
		keys = append(keys, key)
		seen[key] = true
	}
	for key := range managedIssues {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].name != keys[j].name {
			return keys[i].name < keys[j].name
		}
		return keys[i].id < keys[j].id
	})
	return keys
}

func reconcileWorkflow(repo RepositoryIdentity, key workflowKey, source []RunAttempt, snapshots []IssueSnapshot) (IssueChange, error) {
	open, closed, err := classifyIssues(snapshots)
	if err != nil {
		return IssueChange{}, err
	}
	if open != nil {
		return updateOpenIncident(repo, key, source, *open)
	}
	if len(closed) > 0 {
		return createPostClosureIncident(repo, key, source, closed[len(closed)-1])
	}
	return createFirstIncident(repo, key, source)
}

func normalizeEligibleAttempts(attempts []RunAttempt) ([]RunAttempt, error) {
	byKey := make(map[string]RunAttempt)
	for _, original := range attempts {
		if _, monitored := monitoredWorkflowNames[original.WorkflowName]; !monitored {
			continue
		}
		if original.HeadBranch != "main" || (original.Event != "push" && original.Event != "schedule" && original.Event != "workflow_dispatch") {
			continue
		}
		if !isFailure(original.Conclusion) && original.Conclusion != "success" {
			continue
		}
		attempt := cloneAttempt(original)
		if attempt.WorkflowID <= 0 || attempt.RunID <= 0 || attempt.RunNumber <= 0 || attempt.Attempt <= 0 {
			return nil, errors.New("eligible run attempt is missing stable workflow or attempt identity")
		}
		if attempt.StartedAt.IsZero() || attempt.CompletedAt.IsZero() || attempt.CompletedAt.Before(attempt.StartedAt) {
			return nil, fmt.Errorf("eligible run attempt %d/%d has invalid timestamps", attempt.RunID, attempt.Attempt)
		}
		if attempt.HeadSHA == "" {
			return nil, fmt.Errorf("eligible run attempt %d/%d is missing its commit SHA", attempt.RunID, attempt.Attempt)
		}
		if !validHTTPSURL(attempt.HTMLURL) {
			return nil, fmt.Errorf("eligible run attempt %d/%d is missing a trusted HTTPS run URL", attempt.RunID, attempt.Attempt)
		}
		key := attemptKey(attempt)
		if previous, ok := byKey[key]; ok {
			if !sameAttempt(previous, attempt) {
				return nil, fmt.Errorf("run attempt %s has conflicting duplicate metadata", key)
			}
			continue
		}
		byKey[key] = attempt
	}
	result := make([]RunAttempt, 0, len(byKey))
	for _, attempt := range byKey {
		result = append(result, attempt)
	}
	sort.Slice(result, func(i, j int) bool { return attemptLess(result[i], result[j]) })
	return result, nil
}

func indexManagedIssues(repo RepositoryIdentity, appLogin string, issues []IssueSnapshot) (map[workflowKey][]IssueSnapshot, error) {
	managed := make(map[workflowKey][]IssueSnapshot)
	seenNumbers := make(map[int]bool)
	for _, issue := range issues {
		if issue.Number <= 0 {
			return nil, errors.New("issue snapshot has an invalid issue number")
		}
		if seenNumbers[issue.Number] {
			return nil, fmt.Errorf("issue #%d appears more than once in the issue snapshot", issue.Number)
		}
		seenNumbers[issue.Number] = true
		if !strings.Contains(issue.Body, "<!-- actions-watcher:v1 ") {
			continue
		}
		workflowID, workflowName, err := managedIssueIdentity(issue.Body, repo)
		if err != nil {
			return nil, fmt.Errorf("issue #%d has an invalid watcher identity: %w", issue.Number, err)
		}
		if _, ok := monitoredWorkflowNames[workflowName]; !ok {
			return nil, fmt.Errorf("issue #%d identifies an unmonitored workflow %q", issue.Number, workflowName)
		}
		if issue.AuthorLogin != appLogin {
			return nil, fmt.Errorf("issue #%d is marked as managed but was not authored by %q", issue.Number, appLogin)
		}
		if issue.State != "open" && issue.State != "closed" {
			return nil, fmt.Errorf("issue #%d has unknown state %q", issue.Number, issue.State)
		}
		if issue.State == "closed" && issue.ClosedAt == nil {
			return nil, fmt.Errorf("closed managed issue #%d is missing its closure timestamp", issue.Number)
		}
		if containsLabel(issue.Labels, FailingLabel) && containsLabel(issue.Labels, RecoveredLabel) {
			return nil, fmt.Errorf("issue #%d has conflicting watcher status labels", issue.Number)
		}
		if _, err := RenderBodyState(issue.Body, repo, workflowID, workflowName); err != nil {
			return nil, fmt.Errorf("issue #%d has invalid managed body: %w", issue.Number, err)
		}
		key := workflowKey{id: workflowID, name: workflowName}
		for _, comment := range issue.Comments {
			if strings.Contains(comment.Body, archiveMarkerPrefix) || strings.Contains(comment.Body, transitionMarkerPrefix) {
				if comment.AuthorLogin != appLogin {
					return nil, fmt.Errorf("issue #%d has a watcher marker written by an unexpected author", issue.Number)
				}
				if strings.Contains(comment.Body, transitionMarkerPrefix) {
					if _, err := parseTransitionMarker(comment.Body); err != nil {
						return nil, fmt.Errorf("issue #%d has an invalid transition comment: %w", issue.Number, err)
					}
				}
			}
		}
		if _, _, err := parseArchives(issue.Comments, workflowID); err != nil {
			return nil, fmt.Errorf("issue #%d has invalid archive comments: %w", issue.Number, err)
		}
		managed[key] = append(managed[key], issue)
	}
	return managed, nil
}

func classifyIssues(issues []IssueSnapshot) (*IssueSnapshot, []IssueSnapshot, error) {
	var open *IssueSnapshot
	closed := make([]IssueSnapshot, 0)
	for i := range issues {
		if issues[i].State == "open" {
			if open != nil {
				return nil, nil, errors.New("multiple open watcher-managed incidents exist")
			}
			openSnapshot := issues[i]
			open = &openSnapshot
		} else {
			closed = append(closed, issues[i])
		}
	}
	sort.Slice(closed, func(i, j int) bool {
		if closed[i].ClosedAt.Equal(*closed[j].ClosedAt) {
			return closed[i].Number < closed[j].Number
		}
		return closed[i].ClosedAt.Before(*closed[j].ClosedAt)
	})
	return open, closed, nil
}

func createFirstIncident(repo RepositoryIdentity, key workflowKey, source []RunAttempt) (IssueChange, error) {
	return createIncident(repo, key, source, 0)
}

func createPostClosureIncident(repo RepositoryIdentity, key workflowKey, source []RunAttempt, closed IssueSnapshot) (IssueChange, error) {
	if closed.ClosedAt == nil {
		return IssueChange{}, errors.New("previous issue is missing its closure timestamp")
	}
	postClosure := make([]RunAttempt, 0, len(source))
	for _, attempt := range source {
		if attempt.StartedAt.After(*closed.ClosedAt) {
			postClosure = append(postClosure, attempt)
		}
	}
	return createIncident(repo, key, postClosure, closed.Number)
}

func createIncident(repo RepositoryIdentity, key workflowKey, attempts []RunAttempt, previousIssue int) (IssueChange, error) {
	failures := make([]RunAttempt, 0)
	for _, attempt := range attempts {
		if isFailure(attempt.Conclusion) {
			failures = append(failures, attempt)
		}
	}
	if len(failures) == 0 {
		return IssueChange{}, nil
	}
	state, err := stateFromAttempts(key, nil, attempts)
	if err != nil {
		return IssueChange{}, err
	}
	if state.PreviousIssueNumber == 0 {
		state.PreviousIssueNumber = previousIssue
	}
	body, archives, err := RenderBody(repo, state, "", nil)
	if err != nil {
		return IssueChange{}, err
	}
	change := IssueChange{
		Action: ActionCreate, WorkflowID: key.id, WorkflowName: key.name,
		PreviousIssueNumber: previousIssue, Title: incidentTitle(key.name), Body: body,
		CurrentStatus: state.Status, FailureCount: len(state.Failures),
		AddLabels: []string{ManagedLabel, statusLabel(state.Status)}, Archives: archives,
	}
	if len(attempts) > 0 {
		firstFailure := failures[0]
		initial := IncidentState{WorkflowID: key.id, WorkflowName: key.name, Status: StatusFailing, Failures: []RunAttempt{firstFailure}, Latest: firstFailure}
		initialBody, _, err := RenderBody(repo, initial, "", nil)
		if err != nil {
			return IssueChange{}, err
		}
		initialLabels := []string{ManagedLabel, FailingLabel}
		if initialBody != body || state.Status != StatusFailing {
			change.InitialBody = initialBody
			change.InitialLabels = initialLabels
		}
		change.Comments = transitionComments(initial, state, attempts, nil)
	}
	return change, nil
}

func updateOpenIncident(repo RepositoryIdentity, key workflowKey, source []RunAttempt, issue IssueSnapshot) (IssueChange, error) {
	previous, err := RenderBodyState(issue.Body, repo, key.id, key.name)
	if err != nil {
		return IssueChange{}, err
	}
	previous.Status = normalizeIssueStatus(previous.Status, issue.Labels)
	oldState := previous
	oldFailures := append([]RunAttempt(nil), previous.Failures...)
	archives, _, err := parseArchives(issue.Comments, key.id)
	if err != nil {
		return IssueChange{}, err
	}
	archivedFailures := make([]RunAttempt, 0)
	for _, archive := range archives {
		attempts, err := parseFailureMarkers(archive.Body)
		if err != nil {
			return IssueChange{}, err
		}
		for _, attempt := range attempts {
			if attempt.WorkflowID != key.id || attempt.WorkflowName != key.name || !validPersistedRunAttempt(attempt) {
				return IssueChange{}, errors.New("archive comment contains a mismatched failure attempt")
			}
		}
		archivedFailures = append(archivedFailures, attempts...)
	}
	previous.Failures, err = mergeFailures(previous.Failures, archivedFailures)
	if err != nil {
		return IssueChange{}, err
	}
	state, err := stateFromAttempts(key, &previous, source)
	if err != nil {
		return IssueChange{}, err
	}
	if state.Latest.WorkflowID == 0 {
		state.Latest = previous.Latest
		state.Status = previous.Status
		state.Recovery = previous.Recovery
	}
	state.Failures, err = mergeFailures(previous.Failures, state.Failures)
	if err != nil {
		return IssueChange{}, err
	}
	if len(state.Failures) == 0 {
		state.Failures = oldFailures
	}
	body, newArchives, err := RenderBody(repo, state, issue.Body, issue.Comments)
	if err != nil {
		return IssueChange{}, err
	}
	addLabels, removeLabels := desiredLabelDelta(issue.Labels, state.Status)
	comments := transitionComments(oldState, state, source, issue.Comments)
	change := IssueChange{
		Action: ActionNoop, WorkflowID: key.id, WorkflowName: key.name, IssueNumber: issue.Number,
		Title: incidentTitle(key.name), Body: body, CurrentStatus: state.Status,
		FailureCount: len(state.Failures), AddLabels: addLabels, RemoveLabels: removeLabels,
		Comments: comments, Archives: newArchives,
	}
	if body != issue.Body || len(addLabels) != 0 || len(removeLabels) != 0 || len(comments) != 0 || len(newArchives) != 0 {
		change.Action = ActionUpdate
	}
	return change, nil
}

func stateFromAttempts(key workflowKey, previous *IncidentState, attempts []RunAttempt) (IncidentState, error) {
	state := IncidentState{WorkflowID: key.id, WorkflowName: key.name}
	var oldLatest *RunAttempt
	if previous != nil {
		state = *previous
		state.Failures = append([]RunAttempt(nil), previous.Failures...)
		oldLatest = cloneAttemptPointer(&previous.Latest)
	}
	newFailures := make([]RunAttempt, 0)
	for _, attempt := range attempts {
		if isFailure(attempt.Conclusion) {
			newFailures = append(newFailures, attempt)
		}
	}
	merged, err := mergeFailures(state.Failures, newFailures)
	if err != nil {
		return IncidentState{}, err
	}
	state.Failures = merged
	if oldLatest == nil || oldLatest.WorkflowID == 0 {
		state.Status = StatusFailing
		state.Latest = RunAttempt{}
	}
	ordered := append([]RunAttempt(nil), attempts...)
	sort.Slice(ordered, func(i, j int) bool { return attemptLess(ordered[i], ordered[j]) })
	for _, attempt := range ordered {
		if oldLatest != nil && oldLatest.WorkflowID != 0 && !attemptLess(*oldLatest, attempt) {
			continue
		}
		if attempt.Conclusion == "success" {
			latestFailure, ok := latestFailureAt(state.Failures, attempt.StartedAt)
			if ok && attempt.StartedAt.After(latestFailure.StartedAt) {
				if state.Status != StatusRecovered {
					state.Recovery = cloneAttemptPointer(&attempt)
				}
				state.Status = StatusRecovered
				state.Recovery = cloneAttemptPointer(&attempt)
				state.Latest = attempt
			}
			continue
		}
		if isFailure(attempt.Conclusion) {
			state.Status = StatusFailing
			state.Recovery = nil
			state.Latest = attempt
		}
	}
	if state.Latest.WorkflowID == 0 {
		if previous != nil {
			state.Latest, state.Status, state.Recovery = previous.Latest, previous.Status, previous.Recovery
		} else if len(state.Failures) > 0 {
			state.Latest = state.Failures[len(state.Failures)-1]
			state.Status = StatusFailing
		}
	}
	if state.Status == "" && len(state.Failures) > 0 {
		state.Status = StatusFailing
	}
	return state, nil
}

func transitionComments(previous, next IncidentState, incoming []RunAttempt, existing []CommentSnapshot) []ManagedComment {
	if previous.Latest.WorkflowID == 0 {
		return nil
	}
	ordered := append([]RunAttempt(nil), incoming...)
	sort.Slice(ordered, func(i, j int) bool { return attemptLess(ordered[i], ordered[j]) })
	status := previous.Status
	comments := make([]ManagedComment, 0)
	known := make(map[string]bool)
	for _, comment := range existing {
		if strings.Contains(comment.Body, transitionMarkerPrefix) {
			value, err := parseTransitionMarker(comment.Body)
			if err == nil {
				known[value] = true
			}
		}
	}
	for _, attempt := range ordered {
		if !attemptLess(previous.Latest, attempt) {
			continue
		}
		newStatus := status
		if isFailure(attempt.Conclusion) {
			newStatus = StatusFailing
		} else if attempt.Conclusion == "success" {
			failure, ok := latestFailureAt(next.Failures, attempt.StartedAt)
			if ok && attempt.StartedAt.After(failure.StartedAt) {
				newStatus = StatusRecovered
			}
		}
		if newStatus == status {
			continue
		}
		markerValue := transitionKey(newStatus, attempt)
		if !known[markerValue] {
			marker := transitionMarkerPrefix + markerValue + " -->"
			message := "Observed a new failure on " + renderRunLink(attempt) + ". The incident is failing again."
			if newStatus == StatusRecovered {
				message = "Observed a successful run after the latest failure: " + renderRunLink(attempt) + ". This records an observed recovery and does not prove the root cause is fixed."
			}
			comments = append(comments, ManagedComment{Marker: marker, Body: message + "\n\n" + marker})
			known[markerValue] = true
		}
		status = newStatus
	}
	return comments
}

func mergeFailures(groups ...[]RunAttempt) ([]RunAttempt, error) {
	byKey := make(map[string]RunAttempt)
	for _, group := range groups {
		for _, original := range group {
			attempt := cloneAttempt(original)
			if !isFailure(attempt.Conclusion) {
				return nil, fmt.Errorf("history includes non-failure attempt %s", attemptKey(attempt))
			}
			key := attemptKey(attempt)
			if previous, ok := byKey[key]; ok {
				if !sameAttempt(previous, attempt) {
					return nil, fmt.Errorf("history has conflicting duplicate attempt %s", key)
				}
				continue
			}
			byKey[key] = attempt
		}
	}
	result := make([]RunAttempt, 0, len(byKey))
	for _, attempt := range byKey {
		result = append(result, attempt)
	}
	sort.Slice(result, func(i, j int) bool { return attemptLess(result[i], result[j]) })
	return result, nil
}

func uniqueFailures(attempts []RunAttempt) ([]RunAttempt, error) {
	return mergeFailures(attempts)
}

func latestFailureAt(failures []RunAttempt, by time.Time) (RunAttempt, bool) {
	var latest RunAttempt
	found := false
	for _, attempt := range failures {
		if attempt.StartedAt.After(by) {
			continue
		}
		if !found || attemptLess(latest, attempt) {
			latest = attempt
			found = true
		}
	}
	return latest, found
}

func attemptLess(left, right RunAttempt) bool {
	if !left.StartedAt.Equal(right.StartedAt) {
		return left.StartedAt.Before(right.StartedAt)
	}
	if left.RunNumber != right.RunNumber {
		return left.RunNumber < right.RunNumber
	}
	if left.Attempt != right.Attempt {
		return left.Attempt < right.Attempt
	}
	return left.RunID < right.RunID
}

func attemptKey(attempt RunAttempt) string {
	return fmt.Sprintf("%d/%d/%d", attempt.WorkflowID, attempt.RunID, attempt.Attempt)
}

func isFailure(conclusion string) bool {
	return conclusion == "failure" || conclusion == "timed_out" || conclusion == "startup_failure"
}

func cloneAttempt(value RunAttempt) RunAttempt {
	value.FailedJobs = append([]string(nil), value.FailedJobs...)
	sort.Strings(value.FailedJobs)
	value.FailedJobs = uniqueStrings(value.FailedJobs)
	return value
}

func cloneAttemptPointer(value *RunAttempt) *RunAttempt {
	if value == nil {
		return nil
	}
	clone := cloneAttempt(*value)
	return &clone
}

func sameAttempt(left, right RunAttempt) bool {
	left, right = cloneAttempt(left), cloneAttempt(right)
	a, _ := json.Marshal(left)
	b, _ := json.Marshal(right)
	return string(a) == string(b)
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil
}

func statusLabel(status IncidentStatus) string {
	if status == StatusRecovered {
		return RecoveredLabel
	}
	return FailingLabel
}

func desiredLabelDelta(labels []string, status IncidentStatus) (add, remove []string) {
	want := statusLabel(status)
	other := FailingLabel
	if want == FailingLabel {
		other = RecoveredLabel
	}
	if !containsLabel(labels, ManagedLabel) {
		add = append(add, ManagedLabel)
	}
	if !containsLabel(labels, want) {
		add = append(add, want)
	}
	if containsLabel(labels, other) {
		remove = append(remove, other)
	}
	sort.Strings(add)
	sort.Strings(remove)
	return add, remove
}

func containsLabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}

func normalizeIssueStatus(status IncidentStatus, labels []string) IncidentStatus {
	if status == StatusFailing || status == StatusRecovered {
		return status
	}
	if containsLabel(labels, RecoveredLabel) {
		return StatusRecovered
	}
	return StatusFailing
}

func transitionKey(status IncidentStatus, attempt RunAttempt) string {
	value := string(status) + ":" + attemptKey(attempt)
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}
