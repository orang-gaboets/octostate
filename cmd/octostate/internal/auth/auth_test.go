package auth

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"
)

func TestNewClientPATConstructorError(t *testing.T) {
	old := newPATGitHubClient
	t.Cleanup(func() { newPATGitHubClient = old })

	wantErr := errors.New("boom")
	newPATGitHubClient = func(context.Context, string) (*gh.Client, error) {
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
