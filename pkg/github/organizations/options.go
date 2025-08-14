package organizations

// GetOptions defines the options for retrieving organization details.
type GetOptions struct {
	OrgName string
	Service Service
}

// InviteUserOptions defines the options for inviting a user to an organization.
type InviteUserOptions struct {
	OrgName string
	UserID  int64
	Service Service
}
