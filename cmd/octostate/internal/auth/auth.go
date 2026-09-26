package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"

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

type githubTeamServiceWrapper struct {
	teams.Service
	client *gh.Client
}

type teamMemberResponse struct {
	Login string `json:"login"`
	Role  string `json:"role"`
}

func (s repositoriesServiceWrapper) ListAllTopics(ctx context.Context, owner, repo string) ([]string, *gh.Response, error) {
	return s.RepositoriesService.ListAllTopics(ctx, owner, repo, nil)
}

func (s githubTeamServiceWrapper) ListTeamMembersBySlugWithRoles(ctx context.Context, org, slug string, opts *gh.ListOptions) ([]teams.TeamMember, *gh.Response, error) {
	query := url.Values{}
	query.Set("role", string(teams.TeamMemberRoleAll))
	if opts != nil {
		if opts.PerPage > 0 {
			query.Set("per_page", strconv.Itoa(opts.PerPage))
		}
		if opts.Page > 0 {
			query.Set("page", strconv.Itoa(opts.Page))
		}
	}

	endpoint := fmt.Sprintf("orgs/%s/teams/%s/members?%s", url.PathEscape(org), url.PathEscape(slug), query.Encode())
	req, err := s.client.NewRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}

	var response []teamMemberResponse
	resp, err := s.client.Do(req, &response)
	if err != nil {
		return nil, resp, err
	}

	members := make([]teams.TeamMember, 0, len(response))
	for _, member := range response {
		members = append(members, teams.TeamMember{
			Username: member.Login,
			Role:     teams.TeamMemberRole(member.Role),
		})
	}
	return members, resp, nil
}

func (g githubClientWrapper) Organizations() organizations.Service { return g.Client.Organizations }
func (g githubClientWrapper) Repositories() repos.Service {
	return repositoriesServiceWrapper{g.Client.Repositories}
}
func (g githubClientWrapper) Teams() teams.Service {
	return githubTeamServiceWrapper{Service: g.Client.Teams, client: g.Client}
}
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
