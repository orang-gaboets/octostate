package organizations

import (
	"context"

	gh "github.com/google/go-github/v55/github"
)

// Service defines the subset of GitHub organization APIs used by the organization package.
type Service interface {
	// Organization-related functions

	// Get gets an organization by its name.
	Get(ctx context.Context, org string) (*gh.Organization, *gh.Response, error)
}
