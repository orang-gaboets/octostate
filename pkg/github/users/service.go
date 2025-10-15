package users

import (
	"context"
	"fmt"
	"log"

	"github.com/orang-gaboets/repo-builder/pkg/github"
)

// GetUserByID retrieves a GitHub user by their ID.
func GetUserByID(ctx context.Context, opts GetUserByIDOptions) (*github.User, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	log.Printf("Retrieving user with ID: %d", opts.ID)
	ghUser, _, err := opts.Service.GetByID(ctx, opts.ID)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve user with ID %d", opts.ID))
	}
	user := github.UserFromGhUser(ghUser)
	log.Printf("Retrieved user: %s", user)
	return user, nil
}

// GetUserByUsername retrieves a GitHub user by their username.
func GetUserByUsername(ctx context.Context, opts GetUserByUsernameOptions) (*github.User, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
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
