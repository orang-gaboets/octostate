package github

// TeamMembershipOrigin describes the evidence for a user's membership in a
// GitHub team.
type TeamMembershipOrigin string

const (
	// TeamMembershipOriginDirect identifies an explicitly direct membership.
	TeamMembershipOriginDirect TeamMembershipOrigin = "direct"
	// TeamMembershipOriginInherited identifies membership inherited from a child team.
	TeamMembershipOriginInherited TeamMembershipOrigin = "inherited"
	// TeamMembershipOriginUnknown means the membership exists but its origin is not known.
	TeamMembershipOriginUnknown TeamMembershipOrigin = "unknown"
)

// IsValid reports whether origin is one of the supported membership origins.
func (origin TeamMembershipOrigin) IsValid() bool {
	switch origin {
	case TeamMembershipOriginDirect, TeamMembershipOriginInherited, TeamMembershipOriginUnknown:
		return true
	default:
		return false
	}
}
