package auth

import (
	"context"
	"errors"
	"os"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/github"
	githubclient "github.com/orang-gaboets/octostate/pkg/github/client"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/repos"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	"github.com/orang-gaboets/octostate/pkg/github/users"
)

// Client defines the GitHub client contract used by commands.
type Client interface {
	Organizations() organizations.Service
	Repositories() repos.Service
	Teams() teams.Service
	Users() users.Service
}

const githubTokenEnv = "OCTOSTATE_GITHUB_TOKEN"

type githubClientWrapper struct {
	*gh.Client
}

type repositoriesServiceWrapper struct {
	*gh.RepositoriesService
}

func (s repositoriesServiceWrapper) ListAllTopics(ctx context.Context, owner, repo string) ([]string, *gh.Response, error) {
	return s.RepositoriesService.ListAllTopics(ctx, owner, repo, nil)
}

func (g githubClientWrapper) Organizations() organizations.Service { return g.Client.Organizations }
func (g githubClientWrapper) Repositories() repos.Service {
	return repositoriesServiceWrapper{g.Client.Repositories}
}
func (g githubClientWrapper) Teams() teams.Service { return g.Client.Teams }
func (g githubClientWrapper) Users() users.Service { return g.Client.Users }

var (
	errNilPATGitHubClient = errors.New("github PAT client construction returned nil client")

	// newPATGitHubClient overrides only the raw client construction so tests can
	// exercise originalNewPATClient's error-propagation behavior.
	newPATGitHubClient = githubclient.NewPAT

	// originalNewPATClient is the original function to create a new GitHub client using a personal access token.
	originalNewPATClient = func(ctx context.Context, token string) (Client, error) {
		c, err := newPATGitHubClient(ctx, token)
		if err != nil {
			return nil, err
		}
		if c == nil {
			return nil, errNilPATGitHubClient
		}
		return githubClientWrapper{c}, nil
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
func SetNewPATClient(f func(context.Context, string) (Client, error)) {
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
	newPATGitHubClient = githubclient.NewPAT
}

// NewClient returns an authenticated GitHub client based on the provided credentials.
// Exactly one authentication method must be supplied: either a personal access token
// or a GitHub App's credentials.
func NewClient(ctx context.Context, token string, appID, installationID int64, appKeyPath string) (Client, error) {
	tokenProvided := token != "" && token != explicitEmptyToken
	if token == "" {
		token = os.Getenv(githubTokenEnv)
		tokenProvided = token != ""
	}
	appProvided := appID > 0 || installationID > 0 || appKeyPath != ""

	switch {
	case tokenProvided && appProvided:
		return nil, github.ErrConflictingCredentials
	case tokenProvided:
		return newPATClient(ctx, token)
	case appID != 0 && installationID != 0 && appKeyPath != "":
		return newAppClient(appID, installationID, appKeyPath)
	default:
		return nil, github.ErrNoValidCredentials
	}
}
