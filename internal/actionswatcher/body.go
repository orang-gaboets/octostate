package actionswatcher

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	failureMarkerPrefix      = "<!-- actions-watcher:failure:v1 "
	stateMarkerPrefix        = "<!-- actions-watcher:state:v1 "
	archiveMarkerPrefix      = "<!-- actions-watcher:archive:v1 "
	archiveIndexPrefix       = "<!-- actions-watcher:archive-index:v2 "
	legacyArchiveIndexPrefix = "<!-- actions-watcher:archive-index:v1 "
	transitionMarkerPrefix   = "<!-- actions-watcher:transition:v1 "
	archiveChunkFailures     = 25
	archiveReferenceLimit    = 20
	// MaxPendingRunIDs allows a 1,000-run backlog and one newly delivered run.
	MaxPendingRunIDs = 1001
)

var identityMarkerPattern = regexp.MustCompile(`^<!-- actions-watcher:v1 repo_id=([0-9]+) workflow_id=([0-9]+) workflow_name=("(?:[^"\\]|\\.)*") -->$`)

type persistedState struct {
	Status              IncidentStatus `json:"status"`
	Latest              RunAttempt     `json:"latest"`
	Recovery            *RunAttempt    `json:"recovery,omitempty"`
	PreviousIssueNumber int            `json:"previous_issue_number,omitempty"`
	PendingRunIDs       []int64        `json:"pending_run_ids,omitempty"`
}

type archiveIndex struct {
	Version int
	Count   int
	Digest  string
	Hashes  []string
}

func identityMarker(repo RepositoryIdentity, workflowID int64, workflowName string) string {
	return fmt.Sprintf("<!-- actions-watcher:v1 repo_id=%d workflow_id=%d workflow_name=%s -->", repo.ID, workflowID, strconv.Quote(workflowName))
}

func incidentTitle(workflowName string) string {
	return "ci(actions): investigate " + workflowName + " workflow failures on main"
}

// RenderBody replaces only the marked managed block, preserving every byte of
// human-owned text around it. Overflow records are returned as immutable,
// machine-marked issue-comment chunks for the caller to create idempotently.
func RenderBody(repo RepositoryIdentity, state IncidentState, existing string, archiveComments []CommentSnapshot) (string, []ArchiveComment, error) {
	state, failures, err := validateIncidentState(repo, state)
	if err != nil {
		return "", nil, err
	}
	return renderBodyContent(repo, state, failures, existing, archiveComments)
}

// RenderBodyWithArchiveHistory merges failure records from persisted comments
// and planned archives into state before rendering. Planned archives may
// overlap persisted checkpoint archives when the checkpoint changes chunk
// boundaries; identical attempts are deduplicated, while conflicting records
// remain an error. Only archiveComments are considered already persisted.
func RenderBodyWithArchiveHistory(repo RepositoryIdentity, state IncidentState, existing string, archiveComments []CommentSnapshot, plannedArchives []ArchiveComment) (string, []ArchiveComment, error) {
	state, err := mergeArchivedFailureHistory(repo, state, archiveComments, plannedArchives)
	if err != nil {
		return "", nil, err
	}
	return RenderBody(repo, state, existing, archiveComments)
}

func mergeArchivedFailureHistory(repo RepositoryIdentity, state IncidentState, archiveComments []CommentSnapshot, plannedArchives []ArchiveComment) (IncidentState, error) {
	persisted, _, err := parseArchives(archiveComments, state.WorkflowID)
	if err != nil {
		return IncidentState{}, fmt.Errorf("parse archived failure history: %w", err)
	}
	failures := make([]RunAttempt, 0)
	for _, archive := range persisted {
		failures = append(failures, archive.Attempts...)
	}
	for _, planned := range plannedArchives {
		if len(planned.Attempts) == 0 {
			return IncidentState{}, errors.New("planned archive contains no failure attempts")
		}
		canonical, err := makeArchive(state.WorkflowID, planned.Attempts)
		if err != nil {
			return IncidentState{}, fmt.Errorf("validate planned archive: %w", err)
		}
		if canonical.Marker != planned.Marker || canonical.Body != planned.Body {
			return IncidentState{}, errors.New("planned archive marker or body does not match its failure attempts")
		}
		parsed, _, err := parseArchives([]CommentSnapshot{{Body: planned.Body}}, state.WorkflowID)
		if err != nil || len(parsed) != 1 {
			return IncidentState{}, errors.New("planned archive contains invalid failure records")
		}
		failures = append(failures, parsed[0].Attempts...)
	}
	for _, failure := range failures {
		if failure.WorkflowID != state.WorkflowID || failure.WorkflowName != state.WorkflowName || !validPersistedRunAttempt(repo, failure) {
			return IncidentState{}, errors.New("archive comment contains a mismatched failure attempt")
		}
	}
	state.Failures, err = mergeFailures(state.Failures, failures)
	if err != nil {
		return IncidentState{}, fmt.Errorf("reconstruct archived failure history: %w", err)
	}
	return state, nil
}

func validateIncidentState(repo RepositoryIdentity, state IncidentState) (IncidentState, []RunAttempt, error) {
	if repo.ID <= 0 || repo.Owner == "" || repo.Name == "" {
		return IncidentState{}, nil, errors.New("repository identity is incomplete")
	}
	if state.WorkflowID <= 0 || state.WorkflowName == "" {
		return IncidentState{}, nil, errors.New("workflow identity is incomplete")
	}
	if state.Status != StatusFailing && state.Status != StatusRecovered {
		return IncidentState{}, nil, fmt.Errorf("unsupported incident status %q", state.Status)
	}
	failures, err := uniqueFailures(state.Failures)
	if err != nil {
		return IncidentState{}, nil, err
	}
	sort.Slice(failures, func(i, j int) bool { return attemptLess(failures[i], failures[j]) })
	if len(failures) == 0 {
		return IncidentState{}, nil, errors.New("incident history must contain at least one failure")
	}
	state.Failures = failures
	state.PendingRunIDs, err = normalizePendingRunIDs(state.PendingRunIDs)
	if err != nil {
		return IncidentState{}, nil, err
	}
	state.Latest = cloneAttempt(state.Latest)
	if state.Latest.WorkflowID == 0 {
		state.Latest = failures[len(failures)-1]
	}
	state.Recovery = cloneAttemptPointer(state.Recovery)
	if err := validateLatestIncidentState(state, failures); err != nil {
		return IncidentState{}, nil, err
	}
	return state, failures, nil
}

func validateLatestIncidentState(state IncidentState, failures []RunAttempt) error {
	if state.Status == StatusRecovered && state.Recovery == nil {
		return errors.New("recovered incident is missing its recovery attempt")
	}
	if state.Latest.WorkflowID != state.WorkflowID || state.Latest.WorkflowName != state.WorkflowName {
		return errors.New("latest attempt does not match the incident workflow")
	}
	for _, failure := range failures {
		if attemptLess(state.Latest, failure) {
			return errors.New("failure history contains an attempt later than the latest status")
		}
	}
	if state.Latest.Conclusion != "success" && !isFailure(state.Latest.Conclusion) {
		return errors.New("latest attempt has an unsupported conclusion")
	}
	if state.Status == StatusFailing && !isFailure(state.Latest.Conclusion) {
		return errors.New("failing incident must have a latest failed attempt")
	}
	if state.Status == StatusRecovered && (state.Latest.Conclusion != "success" || state.Recovery == nil || attemptLess(state.Latest, *state.Recovery)) {
		return errors.New("recovered incident must have a successful recovery no later than its ordering cursor")
	}
	if state.Recovery != nil && (state.Recovery.WorkflowID != state.WorkflowID || state.Recovery.WorkflowName != state.WorkflowName || state.Recovery.Conclusion != "success") {
		return errors.New("recovery attempt does not match the incident workflow")
	}
	if state.Status == StatusRecovered && !attemptLess(failures[len(failures)-1], *state.Recovery) {
		return errors.New("recovery attempt does not follow the latest failure")
	}
	if state.Status == StatusFailing && state.Recovery != nil {
		return errors.New("failing incident cannot retain a recovery attempt")
	}
	return nil
}

func renderBodyContent(repo RepositoryIdentity, state IncidentState, failures []RunAttempt, existing string, archiveComments []CommentSnapshot) (string, []ArchiveComment, error) {
	oldArchives, archivedKeys, err := parseArchives(archiveComments, state.WorkflowID)
	if err != nil {
		return "", nil, err
	}
	visibleFailures := make([]RunAttempt, 0, len(failures))
	for _, failure := range failures {
		if !archivedKeys[attemptKey(failure)] {
			visibleFailures = append(visibleFailures, failure)
		}
	}
	body, newArchives, err := renderWithOverflow(repo, state, visibleFailures, existing, oldArchives)
	if err != nil {
		return "", nil, err
	}
	if len(body) > MaxIssueBodyBytes {
		return "", nil, fmt.Errorf("issue body still exceeds %d bytes after archiving failure history", MaxIssueBodyBytes)
	}
	availableArchives := make(map[string]bool, len(oldArchives)+len(newArchives))
	for _, archive := range oldArchives {
		availableArchives[archive.Marker] = true
	}
	for _, archive := range newArchives {
		availableArchives[archive.Marker] = true
	}
	references, err := bodyArchiveReferences(existing)
	if err != nil {
		return "", nil, err
	}
	for _, marker := range references {
		if !availableArchives[marker] {
			return "", nil, fmt.Errorf("issue body references missing overflow archive %q", marker)
		}
	}
	index, hasIndex, err := bodyArchiveIndex(existing)
	if err != nil {
		return "", nil, err
	}
	if !hasIndex && (len(references) > 0 || bodyHasArchiveSummary(existing)) {
		return "", nil, errors.New("managed issue has archive history but is missing its full-set index")
	}
	if hasIndex {
		if index.Version == 1 {
			availableHashes := make(map[string]bool, len(availableArchives))
			for marker := range availableArchives {
				hash := sha256.Sum256([]byte(marker))
				availableHashes[hex.EncodeToString(hash[:])] = true
			}
			for _, hash := range index.Hashes {
				if !availableHashes[hash] {
					return "", nil, fmt.Errorf("issue body archive index references missing overflow archive digest %q", hash)
				}
			}
		} else {
			if index.Count > len(oldArchives) {
				return "", nil, fmt.Errorf("issue body archive index references %d overflow archives but only %d are available", index.Count, len(oldArchives))
			}
			if got := archiveIndexDigest(oldArchives[:index.Count]); got != index.Digest {
				return "", nil, errors.New("issue body archive index does not match its preserved overflow archives")
			}
		}
	}
	return body, removeExistingArchives(newArchives, oldArchives), nil
}

func renderWithOverflow(repo RepositoryIdentity, state IncidentState, visibleFailures []RunAttempt, existing string, oldArchives []ArchiveComment) (string, []ArchiveComment, error) {
	newArchives := make([]ArchiveComment, 0)
	body, err := renderBodyOnce(repo, state, visibleFailures, existing, oldArchives)
	if err != nil {
		return "", nil, err
	}
	for len(body) > MaxIssueBodyBytes && len(visibleFailures) > 1 {
		count, err := archiveChunkSize(state.WorkflowID, visibleFailures)
		if err != nil {
			return "", nil, err
		}
		chunk := append([]RunAttempt(nil), visibleFailures[:count]...)
		visibleFailures = append([]RunAttempt(nil), visibleFailures[count:]...)
		archive, err := makeArchive(state.WorkflowID, chunk)
		if err != nil {
			return "", nil, err
		}
		newArchives = append(newArchives, archive)
		allArchives := append(append([]ArchiveComment(nil), oldArchives...), newArchives...)
		sortArchives(allArchives)
		body, err = renderBodyOnce(repo, state, visibleFailures, existing, allArchives)
		if err != nil {
			return "", nil, err
		}
	}
	return body, newArchives, nil
}

func renderBodyOnce(repo RepositoryIdentity, state IncidentState, failures []RunAttempt, existing string, archives []ArchiveComment) (string, error) {
	managed := renderManaged(repo, state, failures, archives)
	body, _, _, err := replaceManagedRegion(repo, state, existing, managed)
	return body, err
}

func archiveChunkSize(workflowID int64, failures []RunAttempt) (int, error) {
	count := archiveChunkFailures
	if count >= len(failures) {
		count = len(failures) - 1
	}
	for count > 0 {
		candidate, err := makeArchive(workflowID, failures[:count])
		if err != nil {
			return 0, err
		}
		if len(candidate.Body) <= MaxIssueBodyBytes {
			return count, nil
		}
		count /= 2
	}
	return 0, errors.New("one failure record exceeds the archive comment size limit")
}

func removeExistingArchives(newArchives, oldArchives []ArchiveComment) []ArchiveComment {
	result := make([]ArchiveComment, 0, len(newArchives))
	for _, archive := range newArchives {
		if !archiveMarkerExists(oldArchives, archive.Marker) {
			result = append(result, archive)
		}
	}
	return result
}

func replaceManagedRegion(repo RepositoryIdentity, state IncidentState, existing, managed string) (body, prefix, suffix string, err error) {
	if existing == "" {
		identityLine := identityMarker(repo, state.WorkflowID, state.WorkflowName)
		prefix = identityLine + "\n\n" + managed + "\n\n## Maintainer investigation\n\n_Add investigation notes here._"
		return prefix, "", "", nil
	}
	identityLines := make([]string, 0, 1)
	identityStarts := make([]int, 0, 1)
	lineStart := 0
	for _, line := range strings.Split(existing, "\n") {
		if strings.HasPrefix(line, "<!-- actions-watcher:v1 ") {
			identityLines = append(identityLines, line)
			identityStarts = append(identityStarts, lineStart)
		}
		lineStart += len(line) + 1
	}
	if len(identityLines) != 1 {
		return "", "", "", fmt.Errorf("managed issue must contain exactly one versioned identity marker; found %d", len(identityLines))
	}
	match := identityMarkerPattern.FindStringSubmatch(strings.TrimSpace(identityLines[0]))
	if match == nil {
		return "", "", "", errors.New("managed issue identity marker is malformed")
	}
	repositoryID, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return "", "", "", errors.New("managed issue repository identity is invalid")
	}
	workflowID, err := strconv.ParseInt(match[2], 10, 64)
	if err != nil {
		return "", "", "", errors.New("managed issue workflow identity is invalid")
	}
	workflowName, err := strconv.Unquote(match[3])
	if err != nil {
		return "", "", "", errors.New("managed issue workflow name is invalid")
	}
	if repositoryID != repo.ID || workflowID != state.WorkflowID || workflowName != state.WorkflowName {
		return "", "", "", errors.New("managed issue identity does not match the target repository and workflow")
	}
	if strings.Count(existing, managedStart) != 1 || strings.Count(existing, managedEnd) != 1 {
		return "", "", "", errors.New("managed issue must contain exactly one start and end marker")
	}
	start := strings.Index(existing, managedStart)
	end := strings.Index(existing, managedEnd)
	if start < 0 || end < start+len(managedStart) {
		return "", "", "", errors.New("managed issue markers are out of order")
	}
	identityStart := identityStarts[0]
	if identityStart >= start && identityStart < end+len(managedEnd) {
		return "", "", "", errors.New("managed issue identity marker must remain outside the replaceable body region")
	}
	prefix = existing[:start]
	suffix = existing[end+len(managedEnd):]
	return prefix + managed + suffix, prefix, suffix, nil
}

func renderManaged(repo RepositoryIdentity, state IncidentState, failures []RunAttempt, archives []ArchiveComment) string {
	var b strings.Builder
	allFailures := state.Failures
	if len(allFailures) == 0 {
		allFailures = failures
	}
	b.WriteString(managedStart)
	b.WriteString("\n## Current status\n\n")
	failureCount := len(allFailures)
	if failureCount == 0 {
		failureCount = len(failures)
	}
	fmt.Fprintf(&b, "- Repository: `%s/%s`\n", escapeInline(repo.Owner), escapeInline(repo.Name))
	fmt.Fprintf(&b, "- Workflow: **%s**\n", escapeInline(state.WorkflowName))
	b.WriteString("- Branch: `main`\n")
	fmt.Fprintf(&b, "- Latest observed relevant status: **%s**\n", strings.ToUpper(string(state.Status)))
	fmt.Fprintf(&b, "- First failure: %s\n", formatTime(allFailures[0].StartedAt))
	fmt.Fprintf(&b, "- Latest failure: %s\n", formatTime(allFailures[len(allFailures)-1].StartedAt))
	fmt.Fprintf(&b, "- Total distinct failed attempts: %d\n", failureCount)
	latestFailure := allFailures[len(allFailures)-1]
	fmt.Fprintf(&b, "- Latest failed run/attempt: %s\n", renderRunLink(latestFailure))
	fmt.Fprintf(&b, "- Commit SHA: `%s`\n", escapeInline(latestFailure.HeadSHA))
	fmt.Fprintf(&b, "- Failed jobs: %s\n", renderJobs(latestFailure))
	if state.Recovery != nil {
		fmt.Fprintf(&b, "- Latest observed recovery: %s at %s\n", renderRunLink(*state.Recovery), formatTime(state.Recovery.StartedAt))
	} else {
		b.WriteString("- Latest observed recovery: none\n")
	}
	if state.PreviousIssueNumber > 0 {
		fmt.Fprintf(&b, "- Previous closed incident: #%d\n", state.PreviousIssueNumber)
	}
	b.WriteString("- A successful run records an observed recovery; it does not prove the root cause is fixed.\n")
	b.WriteString(stateMarker(state))
	b.WriteString("\n\n## Failure history\n\n")
	archived := make(map[string]bool)
	firstArchiveReference := max(0, len(archives)-archiveReferenceLimit)
	if len(archives) > 0 {
		fmt.Fprintf(&b, "- Preserved overflow evidence in %d archive comments; %d references are listed below.\n", len(archives), len(archives)-firstArchiveReference)
		b.WriteString(archiveIndexMarker(archives))
		b.WriteByte('\n')
	}
	for _, archive := range archives {
		keys, _ := parseFailureMarkers(archive.Body)
		for _, attempt := range keys {
			archived[attemptKey(attempt)] = true
		}
	}
	for _, archive := range archives[firstArchiveReference:] {
		b.WriteString("- Preserved overflow evidence in archive ")
		b.WriteString("`" + archive.Marker + "`\n")
	}
	for _, attempt := range failures {
		if archived[attemptKey(attempt)] {
			continue
		}
		b.WriteString(renderFailureLine(attempt))
		b.WriteByte('\n')
	}
	b.WriteString(managedEnd)
	return b.String()
}

// RenderBodyState decodes and validates the persisted state in a managed body.
func RenderBodyState(body string, repo RepositoryIdentity, workflowID int64, workflowName string) (IncidentState, error) {
	managed, err := managedBody(body, repo, workflowID, workflowName)
	if err != nil {
		return IncidentState{}, err
	}
	state, err := parseStateMarker(managed)
	if err != nil {
		return IncidentState{}, err
	}
	failures, err := parseFailureMarkers(managed)
	if err != nil {
		return IncidentState{}, err
	}
	state.WorkflowID = workflowID
	state.WorkflowName = workflowName
	state.Failures = failures
	if err := validatePersistedState(state, repo, workflowID, workflowName, failures); err != nil {
		return IncidentState{}, err
	}
	return state, nil
}

// RenderManagedBodyState decodes a managed body after reading its workflow identity marker.
func RenderManagedBodyState(body string, repo RepositoryIdentity) (IncidentState, error) {
	workflowID, workflowName, err := managedIssueIdentity(body, repo)
	if err != nil {
		return IncidentState{}, err
	}
	if !IsMonitoredWorkflow(workflowName) {
		return IncidentState{}, fmt.Errorf("managed issue identifies an unmonitored workflow %q", workflowName)
	}
	return RenderBodyState(body, repo, workflowID, workflowName)
}

// RenderCheckpointBody stores pending run IDs and archives visible failure
// history when needed to keep the issue body within GitHub's size limit.
// archiveComments must contain only comments authored by the watcher App.
func RenderCheckpointBody(body string, repo RepositoryIdentity, runIDs []int64, archiveComments []CommentSnapshot) (string, []ArchiveComment, error) {
	state, err := RenderManagedBodyState(body, repo)
	if err != nil {
		return "", nil, err
	}
	state.PendingRunIDs, err = normalizePendingRunIDs(runIDs)
	if err != nil {
		return "", nil, err
	}
	return RenderBodyWithArchiveHistory(repo, state, body, archiveComments, nil)
}

func managedBody(body string, repo RepositoryIdentity, workflowID int64, workflowName string) (string, error) {
	gotWorkflowID, gotWorkflowName, err := managedIssueIdentity(body, repo)
	if err != nil {
		return "", err
	}
	if gotWorkflowID != workflowID || gotWorkflowName != workflowName {
		return "", errors.New("managed issue identity does not match the target repository and workflow")
	}
	if strings.Count(body, managedStart) != 1 || strings.Count(body, managedEnd) != 1 {
		return "", errors.New("managed issue has missing or duplicate body markers")
	}
	start := strings.Index(body, managedStart)
	end := strings.Index(body, managedEnd)
	if end < start+len(managedStart) {
		return "", errors.New("managed issue body markers are out of order")
	}
	return body[start+len(managedStart) : end], nil
}

func managedIssueIdentity(body string, repo RepositoryIdentity) (int64, string, error) {
	markerLine := ""
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "<!-- actions-watcher:v1 ") {
			if markerLine != "" {
				return 0, "", errors.New("managed issue has duplicate identity markers")
			}
			markerLine = line
		}
	}
	match := identityMarkerPattern.FindStringSubmatch(strings.TrimSpace(markerLine))
	if match == nil {
		return 0, "", errors.New("managed issue identity marker is missing or malformed")
	}
	gotRepositoryID, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return 0, "", errors.New("managed issue repository identity is invalid")
	}
	gotWorkflowID, err := strconv.ParseInt(match[2], 10, 64)
	if err != nil || gotWorkflowID <= 0 {
		return 0, "", errors.New("managed issue workflow identity is invalid")
	}
	gotWorkflowName, err := strconv.Unquote(match[3])
	if err != nil {
		return 0, "", errors.New("managed issue workflow name is invalid")
	}
	if gotRepositoryID != repo.ID || repo.ID <= 0 {
		return 0, "", errors.New("managed issue identity does not match the target repository")
	}
	return gotWorkflowID, gotWorkflowName, nil
}

func validatePersistedState(state IncidentState, repo RepositoryIdentity, workflowID int64, workflowName string, failures []RunAttempt) error {
	if len(failures) == 0 {
		return errors.New("managed failure history is empty")
	}
	for _, failure := range failures {
		if failure.WorkflowID != workflowID || failure.WorkflowName != workflowName || !validPersistedRunAttempt(repo, failure) {
			return errors.New("failure history contains an invalid or mismatched attempt")
		}
	}
	if state.Latest.WorkflowID != workflowID || state.Latest.WorkflowName != workflowName || !validPersistedRunAttempt(repo, state.Latest) {
		return errors.New("managed state contains an invalid or mismatched latest attempt")
	}
	latestFailureRecorded := false
	for _, failure := range failures {
		if attemptLess(state.Latest, failure) {
			return errors.New("failure history contains an attempt later than the latest status")
		}
		if sameAttempt(failure, state.Latest) {
			latestFailureRecorded = true
		}
	}
	if state.Status == StatusFailing && !isFailure(state.Latest.Conclusion) {
		return errors.New("managed failing status does not match its latest attempt")
	}
	if state.Status == StatusFailing && !latestFailureRecorded {
		return errors.New("managed failure history is missing the latest failing attempt")
	}
	if state.Status == StatusRecovered && (state.Latest.Conclusion != "success" || state.Recovery == nil || attemptLess(state.Latest, *state.Recovery)) {
		return errors.New("managed recovered status does not match its ordering cursor")
	}
	if state.Recovery != nil && (state.Recovery.WorkflowID != workflowID || state.Recovery.WorkflowName != workflowName || !validPersistedRunAttempt(repo, *state.Recovery) || state.Recovery.Conclusion != "success") {
		return errors.New("managed recovery attempt is invalid or mismatched")
	}
	if state.Status == StatusRecovered && !attemptLess(failures[len(failures)-1], *state.Recovery) {
		return errors.New("managed recovery does not follow the latest failure")
	}
	if state.PreviousIssueNumber < 0 || repo.ID <= 0 {
		return errors.New("managed state contains an invalid repository or previous issue identity")
	}
	if err := validatePendingRunIDs(state.PendingRunIDs); err != nil {
		return err
	}
	return nil
}

func validatePendingRunIDs(runIDs []int64) error {
	if len(runIDs) > MaxPendingRunIDs {
		return errors.New("managed state contains too many pending workflow runs")
	}
	for i, runID := range runIDs {
		if runID <= 0 || (i > 0 && runIDs[i-1] >= runID) {
			return errors.New("managed state contains invalid or duplicate pending workflow run IDs")
		}
	}
	return nil
}

func normalizePendingRunIDs(runIDs []int64) ([]int64, error) {
	ids := append([]int64(nil), runIDs...)
	for _, runID := range ids {
		if runID <= 0 {
			return nil, errors.New("pending workflow run IDs must be positive")
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	ids = slices.Compact(ids)
	if len(ids) > MaxPendingRunIDs {
		return nil, fmt.Errorf("pending workflow run IDs exceed the limit of %d", MaxPendingRunIDs)
	}
	return ids, nil
}

func validPersistedRunAttempt(repo RepositoryIdentity, attempt RunAttempt) bool {
	return attempt.WorkflowID > 0 && attempt.RunID > 0 && attempt.RunNumber > 0 && attempt.Attempt > 0 &&
		(attempt.Event == "push" || attempt.Event == "schedule" || attempt.Event == "workflow_dispatch") &&
		attempt.HeadBranch == "main" && !attempt.StartedAt.IsZero() && !attempt.CompletedAt.IsZero() &&
		!attempt.CompletedAt.Before(attempt.StartedAt) && attempt.HeadSHA != "" && validRunURL(repo, attempt)
}

func parseArchives(comments []CommentSnapshot, workflowID int64) ([]ArchiveComment, map[string]bool, error) {
	archives := make([]ArchiveComment, 0)
	keys := make(map[string]bool)
	for _, comment := range comments {
		if !strings.Contains(comment.Body, archiveMarkerPrefix) {
			continue
		}
		marker, err := firstMarkedLine(comment.Body, archiveMarkerPrefix)
		if err != nil {
			return nil, nil, err
		}
		fields := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(marker, archiveMarkerPrefix), " -->"))
		if len(fields) != 4 || fields[0] != fmt.Sprintf("workflow_id=%d", workflowID) || !strings.HasPrefix(fields[3], "sha256=") {
			return nil, nil, errors.New("archive comment marker is malformed or belongs to another workflow")
		}
		attempts, err := parseFailureMarkers(comment.Body)
		if err != nil || len(attempts) == 0 {
			return nil, nil, errors.New("archive comment contains invalid failure records")
		}
		encoded := encodedFailureLines(attempts)
		hash := sha256.Sum256([]byte(encoded))
		if fields[3] != "sha256="+hex.EncodeToString(hash[:]) {
			return nil, nil, errors.New("archive comment checksum does not match its failure records")
		}
		if fields[2] != fmt.Sprintf("count=%d", len(attempts)) {
			return nil, nil, errors.New("archive comment record count does not match its failure records")
		}
		wantRange := fmt.Sprintf("range=%s..%s", attemptKey(attempts[0]), attemptKey(attempts[len(attempts)-1]))
		if fields[1] != wantRange {
			return nil, nil, errors.New("archive comment range does not match its failure records")
		}
		archives = append(archives, ArchiveComment{CommentID: comment.ID, Marker: marker, Body: comment.Body, Attempts: attempts})
		for _, attempt := range attempts {
			key := attemptKey(attempt)
			if keys[key] {
				return nil, nil, errors.New("duplicate failure record in archive comments")
			}
			keys[key] = true
		}
	}
	sortArchives(archives)
	return archives, keys, nil
}

func sortArchives(archives []ArchiveComment) {
	// Keep posted archives before planned ones so a failed body write preserves the old index prefix.
	sort.Slice(archives, func(i, j int) bool {
		leftID, rightID := archives[i].CommentID, archives[j].CommentID
		if leftID != 0 && rightID != 0 && leftID != rightID {
			return leftID < rightID
		}
		if (leftID == 0) != (rightID == 0) {
			return leftID != 0
		}
		left, right := archives[i].Attempts, archives[j].Attempts
		if len(left) > 0 && len(right) > 0 && !left[0].StartedAt.Equal(right[0].StartedAt) {
			return attemptLess(left[0], right[0])
		}
		return archives[i].Marker < archives[j].Marker
	})
}

func parseTransitionMarker(body string) (string, error) {
	var found string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, transitionMarkerPrefix) {
			if found != "" || !strings.HasPrefix(line, transitionMarkerPrefix) || !strings.HasSuffix(line, " -->") {
				return "", errors.New("transition comment marker is missing, duplicate, or malformed")
			}
			value := strings.TrimSuffix(strings.TrimPrefix(line, transitionMarkerPrefix), " -->")
			if len(value) != 64 {
				return "", errors.New("transition comment marker has an invalid digest")
			}
			if _, err := hex.DecodeString(value); err != nil {
				return "", errors.New("transition comment marker has an invalid digest")
			}
			found = value
		}
	}
	if found == "" {
		return "", errors.New("transition comment marker is missing")
	}
	return found, nil
}

func makeArchive(workflowID int64, attempts []RunAttempt) (ArchiveComment, error) {
	if len(attempts) == 0 {
		return ArchiveComment{}, errors.New("cannot archive an empty failure chunk")
	}
	content := encodedFailureLines(attempts)
	hash := sha256.Sum256([]byte(content))
	marker := fmt.Sprintf("<!-- actions-watcher:archive:v1 workflow_id=%d range=%s..%s count=%d sha256=%s -->", workflowID, attemptKey(attempts[0]), attemptKey(attempts[len(attempts)-1]), len(attempts), hex.EncodeToString(hash[:]))
	archivedAttempts := make([]RunAttempt, len(attempts))
	for i, attempt := range attempts {
		archivedAttempts[i] = cloneAttempt(attempt)
	}
	return ArchiveComment{Marker: marker, Body: marker + "\n" + content, Attempts: archivedAttempts}, nil
}

func parseFailureMarkers(text string) ([]RunAttempt, error) {
	var attempts []RunAttempt
	seen := make(map[string]bool)
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, failureMarkerPrefix) {
			continue
		}
		if strings.Count(line, failureMarkerPrefix) != 1 || !strings.HasSuffix(line, " -->") {
			return nil, errors.New("failure marker is malformed")
		}
		start := strings.Index(line, failureMarkerPrefix)
		payload := line[start+len(failureMarkerPrefix) : len(line)-len(" -->")]
		decoded, err := base64.RawURLEncoding.DecodeString(payload)
		if err != nil {
			return nil, errors.New("failure marker encoding is invalid")
		}
		var attempt RunAttempt
		if err := json.Unmarshal(decoded, &attempt); err != nil {
			return nil, errors.New("failure marker payload is invalid")
		}
		if !isFailure(attempt.Conclusion) || attempt.WorkflowID <= 0 || attempt.RunID <= 0 || attempt.Attempt <= 0 {
			return nil, errors.New("failure marker contains an invalid attempt")
		}
		key := attemptKey(attempt)
		if seen[key] {
			return nil, errors.New("duplicate failure marker")
		}
		seen[key] = true
		attempts = append(attempts, attempt)
	}
	sort.Slice(attempts, func(i, j int) bool { return attemptLess(attempts[i], attempts[j]) })
	return attempts, nil
}

func parseStateMarker(text string) (IncidentState, error) {
	var value IncidentState
	lines := strings.Split(text, "\n")
	var found bool
	for _, line := range lines {
		if !strings.Contains(line, stateMarkerPrefix) {
			continue
		}
		if found || !strings.HasPrefix(line, stateMarkerPrefix) || !strings.HasSuffix(line, " -->") {
			return value, errors.New("managed body has duplicate state markers")
		}
		found = true
		start := strings.Index(line, stateMarkerPrefix)
		decoded, err := base64.RawURLEncoding.DecodeString(line[start+len(stateMarkerPrefix) : len(line)-len(" -->")])
		if err != nil {
			return value, errors.New("managed state marker encoding is invalid")
		}
		var saved persistedState
		if err := json.Unmarshal(decoded, &saved); err != nil {
			return value, errors.New("managed state marker payload is invalid")
		}
		value.Status, value.Latest, value.Recovery, value.PreviousIssueNumber = saved.Status, saved.Latest, saved.Recovery, saved.PreviousIssueNumber
		value.PendingRunIDs = saved.PendingRunIDs
	}
	if !found || (value.Status != StatusFailing && value.Status != StatusRecovered) {
		return value, errors.New("managed state marker is missing or invalid")
	}
	return value, nil
}

func stateMarker(state IncidentState) string {
	payload, _ := json.Marshal(persistedState{
		Status: state.Status, Latest: state.Latest, Recovery: state.Recovery,
		PreviousIssueNumber: state.PreviousIssueNumber, PendingRunIDs: state.PendingRunIDs,
	})
	return stateMarkerPrefix + base64.RawURLEncoding.EncodeToString(payload) + " -->"
}

func bodyArchiveReferences(body string) ([]string, error) {
	start := strings.Index(body, managedStart)
	if start < 0 {
		return nil, nil
	}
	contentStart := start + len(managedStart)
	end := strings.Index(body[contentStart:], managedEnd)
	if end < 0 {
		return nil, nil
	}
	managed := body[contentStart : contentStart+end]
	const prefix = "- Preserved overflow evidence in archive `"
	var markers []string
	seen := make(map[string]bool)
	for _, line := range strings.Split(managed, "\n") {
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		if !strings.HasSuffix(line, "`") {
			return nil, errors.New("managed body contains a malformed archive reference")
		}
		marker := strings.TrimSuffix(strings.TrimPrefix(line, prefix), "`")
		if !strings.HasPrefix(marker, archiveMarkerPrefix) || !strings.HasSuffix(marker, " -->") {
			return nil, errors.New("managed body contains a malformed archive marker")
		}
		if seen[marker] {
			return nil, errors.New("managed body contains a duplicate archive reference")
		}
		seen[marker] = true
		markers = append(markers, marker)
	}
	sort.Strings(markers)
	return markers, nil
}

func bodyHasArchiveSummary(body string) bool {
	start := strings.Index(body, managedStart)
	if start < 0 {
		return false
	}
	contentStart := start + len(managedStart)
	end := strings.Index(body[contentStart:], managedEnd)
	if end < 0 {
		return false
	}
	managed := body[contentStart : contentStart+end]
	for _, line := range strings.Split(managed, "\n") {
		if strings.HasPrefix(line, "- Preserved overflow evidence in ") && strings.Contains(line, " archive comments;") {
			return true
		}
	}
	return false
}

func archiveIndexMarker(archives []ArchiveComment) string {
	if len(archives) == 0 {
		return ""
	}
	return fmt.Sprintf("%s count=%d sha256=%s -->", archiveIndexPrefix, len(archives), archiveIndexDigest(archives))
}

func archiveIndexDigest(archives []ArchiveComment) string {
	hash := sha256.New()
	for _, archive := range archives {
		markerHash := sha256.Sum256([]byte(archive.Marker))
		_, _ = hash.Write(markerHash[:])
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func bodyArchiveIndex(body string) (archiveIndex, bool, error) {
	start := strings.Index(body, managedStart)
	if start < 0 {
		return archiveIndex{}, false, nil
	}
	contentStart := start + len(managedStart)
	end := strings.Index(body[contentStart:], managedEnd)
	if end < 0 {
		return archiveIndex{}, false, nil
	}
	managed := body[contentStart : contentStart+end]
	var result archiveIndex
	found := false
	for _, line := range strings.Split(managed, "\n") {
		prefix, version := "", 0
		if strings.Contains(line, archiveIndexPrefix) {
			prefix, version = archiveIndexPrefix, 2
		} else if strings.Contains(line, legacyArchiveIndexPrefix) {
			prefix, version = legacyArchiveIndexPrefix, 1
		} else {
			continue
		}
		if found || !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, " -->") {
			return archiveIndex{}, false, errors.New("managed body contains a malformed or duplicate archive index")
		}
		found = true
		fields := strings.Fields(strings.TrimSuffix(strings.TrimPrefix(line, prefix), " -->"))
		if len(fields) != 2 || !strings.HasPrefix(fields[0], "count=") {
			return archiveIndex{}, false, errors.New("managed body contains a malformed archive index")
		}
		var err error
		result.Count, err = strconv.Atoi(strings.TrimPrefix(fields[0], "count="))
		if err != nil || result.Count <= 0 || result.Count > MaxIssueBodyBytes {
			return archiveIndex{}, false, errors.New("managed body contains an invalid archive index count")
		}
		result.Version = version
		if version == 2 {
			if !strings.HasPrefix(fields[1], "sha256=") {
				return archiveIndex{}, false, errors.New("managed body contains an invalid archive index digest")
			}
			result.Digest = strings.TrimPrefix(fields[1], "sha256=")
			decoded, err := hex.DecodeString(result.Digest)
			if err != nil || len(decoded) != sha256.Size {
				return archiveIndex{}, false, errors.New("managed body contains an invalid archive index digest")
			}
			continue
		}
		if !strings.HasPrefix(fields[1], "hashes=") {
			return archiveIndex{}, false, errors.New("managed body contains a malformed archive index")
		}
		encoded := strings.TrimPrefix(fields[1], "hashes=")
		decoded, err := base64.RawStdEncoding.DecodeString(encoded)
		if err != nil || len(decoded) != result.Count*sha256.Size {
			return archiveIndex{}, false, errors.New("managed body contains an invalid archive index digest list")
		}
		seen := make(map[string]bool, result.Count)
		for i := 0; i < len(decoded); i += sha256.Size {
			hash := hex.EncodeToString(decoded[i : i+sha256.Size])
			if seen[hash] {
				return archiveIndex{}, false, errors.New("managed body contains a duplicate archive index digest")
			}
			seen[hash] = true
			result.Hashes = append(result.Hashes, hash)
		}
	}
	if !found {
		return archiveIndex{}, false, nil
	}
	return result, true, nil
}

func encodedFailureLines(attempts []RunAttempt) string {
	var b strings.Builder
	for _, attempt := range attempts {
		b.WriteString(renderFailureLine(attempt))
		b.WriteByte('\n')
	}
	return b.String()
}

func renderFailureLine(attempt RunAttempt) string {
	payload, _ := json.Marshal(attempt)
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	return fmt.Sprintf("- %s — %s — commit `%s` — run ID %d, attempt %d — failed jobs: %s %s%s -->", formatTime(attempt.StartedAt), renderRunLink(attempt), escapeInline(attempt.HeadSHA), attempt.RunID, attempt.Attempt, renderJobs(attempt), failureMarkerPrefix, encoded)
}

func renderRunLink(attempt RunAttempt) string {
	urlValue, err := url.Parse(attempt.HTMLURL)
	if err != nil || urlValue.Scheme != "https" || urlValue.Host == "" || urlValue.User != nil {
		return "run URL unavailable"
	}
	path := strings.TrimRight(urlValue.Path, "/")
	if index := strings.LastIndex(path, "/attempts/"); index >= 0 {
		priorAttempt := path[index+len("/attempts/"):]
		if priorAttempt != "" && !strings.Contains(priorAttempt, "/") {
			if _, err := strconv.Atoi(priorAttempt); err == nil {
				path = path[:index]
			}
		}
	}
	urlValue.Path = path + fmt.Sprintf("/attempts/%d", attempt.Attempt)
	urlValue.RawPath = ""
	label := fmt.Sprintf("run ID %d, attempt %d", attempt.RunID, attempt.Attempt)
	return "[" + label + "](" + strings.ReplaceAll(strings.ReplaceAll(urlValue.String(), "(", "%28"), ")", "%29") + ")"
}

func renderJobs(attempt RunAttempt) string {
	if !attempt.JobDataAvailable {
		return "unavailable"
	}
	jobs := append([]string(nil), attempt.FailedJobs...)
	sort.Strings(jobs)
	jobs = uniqueStrings(jobs)
	if len(jobs) == 0 {
		return "none reported"
	}
	for i := range jobs {
		jobs[i] = "`" + escapeInline(jobs[i]) + "`"
	}
	return strings.Join(jobs, ", ")
}

func escapeInline(value string) string {
	value = html.EscapeString(value)
	value = strings.ReplaceAll(value, "@", "&#64;")
	for _, char := range []string{"\\", "`", "*", "_", "[", "]", "<", ">"} {
		value = strings.ReplaceAll(value, char, "\\"+char)
	}
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r", " "), "\n", " ")
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "unavailable"
	}
	return value.UTC().Format(time.RFC3339)
}

func firstMarkedLine(body, prefix string) (string, error) {
	var found string
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, prefix) {
			if found != "" || !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, " -->") {
				return "", errors.New("archive marker is missing, duplicate, or malformed")
			}
			found = line
		}
	}
	if found == "" {
		return "", errors.New("archive marker is missing")
	}
	return found, nil
}

func archiveMarkerExists(archives []ArchiveComment, marker string) bool {
	for _, archive := range archives {
		if archive.Marker == marker {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}
