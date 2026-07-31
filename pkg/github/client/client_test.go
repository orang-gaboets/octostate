package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v88/github"
	"golang.org/x/oauth2"
)

func TestNew_DefaultTimeout(t *testing.T) {
	ctx := context.Background()
	c := New(ctx, "token")
	if c.Client().Timeout != DefaultTimeout {
		t.Fatalf("expected timeout %v, got %v", DefaultTimeout, c.Client().Timeout)
	}
}

func TestNew_WithTimeout(t *testing.T) {
	ctx := context.Background()
	timeout := 10 * time.Second
	c := New(ctx, "token", WithTimeout(timeout))
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
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			authHeaders <- r.Header.Get("Authorization")
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Request:    r,
			}, nil
		}),
	})

	c, err := NewPAT(ctx, "token")
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (fn roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return fn(r)
}
