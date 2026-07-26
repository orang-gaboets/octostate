package client

import (
	"context"
	"net/http"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	gh "github.com/google/go-github/v88/github"
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

var newGitHubClient = gh.NewClient

// NewPAT creates a new GitHub client using the provided OAuth token.
func NewPAT(ctx context.Context, token string, opts ...Option) (*gh.Client, error) {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	tc.Timeout = DefaultTimeout
	for _, opt := range opts {
		opt(tc)
	}
	return newGitHubClient(gh.WithHTTPClient(tc))
}

// New creates a new GitHub client using the provided OAuth token.
// It is kept for backward compatibility.
func New(ctx context.Context, token string, opts ...Option) *gh.Client {
	client, _ := NewPAT(ctx, token, opts...)
	return client
}

// NewApp creates a new GitHub client using the provided app ID, installation ID, and private key.
func NewApp(appID, installationID int64, pem []byte, opts ...Option) (*gh.Client, error) {
	tr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, pem)
	if err != nil {
		return nil, err
	}
	tc := &http.Client{Transport: tr, Timeout: DefaultTimeout}
	for _, opt := range opts {
		opt(tc)
	}
	client, err := gh.NewClient(gh.WithHTTPClient(tc))
	if err != nil {
		return nil, err
	}
	return client, nil
}
