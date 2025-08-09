package client

import (
	"context"
	"net/http"
	"time"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
)

// DefaultTimeout is the default timeout for HTTP requests made by the GitHub client.
const DefaultTimeout = 30 * time.Second

// Option configures the HTTP client used by New.
type Option func(*http.Client)

// WithTimeout overrides the default HTTP client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *http.Client) {
		c.Timeout = d
	}
}

// New creates a new GitHub client using the provided OAuth token.
func New(ctx context.Context, token string, opts ...Option) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	tc.Timeout = DefaultTimeout
	for _, opt := range opts {
		opt(tc)
	}
	return github.NewClient(tc)
}
