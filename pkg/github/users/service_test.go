package users

import (
	"context"
	"errors"
	"fmt"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/github"
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
		ID:   github.Ptr(int64(99999)),
	}
)

type mockService struct {
	getCalled     bool
	getByIDCalled bool
	getErr        error
	getByIDErr    error
	userName      string
	userID        *int64
	userEmail     *string
	userURL       *string
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

func (m *mockService) GetByID(_ context.Context, id int64) (*gh.User, *gh.Response, error) {
	m.getByIDCalled = true
	if m.getByIDErr != nil {
		return nil, nil, m.getByIDErr
	}
	if id != *existingUser.ID {
		return nil, nil, fmt.Errorf("user with ID %d not found: %w", id, github.ErrNotFound)
	}
	m.userID = &id
	m.userName = *existingUser.Name
	m.userEmail = existingUser.Email
	m.userURL = existingUser.URL
	return &gh.User{}, nil, nil
}

// Test GetUserByID functionality

func TestGetUserByIDSuccess(t *testing.T) {
	mockSvc := &mockService{
		getByIDCalled: false,
		getByIDErr:    nil,
	}

	opts := GetUserByIDOptions{
		Service: mockSvc,
		ID:      *existingUser.ID,
	}

	ctx := context.Background()
	user, err := GetUserByID(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.getByIDCalled {
		t.Fatal("expected GetByID to be called")
	}
	if user == nil {
		t.Fatal("expected user to be returned, got nil")
	}
	if mockSvc.userName != *existingUser.Name {
		t.Fatalf("expected username %s, got %s", *existingUser.Name, mockSvc.userName)
	}
	if *mockSvc.userID != *existingUser.ID {
		t.Fatalf("expected user ID %d, got %d", *existingUser.ID, *mockSvc.userID)
	}
	if mockSvc.userEmail != existingUser.Email {
		t.Fatalf("expected user email %s, got %s", *existingUser.Email, *mockSvc.userEmail)
	}
	if mockSvc.userURL != existingUser.URL {
		t.Fatalf("expected user URL %s, got %s", *existingUser.URL, *mockSvc.userURL)
	}
}

func TestGetUserByIDNonExistentUser(t *testing.T) {
	mockSvc := &mockService{
		getByIDCalled: false,
		getByIDErr:    nil,
	}

	opts := GetUserByIDOptions{
		Service: mockSvc,
		ID:      *nonExistentUser.ID,
	}

	ctx := context.Background()
	user, err := GetUserByID(ctx, opts)
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
	if user != nil {
		t.Fatal("expected no user to be returned, got a user")
	}
	if !mockSvc.getByIDCalled {
		t.Fatal("expected GetByID to be called")
	}
}

func TestGetUserByIDNilService(t *testing.T) {
	opts := GetUserByIDOptions{
		Service: nil,
		ID:      *existingUser.ID,
	}

	ctx := context.Background()
	user, err := GetUserByID(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if user != nil {
		t.Fatal("expected no user to be returned, got a user")
	}
}

func TestGetUserByIDWithServiceError(t *testing.T) {
	mockSvc := &mockService{
		getByIDCalled: false,
		getByIDErr:    fmt.Errorf("service error"),
	}

	opts := GetUserByIDOptions{
		Service: mockSvc,
		ID:      *existingUser.ID,
	}

	ctx := context.Background()
	user, err := GetUserByID(ctx, opts)
	if !errors.Is(err, mockSvc.getByIDErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.getByIDErr, err)
	}
	if user != nil {
		t.Fatal("expected no user to be returned, got a user")
	}
	if !mockSvc.getByIDCalled {
		t.Fatal("expected GetByID to be called")
	}
}

// Test GetUserByUsername functionality

func TestGetUserByUsernameSuccess(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetUserByUsernameOptions{
		Service:  mockSvc,
		Username: *existingUser.Name,
	}

	ctx := context.Background()
	user, err := GetUserByUsername(ctx, opts)
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

func TestGetUserByUsernameNonExistentUser(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetUserByUsernameOptions{
		Service:  mockSvc,
		Username: *nonExistentUser.Name,
	}

	ctx := context.Background()
	user, err := GetUserByUsername(ctx, opts)
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

func TestGetUserByUsernameNilService(t *testing.T) {
	opts := GetUserByUsernameOptions{
		Service:  nil,
		Username: *existingUser.Name,
	}

	ctx := context.Background()
	user, err := GetUserByUsername(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if user != nil {
		t.Fatal("expected no user to be returned, got a user")
	}
}

func TestGetUserByUsernameWithServiceError(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    fmt.Errorf("service error"),
	}

	opts := GetUserByUsernameOptions{
		Service:  mockSvc,
		Username: *existingUser.Name,
	}

	ctx := context.Background()
	user, err := GetUserByUsername(ctx, opts)
	if !errors.Is(err, mockSvc.getErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.getErr, err)
	}
	if user != nil {
		t.Fatal("expected no user to be returned, got a user")
	}
	if !mockSvc.getCalled {
		t.Fatal("expected Get to be called")
	}
}
