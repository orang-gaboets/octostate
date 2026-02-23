package users

import (
	"context"
	"fmt"

	"github.com/orang-gaboets/repo-builder/pkg/github"
	ghlogging "github.com/orang-gaboets/repo-builder/pkg/github/logging"
)

// GetUserByID retrieves a GitHub user by their ID.
func GetUserByID(ctx context.Context, opts GetUserByIDOptions) (*github.User, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	ghlogging.Debugf(ctx, "get user by id %d", opts.ID)
	ghUser, _, err := opts.Service.GetByID(ctx, opts.ID)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve user with ID %d", opts.ID))
	}
	user := github.UserFromGhUser(ghUser)
	ghlogging.Debugf(ctx, "retrieved user by id %d", opts.ID)
	return user, nil
}

// GetUserByUsername retrieves a GitHub user by their username.
func GetUserByUsername(ctx context.Context, opts GetUserByUsernameOptions) (*github.User, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	ghlogging.Debugf(ctx, "get user by username %s", opts.Username)
	ghUser, _, err := opts.Service.Get(ctx, opts.Username)
	if err != nil {
		return nil, github.WrapError(err, fmt.Sprintf("failed to retrieve user %s", opts.Username))
	}
	user := github.UserFromGhUser(ghUser)
	ghlogging.Debugf(ctx, "retrieved user by username %s", opts.Username)
	return user, nil
}
