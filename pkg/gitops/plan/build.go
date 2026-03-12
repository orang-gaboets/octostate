package plan

import (
	"context"
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/repo-builder/pkg/github"
	ghusers "github.com/orang-gaboets/repo-builder/pkg/github/users"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

// Options defines the desired and actual state inputs required to build one
// GitOps reconciliation plan.
type Options struct {
	Desired     config.OrganizationConfig
	Actual      *state.OrganizationState
	UserService ghusers.Service
}

// Validate checks if the planner inputs are usable.
func (opt *Options) Validate() error {
	desiredOrg := strings.TrimSpace(opt.Desired.Organization)
	switch {
	case desiredOrg == "":
		return fmt.Errorf("organization is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Actual == nil:
		return fmt.Errorf("actual state is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Actual.Organization != "" && !strings.EqualFold(opt.Actual.Organization, desiredOrg):
		return fmt.Errorf(
			"actual organization %q does not match desired organization %q: %w",
			opt.Actual.Organization,
			desiredOrg,
			githubpkg.ErrInvalidFieldValue,
		)
	default:
		return nil
	}
}

// Build computes a deterministic, read-only reconciliation plan from desired
// GitOps configuration and collected live GitHub state.
func Build(ctx context.Context, opt Options) (*Report, error) {
	if err := opt.Validate(); err != nil {
		return nil, err
	}

	planner := planner{
		ctx:            ctx,
		desired:        opt.Desired,
		actual:         opt.Actual,
		userService:    opt.UserService,
		userLoginsByID: map[int64]string{},
	}

	report := &Report{
		Organization: strings.TrimSpace(opt.Desired.Organization),
	}

	var err error
	report.Actions = append(report.Actions, planner.planRepositories()...)
	report.Actions = append(report.Actions, planner.planTeams()...)
	report.Actions, err = planner.appendInviteActions(report.Actions)
	if err != nil {
		return nil, err
	}
	report.Actions = append(report.Actions, planner.planTeamMembers()...)
	report.Actions = append(report.Actions, planner.planTeamRepositoryPermissions()...)
	report.Normalize()
	return report, nil
}

type planner struct {
	ctx            context.Context
	desired        config.OrganizationConfig
	actual         *state.OrganizationState
	userService    ghusers.Service
	userLoginsByID map[int64]string
}
