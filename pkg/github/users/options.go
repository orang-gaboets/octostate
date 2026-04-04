package users

import (
	"fmt"

	"github.com/orang-gaboets/octostate/pkg/github"
)

// GetUserByIDOptions defines the options for retrieving a GitHub user by ID.
type GetUserByIDOptions struct {
	Service Service
	ID      int64
}

// Validate checks if the GetUserByIDOptions are valid.
func (opt *GetUserByIDOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.ID <= 0 {
		return fmt.Errorf("user ID must be greater than zero: %w", github.ErrMissingRequiredField)
	}
	return nil
}

// GetUserByUsernameOptions defines the options for retrieving a GitHub user.
type GetUserByUsernameOptions struct {
	Service  Service
	Username string
}

// Validate checks if the GetUserByUsernameOptions are valid.
func (opt *GetUserByUsernameOptions) Validate() error {
	if opt.Service == nil {
		return github.ErrNilService
	}
	if opt.Username == "" {
		return github.ErrMissingRequiredField
	}
	return nil
}
