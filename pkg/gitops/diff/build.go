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
	resolvedInviteUserIDsByUsername, err := normalizeResolvedInviteUserIDsByUsername(opt.ResolvedInviteUserIDsByUsername)
	if err != nil {
		return nil, err
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

func normalizeResolvedInviteUserIDsByUsername(values map[string]int64) (map[string]int64, error) {
	if len(values) == 0 {
		return map[string]int64{}, nil
	}

	normalized := make(map[string]int64, len(values))
	for username, userID := range values {
		username = strings.ToLower(strings.TrimSpace(username))
		if username == "" || userID <= 0 {
			continue
		}
		if existingUserID, ok := normalized[username]; ok && existingUserID != userID {
			return nil, fmt.Errorf(
				"resolved invite user IDs contain conflicting entries for username %q: %w",
				username,
				githubpkg.ErrInvalidFieldValue,
			)
		}
		normalized[username] = userID
	}
	if len(normalized) == 0 {
		return map[string]int64{}, nil
	}
	return normalized, nil
}
