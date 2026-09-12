package plan

import "fmt"

// organizationMemberPlan carries the planned organization-member actions
// alongside the availability those actions produce, so dependent phases reason
// about the final supported state of this plan rather than only the live state
// that preceded it.
//
// This mirrors repositoryPlan, which serves the same purpose for managed
// repository dependencies.
type organizationMemberPlan struct {
	actions      []Action
	availability map[string]organizationMemberAvailability
}

// organizationMemberAvailability records whether a desired team membership can
// rely on the member existing by the time it runs.
type organizationMemberAvailability struct {
	executable bool
	diagnostic string
}

// availabilityFor reports whether the member is usable as a team-membership
// prerequisite, and why not when it is not.
func (p organizationMemberPlan) availabilityFor(username string) organizationMemberAvailability {
	availability, ok := p.availability[organizationMemberKey(username)]
	if !ok {
		availability.diagnostic = fmt.Sprintf(
			"team membership requires organization member %s to exist first",
			organizationMemberID(username),
		)
	}
	return availability
}
