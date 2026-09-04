package apply

import (
	"errors"
	"fmt"
	"strings"

	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
)

// ErrUnfulfilledDesiredAction reports that the plan contains a desired
// create or update that planning already determined cannot execute, while the
// caller required every desired mutation to be executable.
var ErrUnfulfilledDesiredAction = errors.New("desired action cannot be executed")

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

// requireExecutableDesiredActions fails when the caller asked for every desired
// mutation to be executable and the plan does not satisfy that.
func requireExecutableDesiredActions(opt Options) error {
	if !opt.RequireExecutableDesiredActions || opt.Plan == nil {
		return nil
	}

	unfulfilled := UnfulfilledDesiredActions(opt.Plan.Actions)
	if len(unfulfilled) == 0 {
		return nil
	}

	details := make([]string, 0, len(unfulfilled))
	for _, action := range unfulfilled {
		details = append(details, fmt.Sprintf("%s %s %s: %s",
			action.Operation, action.ResourceType, action.ResourceID, action.Message))
	}

	return fmt.Errorf("%w: %d desired action(s) are not executable:\n- %s",
		ErrUnfulfilledDesiredAction, len(unfulfilled), strings.Join(details, "\n- "))
}
