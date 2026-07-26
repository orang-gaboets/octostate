package client

import (
	"context"
	"errors"
	"testing"
	"time"

	gh "github.com/google/go-github/v88/github"
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
