package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/github"
	githubclient "github.com/orang-gaboets/octostate/pkg/github/client"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	"github.com/spf13/cobra"
)

func TestNewClientPATConstructorError(t *testing.T) {
	old := newPATGitHubClient
	t.Cleanup(func() { newPATGitHubClient = old })

	wantErr := errors.New("boom")
	newPATGitHubClient = func(context.Context, string, ...githubclient.Option) (*gh.Client, error) {
		return nil, wantErr
	}

	c, err := NewClient(context.Background(), "token", 0, 0, "")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if c != nil {
		t.Fatalf("expected nil client, got %#v", c)
	}
}

func TestNewClientPATNilClient(t *testing.T) {
	old := newPATGitHubClient
	t.Cleanup(func() { newPATGitHubClient = old })

	newPATGitHubClient = func(context.Context, string, ...githubclient.Option) (*gh.Client, error) {
		return nil, nil
	}

	c, err := NewClient(context.Background(), "token", 0, 0, "")
	if !errors.Is(err, errNilPATGitHubClient) {
		t.Fatalf("expected %v, got %v", errNilPATGitHubClient, err)
	}
	if c != nil {
		t.Fatalf("expected nil client, got %#v", c)
	}
}

func TestNewClientPATSuccess(t *testing.T) {
	old := newPATGitHubClient
	t.Cleanup(func() { newPATGitHubClient = old })

	want, err := gh.NewClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	newPATGitHubClient = func(context.Context, string, ...githubclient.Option) (*gh.Client, error) {
		return want, nil
	}

	c, err := NewClient(context.Background(), "token", 0, 0, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wrapper, ok := c.(githubClientWrapper)
	if !ok {
		t.Fatalf("expected githubClientWrapper, got %T", c)
	}
	if wrapper.Client != want {
		t.Fatalf("expected wrapped client %p, got %p", want, wrapper.Client)
	}
}

func TestNewClientExplicitTokenTakesPrecedenceOverEnvironment(t *testing.T) {
	t.Setenv("OCTOSTATE_GITHUB_TOKEN", "environment-token")
	t.Cleanup(ResetClients)

	var gotToken string
	SetNewPATClient(func(_ context.Context, token string) (Client, error) {
		gotToken = token
		return MockClient{}, nil
	})

	if _, err := NewClient(context.Background(), "explicit-token", 0, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotToken != "explicit-token" {
		t.Fatalf("expected explicit token, got %q", gotToken)
	}
}

func TestAddFlagsExplicitEmptyTokenDoesNotUseEnvironment(t *testing.T) {
	t.Setenv("OCTOSTATE_GITHUB_TOKEN", "environment-token")
	t.Cleanup(ResetClients)
	SetNewPATClient(func(_ context.Context, token string) (Client, error) {
		t.Fatalf("unexpected PAT client construction with token %q", token)
		return nil, nil
	})

	var token string
	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := NewClient(cmd.Context(), token, 0, 0, "")
			return err
		},
	}
	AddFlags(cmd, &token, new(int64), new(int64), new(string))
	cmd.SetArgs([]string{"--token="})

	err := cmd.Execute()
	if !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestNewClientUsesEnvironmentToken(t *testing.T) {
	t.Setenv("OCTOSTATE_GITHUB_TOKEN", "environment-token")
	t.Cleanup(ResetClients)

	var gotToken string
	SetNewPATClient(func(_ context.Context, token string) (Client, error) {
		gotToken = token
		return MockClient{}, nil
	})

	if _, err := NewClient(context.Background(), "", 0, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotToken != "environment-token" {
		t.Fatalf("expected environment token, got %q", gotToken)
	}
}

func TestNewClientEmptyEnvironmentFallsBackToApp(t *testing.T) {
	t.Setenv("OCTOSTATE_GITHUB_TOKEN", "")
	t.Cleanup(ResetClients)

	called := false
	SetNewAppClient(func(appID, installationID int64, appKeyPath string) (Client, error) {
		called = true
		if appID != 1 || installationID != 2 || appKeyPath != "key.pem" {
			t.Fatalf("unexpected app credentials: %d, %d, %q", appID, installationID, appKeyPath)
		}
		return MockClient{}, nil
	})

	if _, err := NewClient(context.Background(), "", 1, 2, "key.pem"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected App client constructor to be called")
	}
}

func TestNewClientEnvironmentTokenConflictsWithAppCredentials(t *testing.T) {
	t.Setenv("OCTOSTATE_GITHUB_TOKEN", "environment-token")

	_, err := NewClient(context.Background(), "", 1, 2, "key.pem")
	if !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected %v, got %v", github.ErrConflictingCredentials, err)
	}
	if strings.Contains(err.Error(), "environment-token") {
		t.Fatalf("credential leaked in error: %v", err)
	}
}

func TestNewClientEmptyEnvironmentWithoutCredentials(t *testing.T) {
	t.Setenv("OCTOSTATE_GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "unrelated-token")
	t.Setenv("GITHUB_TOKEN", "another-unrelated-token")

	_, err := NewClient(context.Background(), "", 0, 0, "")
	if !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestGitHubTeamServiceRoleAwareListingDecodesRolesAndPaginates(t *testing.T) {
	var requestedPages []string
	transport := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.Path != "/orgs/acme/teams/platform/members" {
			t.Errorf("request = %s %s, want GET /orgs/acme/teams/platform/members", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("role"); got != "all" {
			t.Errorf("role query = %q, want all", got)
		}
		if got := r.URL.Query().Get("per_page"); got != "100" {
			t.Errorf("per_page query = %q, want 100", got)
		}
		page := r.URL.Query().Get("page")
		requestedPages = append(requestedPages, page)
		switch page {
		case "":
			nextPageURL := *r.URL
			nextQuery := nextPageURL.Query()
			nextQuery.Set("page", "2")
			nextPageURL.RawQuery = nextQuery.Encode()
			return jsonResponse(r, http.Header{"Link": {fmt.Sprintf(`<%s>; rel="next"`, nextPageURL.String())}}, `[{"login":"alice","role":"member","inherited":false}]`), nil
		case "2":
			return jsonResponse(r, nil, `[{"login":"bob","role":"maintainer","inherited":true}]`), nil
		default:
			t.Errorf("unexpected page query %q", page)
			return jsonResponse(r, nil, "[]"), nil
		}
	})

	client := newGitHubClientWithTransport(t, transport)
	service := githubClientWrapper{Client: client}.Teams()
	members, err := teams.ListTeamMembersWithRolesBySlug(context.Background(), teams.ListTeamMembersWithRolesBySlugOptions{
		Service: service,
		Org:     "acme",
		Slug:    "platform",
	})
	if err != nil {
		t.Fatalf("ListTeamMembersWithRolesBySlug returned error: %v", err)
	}
	want := []teams.TeamMember{
		{Username: "alice", Role: teams.TeamMemberRoleMember},
		{Username: "bob", Role: teams.TeamMemberRoleMaintainer},
	}
	if len(members) != len(want) {
		t.Fatalf("members = %#v, want %#v", members, want)
	}
	for i := range want {
		if members[i] != want[i] {
			t.Errorf("member[%d] = %#v, want %#v", i, members[i], want[i])
		}
	}
	if len(requestedPages) != 2 || requestedPages[0] != "" || requestedPages[1] != "2" {
		t.Fatalf("requested pages = %#v, want [empty 2]", requestedPages)
	}
}

func TestGitHubTeamServiceRoleAwareListingHonorsCancellation(t *testing.T) {
	client := newGitHubClientWithTransport(t, roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		if err := r.Context().Err(); err != nil {
			return nil, err
		}
		return jsonResponse(r, nil, "[]"), nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := teams.ListTeamMembersWithRolesBySlug(ctx, teams.ListTeamMembersWithRolesBySlugOptions{
		Service: githubClientWrapper{Client: client}.Teams(),
		Org:     "acme",
		Slug:    "platform",
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want %v", err, context.Canceled)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func newGitHubClientWithTransport(t *testing.T, transport http.RoundTripper) *gh.Client {
	t.Helper()
	client, err := gh.NewClient(
		gh.WithEnterpriseURLs("https://api.github.test/", "https://uploads.github.test/"),
		gh.WithHTTPClient(&http.Client{Transport: transport}),
	)
	if err != nil {
		t.Fatalf("gh.NewClient returned error: %v", err)
	}
	return client
}

func jsonResponse(request *http.Request, headers http.Header, body string) *http.Response {
	if headers == nil {
		headers = make(http.Header)
	}
	headers.Set("Content-Type", "application/json")
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     headers,
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}
