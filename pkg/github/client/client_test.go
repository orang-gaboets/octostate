package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gh "github.com/google/go-github/v88/github"
)

func TestNewPAT_DefaultTimeout(t *testing.T) {
	ctx := context.Background()
	c, err := NewPAT(ctx, "token")
	if err != nil {
		t.Fatal(err)
	}
	if c.Client().Timeout != DefaultTimeout {
		t.Fatalf("expected timeout %v, got %v", DefaultTimeout, c.Client().Timeout)
	}
}

func TestNewPAT_WithTimeout(t *testing.T) {
	ctx := context.Background()
	timeout := 10 * time.Second
	c, err := NewPAT(ctx, "token", WithTimeout(timeout))
	if err != nil {
		t.Fatal(err)
	}
	if c.Client().Timeout != timeout {
		t.Fatalf("expected timeout %v, got %v", timeout, c.Client().Timeout)
	}
}

func TestNewPAT_PropagatesConstructorError(t *testing.T) {
	old := newGitHubClient
	t.Cleanup(func() { newGitHubClient = old })

	wantErr := errors.New("boom")
	newGitHubClient = func(...gh.ClientOptionsFunc) (*gh.Client, error) {
		return nil, wantErr
	}

	c, err := NewPAT(context.Background(), "token")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if c != nil {
		t.Fatalf("expected nil client, got %#v", c)
	}
}

func TestNewPAT_SetsAuthorizationHeader(t *testing.T) {
	authHeaders := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(server.Close)

	old := newGitHubClient
	t.Cleanup(func() { newGitHubClient = old })

	baseURL := server.URL + "/"
	newGitHubClient = func(opts ...gh.ClientOptionsFunc) (*gh.Client, error) {
		opts = append(opts, gh.WithURLs(&baseURL, &baseURL))
		return gh.NewClient(opts...)
	}

	c, err := NewPAT(context.Background(), "token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, _, err := c.Users.Get(context.Background(), "octocat"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := <-authHeaders; got != "Bearer token" {
		t.Fatalf("expected authorization header %q, got %q", "Bearer token", got)
	}
}
