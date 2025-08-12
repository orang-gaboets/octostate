package users

import (
	"context"
	"fmt"
	"log"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// GetUser retrieves a GitHub user by their username.
func GetUser(ctx context.Context, opts GetUserOptions) (*github.User, error) {
	if opts.Service == nil {
		return nil, github.ErrNilService
	}

	if opts.Username == "" {
		return nil, fmt.Errorf("username must be provided: %w", github.ErrMissingRequiredField)
	}

	log.Printf("Retrieving user: %s", opts.Username)
	ghUser, _, err := opts.Service.Get(ctx, opts.Username)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve user %s", opts.Username))
	}
	user := github.UserFromGhUser(ghUser)
	log.Printf("Retrieved user: %s", user)
	return user, nil
}
