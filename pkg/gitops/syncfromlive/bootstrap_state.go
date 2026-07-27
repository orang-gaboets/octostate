package syncfromlive

import (
	"strings"

	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func cloneOrganizationState(actual *state.OrganizationState) state.OrganizationState {
	if actual == nil {
		return state.OrganizationState{}
	}

	cloned := actual.Clone()
	cloned.Organization = strings.TrimSpace(cloned.Organization)
	cloned.Normalize()
	return cloned
}
