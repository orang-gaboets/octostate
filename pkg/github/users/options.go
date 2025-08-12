package users

// GetUserOptions defines the options for retrieving a GitHub user.
type GetUserOptions struct {
	Username string
	Service  Service
}
