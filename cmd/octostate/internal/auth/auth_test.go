package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/github"
	githubclient "github.com/orang-gaboets/octostate/pkg/github/client"
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
