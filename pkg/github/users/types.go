package users

import (
	"context"

	gh "github.com/google/go-github/v55/github"
)

// Service defines the interface for managing GitHub users.
type Service interface {
	// Get retrieves a GitHub user by their username.
	Get(ctx context.Context, username string) (*gh.User, *gh.Response, error)
}
