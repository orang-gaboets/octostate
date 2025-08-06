package builder

import (
	"context"

	"github.com/google/go-github/v55/github"
	"golang.org/x/oauth2"
)

// NewGitHubClient creates a new GitHub client using the provided OAuth token.
func New(ctx context.Context, token string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}
