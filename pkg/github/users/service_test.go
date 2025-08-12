package users

import (
	"context"
	"fmt"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

var (
	existingUser = github.User{
		ID:    github.Ptr(int64(12345)),
		Name:  github.Ptr("existing-user"),
		Email: github.Ptr("existing-email"),
		URL:   github.Ptr("existing-url"),
	}
	nonExistentUser = github.User{
		Name: github.Ptr("non-existent-user"),
	}
)

type mockService struct {
	getCalled bool
	getErr    error
	userName  string
	userID    *int64
	userEmail *string
	userURL   *string
}

func (m *mockService) Get(_ context.Context, username string) (*gh.User, *gh.Response, error) {
	m.getCalled = true
	if m.getErr != nil {
		return nil, nil, m.getErr
	}
	if username != *existingUser.Name {
		return nil, nil, fmt.Errorf("user %s not found: %w", username, github.ErrNotFound)
	}
	m.userName = username
	m.userID = existingUser.ID
	m.userEmail = existingUser.Email
	m.userURL = existingUser.URL
	return &gh.User{}, nil, nil
}

// Test Get functionality

func TestGetUserSuccess(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetUserOptions{
		Service:  mockSvc,
		Username: *existingUser.Name,
	}

	ctx := context.Background()
	user, err := GetUser(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected Get to be called")
	}
	if user == nil {
		t.Fatal("expected user to be returned, got nil")
	}
	if mockSvc.userName != *existingUser.Name {
		t.Fatalf("expected username %s, got %s", *existingUser.Name, mockSvc.userName)
	}
	if mockSvc.userID != existingUser.ID {
		t.Fatalf("expected user ID %d, got %d", *existingUser.ID, *mockSvc.userID)
	}
	if mockSvc.userEmail != existingUser.Email {
		t.Fatalf("expected user email %s, got %s", *existingUser.Email, *mockSvc.userEmail)
	}
	if mockSvc.userURL != existingUser.URL {
		t.Fatalf("expected user URL %s, got %s", *existingUser.URL, *mockSvc.userURL)
	}
}

func TestGetUserNonExistentUser(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetUserOptions{
		Service:  mockSvc,
		Username: *nonExistentUser.Name,
	}

	ctx := context.Background()
	user, err := GetUser(ctx, opts)
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
	if user != nil {
		t.Fatal("expected no user to be returned, got a user")
	}
	if !mockSvc.getCalled {
		t.Fatal("expected Get to be called")
	}
}

func TestGetUserNilService(t *testing.T) {
	opts := GetUserOptions{
		Service:  nil,
		Username: *existingUser.Name,
	}

	ctx := context.Background()
	user, err := GetUser(ctx, opts)
	if err == nil {
		t.Fatal("expected error when service is nil, got nil")
	}
	if user != nil {
		t.Fatal("expected no user to be returned, got a user")
	}
}

func TestGetUserWithServiceError(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    fmt.Errorf("service error"),
	}

	opts := GetUserOptions{
		Service:  mockSvc,
		Username: *existingUser.Name,
	}

	ctx := context.Background()
	user, err := GetUser(ctx, opts)
	if err == nil {
		t.Fatal("expected error from service, got nil")
	}
	if user != nil {
		t.Fatal("expected no user to be returned, got a user")
	}
	if !mockSvc.getCalled {
		t.Fatal("expected Get to be called")
	}
}
