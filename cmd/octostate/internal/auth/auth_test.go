package auth

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"
	githubclient "github.com/orang-gaboets/octostate/pkg/github/client"
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
