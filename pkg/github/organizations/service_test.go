package organizations

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

var (
	existingOrg = github.Organization{
		ID:          github.Ptr(int64(12345)),
		Name:        github.Ptr("existing-org"),
		Description: github.Ptr("This is an existing organization."),
		ReposURL:    github.Ptr("https://api.github.com/orgs/existing-org/repos"),
	}
	nonExistingOrg = github.Organization{
		Name: github.Ptr("non-existing-org"),
	}
)

type mockService struct {
	getCalled      bool
	getErr         error
	orgName        string
	orgID          int64
	orgDescription string
	orgReposURL    string
}

// Get returns a mock organization based on the orgName.
func (m *mockService) Get(_ context.Context, orgName string) (*gh.Organization, *gh.Response, error) {
	m.getCalled = true
	m.orgName = orgName

	if m.getErr != nil {
		return nil, nil, m.getErr
	}

	if orgName != *existingOrg.Name {
		return nil, nil, github.ErrNotFound
	}

	m.orgID = *existingOrg.ID
	m.orgDescription = *existingOrg.Description
	m.orgReposURL = *existingOrg.ReposURL

	return &gh.Organization{}, nil, nil
}

func TestGetSuccess(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
	}

	ctx := context.Background()
	org, err := Get(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected Get to be called, but it was not")
	}
	if org == nil {
		t.Fatal("expected organization to be returned, but it was nil")
	}
	if mockSvc.orgName != *existingOrg.Name {
		t.Fatalf("expected organization name %s, got %s", *existingOrg.Name, mockSvc.orgName)
	}
	if mockSvc.orgID != *existingOrg.ID {
		t.Fatalf("expected organization ID %d, got %d", *existingOrg.ID, mockSvc.orgID)
	}
	if mockSvc.orgDescription != *existingOrg.Description {
		t.Fatalf("expected organization description %s, got %s", *existingOrg.Description, mockSvc.orgDescription)
	}
	if mockSvc.orgReposURL != *existingOrg.ReposURL {
		t.Fatalf("expected organization repos URL %s, got %s", *existingOrg.ReposURL, mockSvc.orgReposURL)
	}
}

func TestGetNonExistingOrg(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetOptions{
		Service: mockSvc,
		OrgName: *nonExistingOrg.Name,
	}

	ctx := context.Background()
	org, err := Get(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected Get to be called, but it was not")
	}
	if org != nil {
		t.Fatal("expected organization to be nil, but it was not")
	}
}

func TestGetWithEmptyOrgName(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetOptions{
		Service: mockSvc,
		OrgName: "",
	}

	ctx := context.Background()
	org, err := Get(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.getCalled {
		t.Fatal("expected Get to not be called, but it was")
	}
	if org != nil {
		t.Fatal("expected organization to be nil, but it was not")
	}
}

func TestGetNilService(t *testing.T) {
	opts := GetOptions{
		Service: nil,
		OrgName: *existingOrg.Name,
	}

	ctx := context.Background()
	org, err := Get(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if org != nil {
		t.Fatal("expected organization to be nil, but it was not")
	}
}

func TestGetWithServiceError(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    errors.New("service error"),
	}

	opts := GetOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
	}

	ctx := context.Background()
	org, err := Get(ctx, opts)
	if !errors.Is(err, mockSvc.getErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.getErr, err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected Get to be called, but it was not")
	}
	if org != nil {
		t.Fatal("expected organization to be nil, but it was not")
	}
}
