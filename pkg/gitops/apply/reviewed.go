package apply

import (
	"context"
	"fmt"
	"strings"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	ghusers "github.com/orang-gaboets/octostate/pkg/github/users"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// ReviewedOptions contains the current desired and actual state for an apply
// bound to a caller-provided reviewed configuration identity. Actual must be
// collected for the current invocation. It intentionally has no Plan field:
// reviewed operations build a fresh plan from Desired and Actual after the
// identity check succeeds.
type ReviewedOptions struct {
	Desired                         config.OrganizationConfig
	Actual                          *state.OrganizationState
	ExpectedConfigIdentity          string
	ExpectedOrganization            string
	OrganizationService             organizations.Service
	RepositoryService               repos.Service
	TeamService                     teams.Service
	UserService                     ghusers.Service
	RequireExecutableDesiredActions bool
}

// CheckReviewed verifies the reviewed desired-state identity, builds a fresh
// plan from the supplied current Actual state, and runs apply preflight without
// mutating GitHub.
func CheckReviewed(ctx context.Context, opt ReviewedOptions) (*CheckResult, error) {
	applyOptions, err := buildReviewedApplyOptions(ctx, opt)
	if err != nil {
		return nil, err
	}
	return Check(ctx, applyOptions)
}

// ExecuteReviewed verifies the reviewed desired-state identity, builds a
// fresh plan from the supplied current Actual state, and executes that plan.
func ExecuteReviewed(ctx context.Context, opt ReviewedOptions) (*Result, error) {
	applyOptions, err := buildReviewedApplyOptions(ctx, opt)
	if err != nil {
		return nil, err
	}
	return Execute(ctx, applyOptions)
}

func buildReviewedApplyOptions(ctx context.Context, opt ReviewedOptions) (Options, error) {
	if err := config.CheckExpectedDesiredStateIdentity(opt.Desired, opt.ExpectedConfigIdentity); err != nil {
		return Options{}, err
	}
	if opt.ExpectedOrganization != "" {
		if err := config.CheckExpectedOrganization(opt.Desired.Organization, opt.ExpectedOrganization); err != nil {
			return Options{}, err
		}
	}
	if err := opt.validate(); err != nil {
		return Options{}, err
	}

	report, err := gitopsplan.Build(ctx, gitopsplan.Options{
		Desired:     opt.Desired,
		Actual:      opt.Actual,
		UserService: opt.UserService,
	})
	if err != nil {
		return Options{}, fmt.Errorf("build fresh reviewed reconciliation plan: %w", err)
	}
	return Options{
		Desired:                         opt.Desired,
		Actual:                          opt.Actual,
		Plan:                            report,
		OrganizationService:             opt.OrganizationService,
		RepositoryService:               opt.RepositoryService,
		TeamService:                     opt.TeamService,
		UserService:                     opt.UserService,
		RequireExecutableDesiredActions: opt.RequireExecutableDesiredActions,
	}, nil
}

func (opt ReviewedOptions) validate() error {
	desiredOrg := strings.TrimSpace(opt.Desired.Organization)
	switch {
	case desiredOrg == "":
		return fmt.Errorf("organization is required: %w", githubpkg.ErrMissingRequiredField)
	case opt.Actual == nil:
		return fmt.Errorf("actual state is required: %w", githubpkg.ErrMissingRequiredField)
	case strings.TrimSpace(opt.Actual.Organization) != "" && !strings.EqualFold(strings.TrimSpace(opt.Actual.Organization), desiredOrg):
		return fmt.Errorf("actual organization %q does not match desired organization %q: %w", opt.Actual.Organization, desiredOrg, githubpkg.ErrInvalidFieldValue)
	case opt.OrganizationService == nil || opt.RepositoryService == nil || opt.TeamService == nil || opt.UserService == nil:
		return githubpkg.ErrNilService
	}
	return nil
}
