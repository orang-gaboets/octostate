// Package actionswatcher contains the deterministic incident model used by
// the GitHub Actions workflow watcher.
package actionswatcher

import (
	"time"
)

const (
	// ManagedLabel identifies incidents created and maintained by this watcher.
	ManagedLabel = "actions-watcher"
	// FailingLabel marks an incident whose latest eligible attempt failed.
	FailingLabel = "workflow: failing"
	// RecoveredLabel marks an incident with a later observed successful attempt.
	RecoveredLabel = "workflow: recovered"
	// MaxIssueBodyBytes keeps generated issue bodies below GitHub's API limit.
	MaxIssueBodyBytes = 60000
	managedStart      = "<!-- actions-watcher:managed:start -->"
	managedEnd        = "<!-- actions-watcher:managed:end -->"
)

// RepositoryIdentity is the stable GitHub identity for a repository.
type RepositoryIdentity struct {
	ID    int64
	Owner string
	Name  string
}

// RunAttempt is authoritative metadata for one workflow run attempt.
type RunAttempt struct {
	WorkflowID       int64
	WorkflowName     string
	RunID            int64
	RunNumber        int
	Attempt          int
	Event            string
	HeadBranch       string
	Conclusion       string
	HeadSHA          string
	HTMLURL          string
	StartedAt        time.Time
	CompletedAt      time.Time
	FailedJobs       []string
	JobDataAvailable bool
}

// CommentSnapshot is a GitHub issue comment used for idempotent reconciliation.
type CommentSnapshot struct {
	ID          int64
	AuthorLogin string
	Body        string
}

// IssueSnapshot is the issue state observed before planning changes.
type IssueSnapshot struct {
	Number      int
	State       string
	ClosedAt    *time.Time
	AuthorLogin string
	Title       string
	Body        string
	Labels      []string
	Comments    []CommentSnapshot
}

// IncidentStatus is the watcher-managed current state of an incident.
type IncidentStatus string

const (
	// StatusFailing means the latest relevant attempt is a failure.
	StatusFailing IncidentStatus = "failing"
	// StatusRecovered means a later successful attempt was observed.
	StatusRecovered IncidentStatus = "recovered"
)

// IssueAction describes the planned issue operation.
type IssueAction string

const (
	// ActionNoop means the snapshot already matches the desired incident state.
	ActionNoop IssueAction = "noop"
	// ActionCreate creates a new managed incident.
	ActionCreate IssueAction = "create"
	// ActionUpdate updates an existing open managed incident.
	ActionUpdate IssueAction = "update"
)

// IncidentState is the reconstructed state for one workflow incident.
type IncidentState struct {
	WorkflowID          int64
	WorkflowName        string
	Status              IncidentStatus
	PreviousIssueNumber int
	PendingRunIDs       []int64 // Delivered runs to re-fetch before clearing a partial-write checkpoint.
	Failures            []RunAttempt
	Latest              RunAttempt
	Recovery            *RunAttempt
}

// ManagedComment is an idempotent recovery or regression transition comment.
type ManagedComment struct {
	Marker string
	Body   string
}

// ArchiveComment preserves overflowed failure history in a marked comment.
type ArchiveComment struct {
	// CommentID is GitHub's creation-order ID; zero until the archive is posted.
	CommentID int64
	Marker    string
	Body      string
	Attempts  []RunAttempt
}

// IssueChange describes a single create or update in a reconciliation plan.
type IssueChange struct {
	Action              IssueAction
	WorkflowID          int64
	WorkflowName        string
	IssueNumber         int
	PreviousIssueNumber int
	Title               string
	Body                string
	InitialBody         string
	InitialLabels       []string
	CurrentStatus       IncidentStatus
	FailureCount        int
	AddLabels           []string
	RemoveLabels        []string
	Comments            []ManagedComment
	Archives            []ArchiveComment
	PendingRunIDs       []int64 // Transient run IDs that applyPlan must persist before side effects.
}

// Plan is the deterministic set of issue mutations derived from snapshots.
type Plan struct {
	Changes []IssueChange
}
