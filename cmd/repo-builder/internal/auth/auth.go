package auth

import (
	"context"
	"os"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	githubclient "github.com/orang-gaboets/repo-builder/pkg/github/client"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
	"github.com/orang-gaboets/repo-builder/pkg/github/users"
)

// Client defines the GitHub client contract used by commands.
type Client interface {
	Organizations() organizations.Service
	Repositories() repos.Service
	Teams() teams.Service
	Users() users.Service
}

type githubClientWrapper struct {
	*gh.Client
}

func (g githubClientWrapper) Organizations() organizations.Service { return g.Client.Organizations }
func (g githubClientWrapper) Repositories() repos.Service          { return g.Client.Repositories }
func (g githubClientWrapper) Teams() teams.Service                 { return g.Client.Teams }
func (g githubClientWrapper) Users() users.Service                 { return g.Client.Users }

var (
	// originalNewPATClient is the original function to create a new GitHub client using a personal access token.
	originalNewPATClient = func(ctx context.Context, token string) Client {
		return githubClientWrapper{githubclient.New(ctx, token)}
	}

	// newPATClient is a function that creates a new GitHub client using a personal access token.
	newPATClient = originalNewPATClient

	// originalNewAppClient is the original function to create a new GitHub App client.
	originalNewAppClient = func(appID, installationID int64, appKeyPath string) (Client, error) {
		key, err := os.ReadFile(appKeyPath)
		if err != nil {
			return nil, err
		}
		c, err := githubclient.NewApp(appID, installationID, key)
		if err != nil {
			return nil, err
		}
		return githubClientWrapper{c}, nil
	}

	// newAppClient is a function that creates a new GitHub App client.
	newAppClient = originalNewAppClient
)

// SetNewPATClient overrides the personal access token client constructor. Used for testing.
func SetNewPATClient(f func(context.Context, string) Client) {
	newPATClient = f
}

// SetNewAppClient overrides the GitHub App client constructor. Used for testing.
func SetNewAppClient(f func(int64, int64, string) (Client, error)) {
	newAppClient = f
}

// ResetClients restores the default client constructors. Used for testing.
func ResetClients() {
	newPATClient = originalNewPATClient
	newAppClient = originalNewAppClient
}

// NewClient returns an authenticated GitHub client based on the provided credentials.
// Exactly one authentication method must be supplied: either a personal access token
// or a GitHub App's credentials.
func NewClient(ctx context.Context, token string, appID, installationID int64, appKeyPath string) (Client, error) {
	tokenProvided := token != ""
	appProvided := appID > 0 || installationID > 0 || appKeyPath != ""

	switch {
	case tokenProvided && appProvided:
		return nil, github.ErrConflictingCredentials
	case tokenProvided:
		return newPATClient(ctx, token), nil
	case appID != 0 && installationID != 0 && appKeyPath != "":
		return newAppClient(appID, installationID, appKeyPath)
	default:
		return nil, github.ErrNoValidCredentials
	}
}
