package apply

import (
	"errors"
	"fmt"
	"strings"

	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/internal/resourceid"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// ErrUnfulfilledDesiredAction reports that a requested mutation cannot be
// completed, including a desired team membership whose actual origin is
// unknown, while the caller required every desired mutation to be executable.
var ErrUnfulfilledDesiredAction = errors.New("desired action cannot be executed")

// ErrUnresolvedTeamMembership reports a supplied plan action that would act
// on a membership whose direct/inherited origin cannot be confirmed.
var ErrUnresolvedTeamMembership = errors.New("team membership origin is unresolved")

// UnfulfilledDesiredActions returns the desired create and update actions that
// planning has determined cannot execute.
//
// A non-executable action means one of two different things, and callers
// automating desired state need to tell them apart:
//
//   - a delete or remove is drift Octostate intentionally declines to
//     reconcile, and is expected to stay non-executable; and
//   - a create or update is a requested mutation that will silently not happen.
//
// Only the second kind is returned here.
func UnfulfilledDesiredActions(actions []gitopsplan.Action) []gitopsplan.Action {
	unfulfilled := make([]gitopsplan.Action, 0)
	for _, action := range actions {
		if action.Executable {
			continue
		}
		switch action.Operation {
		case gitopsplan.ActionOperationCreate, gitopsplan.ActionOperationUpdate:
			unfulfilled = append(unfulfilled, action)
		}
	}
	return unfulfilled
}

// requireExecutableDesiredActions validates membership-origin safety, returns
// unresolved actual memberships for the result, and fails when the caller
// asked for every desired mutation to be executable but that condition cannot
// be established.
func requireExecutableDesiredActions(opt Options) ([]state.TeamMember, error) {
	if opt.Actual == nil || opt.Plan == nil {
		return []state.TeamMember{}, nil
	}

	actualMembers, err := state.CanonicalTeamMembers(opt.Actual.TeamMembers)
	if err != nil {
		return nil, fmt.Errorf("validate actual team memberships: %w", err)
	}
	unknownMembers := make(map[string]struct{})
	unresolved := make([]state.TeamMember, 0)
	for _, member := range actualMembers {
		if member.Origin == github.TeamMembershipOriginUnknown {
			unknownMembers[resourceid.TeamMemberKey(member.TeamSlug, member.Username)] = struct{}{}
			unresolved = append(unresolved, member)
		}
	}

	for _, action := range opt.Plan.Actions {
		if action.ResourceType != gitopsplan.ActionResourceTypeTeamMember {
			continue
		}
		teamSlug, username, ok := strings.Cut(action.ResourceID, "/")
		if !ok {
			continue
		}
		key := resourceid.TeamMemberKey(strings.TrimSpace(teamSlug), strings.TrimSpace(username))
		if _, ok := unknownMembers[key]; ok {
			return nil, fmt.Errorf("plan action %s targets unresolved team membership %s: %w", action.Operation, action.ResourceID, ErrUnresolvedTeamMembership)
		}
	}

	if !opt.RequireExecutableDesiredActions {
		return unresolved, nil
	}

	unfulfilled := UnfulfilledDesiredActions(opt.Plan.Actions)
	unresolvedDesired := make([]string, 0)
	for _, team := range opt.Desired.Teams {
		for _, member := range team.Members {
			key := resourceid.TeamMemberKey(strings.TrimSpace(team.Slug), strings.TrimSpace(member.Username))
			if _, ok := unknownMembers[key]; ok {
				unresolvedDesired = append(unresolvedDesired, resourceid.TeamMemberID(strings.TrimSpace(team.Slug), strings.TrimSpace(member.Username)))
			}
		}
	}
	if len(unfulfilled) == 0 && len(unresolvedDesired) == 0 {
		return unresolved, nil
	}

	details := make([]string, 0, len(unfulfilled)+len(unresolvedDesired))
	for _, action := range unfulfilled {
		details = append(details, fmt.Sprintf("%s %s %s: %s",
			action.Operation, action.ResourceType, action.ResourceID, action.Message))
	}
	for _, membershipID := range unresolvedDesired {
		details = append(details, fmt.Sprintf("team membership %s has unknown origin", membershipID))
	}

	return nil, fmt.Errorf("%w: %d desired action(s) are not executable and %d desired membership(s) are unresolved:\n- %s",
		ErrUnfulfilledDesiredAction, len(unfulfilled), len(unresolvedDesired), strings.Join(details, "\n- "))
}

func applyPlanSummary(plan *gitopsplan.Report, unresolved []state.TeamMember) gitopsplan.Summary {
	if plan == nil {
		return gitopsplan.Summary{}
	}
	report := gitopsplan.Report{
		Organization:              plan.Organization,
		Actions:                   append([]gitopsplan.Action(nil), plan.Actions...),
		UnresolvedTeamMemberships: append([]state.TeamMember(nil), unresolved...),
	}
	report.Normalize()
	return report.Summary
}
