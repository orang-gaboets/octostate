package diff

import (
	"context"
	"fmt"
	"strings"

	"github.com/orang-gaboets/octostate/internal/orderedtasks"
	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/snapshot"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// Options defines the desired and snapshot inputs required to build one
// offline GitOps drift report.
type Options struct {
	Desired                         config.OrganizationConfig
	Snapshot                        *snapshot.ActualSnapshot
	ResolvedInviteUserIDsByUsername map[string]int64
}

// Validate checks if the drift builder inputs are usable.
func (opt *Options) Validate() error {
	desiredOrg := strings.TrimSpace(opt.Desired.Organization)
	switch {
	case desiredOrg == "":
		return fmt.Errorf("organization is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Snapshot == nil:
		return fmt.Errorf("actual snapshot is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Snapshot.Organization != "" && !strings.EqualFold(opt.Snapshot.Organization, desiredOrg):
		return fmt.Errorf(
			"snapshot organization %q does not match desired organization %q: %w",
			opt.Snapshot.Organization,
			desiredOrg,
			githubpkg.ErrInvalidFieldValue,
		)
	default:
		return nil
	}
}

// Build computes a deterministic, read-only drift report from desired GitOps
// configuration and a stored actual-state snapshot.
func Build(opt Options) (*Report, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	var rawResolvedInviteUserIDsByUsername map[string]int64
	var err error
	if opt.Snapshot != nil && opt.Snapshot.ResolvedInviteUserIDsByUsername != nil {
		rawResolvedInviteUserIDsByUsername = make(map[string]int64, len(opt.Snapshot.ResolvedInviteUserIDsByUsername))
		for username, userID := range opt.Snapshot.ResolvedInviteUserIDsByUsername {
			rawResolvedInviteUserIDsByUsername[username] = userID
		}
	}
	if opt.ResolvedInviteUserIDsByUsername != nil {
		if rawResolvedInviteUserIDsByUsername == nil {
			rawResolvedInviteUserIDsByUsername = make(map[string]int64, len(opt.ResolvedInviteUserIDsByUsername))
		}
		for username, userID := range opt.ResolvedInviteUserIDsByUsername {
			rawResolvedInviteUserIDsByUsername[username] = userID
		}
	}

	var resolvedInviteUserIDsByUsername map[string]int64
	if rawResolvedInviteUserIDsByUsername != nil {
		resolvedInviteUserIDsByUsername, err = snapshot.NormalizeResolvedInviteUserIDsByUsername(rawResolvedInviteUserIDsByUsername)
		if err != nil {
			return nil, err
		}
	}

	builder := builder{
		desired:                         opt.Desired,
		actual:                          organizationStateFromSnapshot(opt.Snapshot),
		resolvedInviteUserIDsByUsername: resolvedInviteUserIDsByUsername,
	}

	report := &Report{
		Organization:     strings.TrimSpace(opt.Desired.Organization),
		SnapshotPulledAt: opt.Snapshot.PulledAt.UTC(),
	}

	report.Actions, err = builder.buildActions()
	if err != nil {
		return nil, err
	}
	report.Normalize()
	return report, nil
}

type builder struct {
	desired                         config.OrganizationConfig
	actual                          state.OrganizationState
	resolvedInviteUserIDsByUsername map[string]int64
}

const diffPhaseConcurrency = 6

type actionPhase func(context.Context) ([]Action, error)

func (b builder) buildActions() ([]Action, error) {
	return b.buildActionsWithLimit(diffPhaseConcurrency)
}

func (b builder) buildActionsWithLimit(limit int) ([]Action, error) {
	phaseResults, err := runPhases(context.Background(), limit, b.phases())
	if err != nil {
		return nil, err
	}
	return flattenPhaseResults(phaseResults), nil
}

func flattenPhaseResults(phaseResults [][]Action) []Action {
	totalActions := 0
	for _, actions := range phaseResults {
		totalActions += len(actions)
	}

	actions := make([]Action, 0, totalActions)
	for _, phaseActions := range phaseResults {
		actions = append(actions, phaseActions...)
	}
	return actions
}

func actionsPhase(run func() []Action) actionPhase {
	return func(ctx context.Context) ([]Action, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return run(), nil
	}
}

func erroringActionsPhase(run func() ([]Action, error)) actionPhase {
	return func(ctx context.Context) ([]Action, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return run()
	}
}

func (b builder) phases() []actionPhase {
	return []actionPhase{
		actionsPhase(b.planRepositories),
		actionsPhase(b.planTeams),
		actionsPhase(b.planOrganizationMembers),
		erroringActionsPhase(func() ([]Action, error) {
			return b.appendInviteActions(nil)
		}),
		actionsPhase(b.planTeamMembers),
		actionsPhase(b.planTeamRepositoryPermissions),
	}
}

func runPhases(ctx context.Context, limit int, phases []actionPhase) ([][]Action, error) {
	results := make([][]Action, len(phases))
	tasks := make([]orderedtasks.Task, 0, len(phases))

	for i, phase := range phases {
		if phase == nil {
			continue
		}

		tasks = append(tasks, func(ctx context.Context) error {
			actions, err := phase(ctx)
			if err != nil {
				return err
			}
			results[i] = actions
			return nil
		})
	}

	if err := orderedtasks.Run(ctx, limit, tasks); err != nil {
		return nil, err
	}
	return results, nil
}

func organizationStateFromSnapshot(actual *snapshot.ActualSnapshot) state.OrganizationState {
	if actual == nil {
		return state.OrganizationState{}
	}

	organization := state.OrganizationState{
		Organization:              actual.Organization,
		Members:                   append([]state.OrganizationMember{}, actual.Members...),
		PendingInvitations:        state.ClonePendingInvitations(actual.PendingInvitations),
		Repositories:              state.CloneRepositories(actual.Repositories),
		Teams:                     append([]state.Team{}, actual.Teams...),
		TeamMembers:               append([]state.TeamMember{}, actual.TeamMembers...),
		TeamRepositoryPermissions: append([]state.TeamRepositoryPermission{}, actual.TeamRepositoryPermissions...),
	}
	organization.Normalize()
	return organization
}
