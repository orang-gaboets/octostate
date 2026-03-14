package diff

import (
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/snapshot"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
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

	report.Actions = append(report.Actions, builder.planRepositories()...)
	report.Actions = append(report.Actions, builder.planTeams()...)
	report.Actions, err = builder.appendInviteActions(report.Actions)
	if err != nil {
		return nil, err
	}
	report.Actions = append(report.Actions, builder.planTeamMembers()...)
	report.Actions = append(report.Actions, builder.planTeamRepositoryPermissions()...)
	report.Normalize()
	return report, nil
}

type builder struct {
	desired                         config.OrganizationConfig
	actual                          state.OrganizationState
	resolvedInviteUserIDsByUsername map[string]int64
}

func organizationStateFromSnapshot(actual *snapshot.ActualSnapshot) state.OrganizationState {
	if actual == nil {
		return state.OrganizationState{}
	}

	organization := state.OrganizationState{
		Organization:              actual.Organization,
		Members:                   append([]state.OrganizationMember{}, actual.Members...),
		PendingInvitations:        clonePendingInvitations(actual.PendingInvitations),
		Repositories:              cloneRepositories(actual.Repositories),
		Teams:                     append([]state.Team{}, actual.Teams...),
		TeamMembers:               append([]state.TeamMember{}, actual.TeamMembers...),
		TeamRepositoryPermissions: append([]state.TeamRepositoryPermission{}, actual.TeamRepositoryPermissions...),
	}
	organization.Normalize()
	return organization
}
