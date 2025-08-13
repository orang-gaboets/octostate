package users

// GetUserByIDOptions defines the options for retrieving a GitHub user by ID.
type GetUserByIDOptions struct {
	ID      int64
	Service Service
}

// GetUserByUsernameOptions defines the options for retrieving a GitHub user.
type GetUserByUsernameOptions struct {
	Username string
	Service  Service
}
