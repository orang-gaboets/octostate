package plan

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	ghusers "github.com/orang-gaboets/octostate/pkg/github/users"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
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

	actions, err := planner.buildActions()
	if err != nil {
		return nil, err
	}
	report.Actions = append(report.Actions, actions...)
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

const planPhaseConcurrency = 6

type planBuildResult struct {
	repositoryActions               []Action
	teamActions                     []Action
	organizationMemberActions       []Action
	inviteActions                   []Action
	teamMemberActions               []Action
	teamRepositoryPermissionActions []Action
}

func (p planner) buildActions() ([]Action, error) {
	g, groupCtx := errgroup.WithContext(p.ctx)
	g.SetLimit(planPhaseConcurrency)

	result := planBuildResult{}

	g.Go(func() error {
		result.repositoryActions = p.planRepositories()
		return nil
	})
	g.Go(func() error {
		result.teamActions = p.planTeams()
		return nil
	})
	g.Go(func() error {
		result.organizationMemberActions = p.planOrganizationMembers()
		return nil
	})
	g.Go(func() error {
		invitePlanner := p
		invitePlanner.ctx = groupCtx

		inviteActions, err := invitePlanner.appendInviteActions(nil)
		if err != nil {
			return err
		}
		result.inviteActions = inviteActions
		return nil
	})
	g.Go(func() error {
		result.teamMemberActions = p.planTeamMembers()
		return nil
	})
	g.Go(func() error {
		result.teamRepositoryPermissionActions = p.planTeamRepositoryPermissions()
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	actions := make([]Action, 0,
		len(result.repositoryActions)+
			len(result.teamActions)+
			len(result.organizationMemberActions)+
			len(result.inviteActions)+
			len(result.teamMemberActions)+
			len(result.teamRepositoryPermissionActions),
	)
	actions = append(actions, result.repositoryActions...)
	actions = append(actions, result.teamActions...)
	actions = append(actions, result.organizationMemberActions...)
	actions = append(actions, result.inviteActions...)
	actions = append(actions, result.teamMemberActions...)
	actions = append(actions, result.teamRepositoryPermissionActions...)
	return actions, nil
}
