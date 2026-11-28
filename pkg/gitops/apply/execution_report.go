package apply

import (
	"context"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// ActionOutcomeStatus describes what is known about one planned action after an
// execution failure.
type ActionOutcomeStatus string

const (
	// ActionOutcomeStatusCompleted indicates all writes for the action returned
	// successfully.
	ActionOutcomeStatusCompleted ActionOutcomeStatus = "completed"
	// ActionOutcomeStatusFailedBeforeWrite indicates the action failed before a
	// mutation call began.
	ActionOutcomeStatusFailedBeforeWrite ActionOutcomeStatus = "failed_before_write"
	// ActionOutcomeStatusPartiallyCompleted indicates at least one write was
	// confirmed before the action failed.
	ActionOutcomeStatusPartiallyCompleted ActionOutcomeStatus = "partially_completed"
	// ActionOutcomeStatusOutcomeUnknown indicates the first mutation call
	// returned an error, so its remote outcome cannot be determined.
	ActionOutcomeStatusOutcomeUnknown ActionOutcomeStatus = "outcome_unknown"
	// ActionOutcomeStatusSkipped indicates the plan action was non-executable.
	ActionOutcomeStatusSkipped ActionOutcomeStatus = "skipped"
	// ActionOutcomeStatusNotAttempted indicates execution stopped before the
	// executable action was reached.
	ActionOutcomeStatusNotAttempted ActionOutcomeStatus = "not_attempted"
)

// ExecutionFailurePhase identifies a batch-level failure that is not
// attributable to one planned action.
type ExecutionFailurePhase string

const (
	// ExecutionFailurePhaseInviteUsernameResolution indicates that invitation
	// username lookups were interrupted before invitation writes began.
	ExecutionFailurePhaseInviteUsernameResolution ExecutionFailurePhase = "invite_username_resolution"
)

// ActionOutcome reports a safe summary of one action in the current execution
// plan.
// It deliberately excludes resource identifiers, messages, and field values.
type ActionOutcome struct {
	PlanIndex      int                           `json:"plan_index"`
	ResourceType   gitopsplan.ActionResourceType `json:"resource_type"`
	Operation      gitopsplan.ActionOperation    `json:"operation"`
	Status         ActionOutcomeStatus           `json:"status"`
	ConfirmedSteps []string                      `json:"confirmed_steps"`
	UncertainStep  string                        `json:"uncertain_step,omitempty"`
}

// ExecutionReport gives callers safe recovery information after execution
// fails. FailedActionIndex and ActionOutcome.PlanIndex are zero-based indexes
// into the current execution plan. FailedActionIndex is -1 when a failure is
// not attributable to one action; FailurePhase identifies that batch phase.
type ExecutionReport struct {
	FailedActionIndex int                   `json:"failed_action_index"`
	FailurePhase      ExecutionFailurePhase `json:"failure_phase,omitempty"`
	ActionOutcomes    []ActionOutcome       `json:"action_outcomes"`
}

// ExecutionError wraps the original action failure while exposing a
// redacted report for recovery decisions.
type ExecutionError struct {
	cause  error
	report ExecutionReport
}

// Error returns a stable message that does not reveal remote error details.
func (e *ExecutionError) Error() string {
	return "apply execution failed; inspect the execution report for recovery guidance"
}

// Unwrap returns the original failure for errors.Is and errors.As checks.
func (e *ExecutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Report returns a copy of the safe execution report.
func (e *ExecutionError) Report() ExecutionReport {
	if e == nil {
		return ExecutionReport{ActionOutcomes: []ActionOutcome{}}
	}
	return cloneExecutionReport(e.report)
}

type executionTracker struct {
	outcomes []ActionOutcome
	current  int
}

func newExecutionTracker(actions []gitopsplan.Action) *executionTracker {
	tracker := &executionTracker{
		outcomes: make([]ActionOutcome, len(actions)),
		current:  -1,
	}
	for index, action := range actions {
		status := ActionOutcomeStatusNotAttempted
		if !action.Executable {
			status = ActionOutcomeStatusSkipped
		}
		tracker.outcomes[index] = ActionOutcome{
			PlanIndex:      index,
			ResourceType:   safeResourceType(action.ResourceType),
			Operation:      safeActionOperation(action.Operation),
			Status:         status,
			ConfirmedSteps: []string{},
		}
	}
	return tracker
}

func (t *executionTracker) beginAction(index int) {
	if t == nil || index < 0 || index >= len(t.outcomes) {
		return
	}
	t.current = index
}

func (t *executionTracker) completeAction(index int) {
	if t == nil || index < 0 || index >= len(t.outcomes) {
		return
	}
	t.outcomes[index].Status = ActionOutcomeStatusCompleted
	t.current = -1
}

func (t *executionTracker) mutate(ctx context.Context, step string, call func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	index := -1
	if t != nil {
		index = t.current
	}
	err := call()
	if t == nil || index < 0 || index >= len(t.outcomes) {
		return err
	}
	outcome := &t.outcomes[index]
	if err != nil {
		outcome.UncertainStep = step
		return err
	}
	outcome.ConfirmedSteps = append(outcome.ConfirmedSteps, step)
	return nil
}

func (t *executionTracker) report(failedIndex int) ExecutionReport {
	if t == nil {
		return ExecutionReport{FailedActionIndex: failedIndex, ActionOutcomes: []ActionOutcome{}}
	}
	if failedIndex >= 0 && failedIndex < len(t.outcomes) {
		outcome := &t.outcomes[failedIndex]
		switch {
		case len(outcome.ConfirmedSteps) > 0:
			outcome.Status = ActionOutcomeStatusPartiallyCompleted
		case outcome.UncertainStep != "":
			outcome.Status = ActionOutcomeStatusOutcomeUnknown
		default:
			outcome.Status = ActionOutcomeStatusFailedBeforeWrite
		}
	}
	return cloneExecutionReport(ExecutionReport{FailedActionIndex: failedIndex, ActionOutcomes: t.outcomes})
}

func cloneExecutionReport(report ExecutionReport) ExecutionReport {
	cloned := ExecutionReport{
		FailedActionIndex: report.FailedActionIndex,
		FailurePhase:      report.FailurePhase,
		ActionOutcomes:    make([]ActionOutcome, len(report.ActionOutcomes)),
	}
	for index, outcome := range report.ActionOutcomes {
		cloned.ActionOutcomes[index] = outcome
		cloned.ActionOutcomes[index].ConfirmedSteps = append([]string{}, outcome.ConfirmedSteps...)
	}
	return cloned
}

func redactFailureResult(result *Result) *Result {
	if result == nil {
		return nil
	}
	redacted := *result
	redacted.Executed = redactActions(result.Executed)
	redacted.SkippedDrift = redactActions(result.SkippedDrift)
	redacted.UnresolvedTeamMemberships = []state.TeamMember{}
	redacted.Normalize()
	return &redacted
}

func redactActions(actions []gitopsplan.Action) []gitopsplan.Action {
	redacted := make([]gitopsplan.Action, len(actions))
	for index, action := range actions {
		redacted[index] = gitopsplan.Action{
			ResourceType: safeResourceType(action.ResourceType),
			Operation:    safeActionOperation(action.Operation),
			Executable:   action.Executable,
			Message:      "",
			ResourceID:   "",
			Changes:      []gitopsplan.FieldChange{},
		}
	}
	return redacted
}

func safeResourceType(resourceType gitopsplan.ActionResourceType) gitopsplan.ActionResourceType {
	switch resourceType {
	case gitopsplan.ActionResourceTypeRepository,
		gitopsplan.ActionResourceTypeTeam,
		gitopsplan.ActionResourceTypeOrganizationMember,
		gitopsplan.ActionResourceTypeInvite,
		gitopsplan.ActionResourceTypeTeamMember,
		gitopsplan.ActionResourceTypeTeamRepositoryPermission:
		return resourceType
	default:
		return "unknown"
	}
}

func safeActionOperation(operation gitopsplan.ActionOperation) gitopsplan.ActionOperation {
	switch operation {
	case gitopsplan.ActionOperationCreate,
		gitopsplan.ActionOperationUpdate,
		gitopsplan.ActionOperationDelete,
		gitopsplan.ActionOperationRemove:
		return operation
	default:
		return "unknown"
	}
}

func skippedActions(actions []gitopsplan.Action) []gitopsplan.Action {
	skipped := make([]gitopsplan.Action, 0, len(actions))
	for _, action := range actions {
		if !action.Executable {
			skipped = append(skipped, action)
		}
	}
	return skipped
}

func failedExecution(result *Result, tracker *executionTracker, failedIndex int, cause error) (*Result, error) {
	return failedExecutionWithPhase(result, tracker, failedIndex, "", cause)
}

func failedExecutionWithPhase(result *Result, tracker *executionTracker, failedIndex int, phase ExecutionFailurePhase, cause error) (*Result, error) {
	if tracker != nil && failedIndex >= 0 {
		tracker.beginAction(failedIndex)
	}
	report := tracker.report(failedIndex)
	report.FailurePhase = phase
	return redactFailureResult(result), &ExecutionError{cause: cause, report: report}
}

// These wrappers embed full service interfaces. Every mutating method used by
// apply must be explicitly overridden here because promoted methods bypass
// execution tracking.
type trackedRepositoryService struct {
	repos.Service
	tracker *executionTracker
}

func (s *trackedRepositoryService) CreateFromTemplate(ctx context.Context, templateOwner, templateRepo string, request *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	var repository *gh.Repository
	var response *gh.Response
	err := s.tracker.mutate(ctx, "create_repository", func() error {
		var callErr error
		repository, response, callErr = s.Service.CreateFromTemplate(ctx, templateOwner, templateRepo, request)
		return callErr
	})
	return repository, response, err
}

func (s *trackedRepositoryService) Create(ctx context.Context, owner string, repositoryRequest *gh.Repository) (*gh.Repository, *gh.Response, error) {
	var repository *gh.Repository
	var response *gh.Response
	err := s.tracker.mutate(ctx, "create_repository", func() error {
		var callErr error
		repository, response, callErr = s.Service.Create(ctx, owner, repositoryRequest)
		return callErr
	})
	return repository, response, err
}

func (s *trackedRepositoryService) Edit(ctx context.Context, owner, name string, repositoryRequest *gh.Repository) (*gh.Repository, *gh.Response, error) {
	var repository *gh.Repository
	var response *gh.Response
	err := s.tracker.mutate(ctx, "apply_repository_settings", func() error {
		var callErr error
		repository, response, callErr = s.Service.Edit(ctx, owner, name, repositoryRequest)
		return callErr
	})
	return repository, response, err
}

func (s *trackedRepositoryService) ReplaceAllTopics(ctx context.Context, owner, name string, topics []string) ([]string, *gh.Response, error) {
	var replacedTopics []string
	var response *gh.Response
	err := s.tracker.mutate(ctx, "replace_repository_topics", func() error {
		var callErr error
		replacedTopics, response, callErr = s.Service.ReplaceAllTopics(ctx, owner, name, topics)
		return callErr
	})
	return replacedTopics, response, err
}

type trackedOrganizationService struct {
	organizations.Service
	tracker *executionTracker
}

func (s *trackedOrganizationService) CreateOrgInvitation(ctx context.Context, org string, options *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	var invitation *gh.Invitation
	var response *gh.Response
	err := s.tracker.mutate(ctx, "create_invitation", func() error {
		var callErr error
		invitation, response, callErr = s.Service.CreateOrgInvitation(ctx, org, options)
		return callErr
	})
	return invitation, response, err
}

func (s *trackedOrganizationService) EditOrgMembership(ctx context.Context, user, org string, membership *gh.Membership) (*gh.Membership, *gh.Response, error) {
	var updatedMembership *gh.Membership
	var response *gh.Response
	err := s.tracker.mutate(ctx, "set_membership", func() error {
		var callErr error
		updatedMembership, response, callErr = s.Service.EditOrgMembership(ctx, user, org, membership)
		return callErr
	})
	return updatedMembership, response, err
}

type trackedTeamService struct {
	teams.Service
	tracker *executionTracker
}

func (s *trackedTeamService) CreateTeam(ctx context.Context, org string, team gh.NewTeam) (*gh.Team, *gh.Response, error) {
	var createdTeam *gh.Team
	var response *gh.Response
	err := s.tracker.mutate(ctx, "create_team", func() error {
		var callErr error
		createdTeam, response, callErr = s.Service.CreateTeam(ctx, org, team)
		return callErr
	})
	return createdTeam, response, err
}

func (s *trackedTeamService) EditTeamBySlug(ctx context.Context, org, slug string, team gh.NewTeam, removeParent bool) (*gh.Team, *gh.Response, error) {
	var updatedTeam *gh.Team
	var response *gh.Response
	err := s.tracker.mutate(ctx, "update_team", func() error {
		var callErr error
		updatedTeam, response, callErr = s.Service.EditTeamBySlug(ctx, org, slug, team, removeParent)
		return callErr
	})
	return updatedTeam, response, err
}

func (s *trackedTeamService) AddTeamMembershipBySlug(ctx context.Context, org, slug, user string, options *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
	var membership *gh.Membership
	var response *gh.Response
	err := s.tracker.mutate(ctx, "add_team_member", func() error {
		var callErr error
		membership, response, callErr = s.Service.AddTeamMembershipBySlug(ctx, org, slug, user, options)
		return callErr
	})
	return membership, response, err
}

func (s *trackedTeamService) AddTeamRepoBySlug(ctx context.Context, org, slug, owner, name string, options *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
	var response *gh.Response
	err := s.tracker.mutate(ctx, "grant_team_repository_permission", func() error {
		var callErr error
		response, callErr = s.Service.AddTeamRepoBySlug(ctx, org, slug, owner, name, options)
		return callErr
	})
	return response, err
}
