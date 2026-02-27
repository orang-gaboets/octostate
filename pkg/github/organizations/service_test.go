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
	existingUser = github.User{
		ID:   github.Ptr(int64(67890)),
		Name: github.Ptr("existing-user"),
	}
	nonExistingUser = github.User{
		ID:   github.Ptr(int64(99999)),
		Name: github.Ptr("non-existing-user"),
	}
)

type mockService struct {
	createOrgInvCalled bool
	getCalled          bool
	listMembersCalled  bool
	createOrgInvErr    error
	getErr             error
	listMembersErr     error
	orgName            string
	orgID              int64
	orgDescription     string
	orgReposURL        string
	invitedUserID      int64
	membersRole        string
}

// CreateOrgInvitation is a mock implementation of the CreateOrgInvitation method.
func (m *mockService) CreateOrgInvitation(_ context.Context, org string, invitationOptions *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	m.createOrgInvCalled = true
	if m.createOrgInvErr != nil {
		return nil, nil, m.createOrgInvErr
	}

	if org != *existingOrg.Name {
		return nil, nil, github.ErrNotFound
	}

	if invitationOptions.InviteeID == nil || *invitationOptions.InviteeID != *existingUser.ID {
		return nil, nil, github.ErrNotFound
	}

	m.orgName = org
	m.invitedUserID = *invitationOptions.InviteeID

	invitation := &gh.Invitation{
		ID: github.Ptr(int64(67890)),
	}
	return invitation, nil, nil
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

// ListMembers returns mock members of an organization.
func (m *mockService) ListMembers(_ context.Context, org string, opts *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
	m.listMembersCalled = true
	m.orgName = org
	m.membersRole = opts.Role

	if m.listMembersErr != nil {
		return nil, nil, m.listMembersErr
	}

	if org != *existingOrg.Name {
		return nil, nil, github.ErrNotFound
	}

	users := []*gh.User{
		{Login: existingUser.Name, Name: existingUser.Name, ID: existingUser.ID},
	}

	return users, &gh.Response{NextPage: 0}, nil
}

// Test InviteUser functionality

func TestInviteUserSuccess(t *testing.T) {
	mockSvc := &mockService{
		createOrgInvCalled: false,
		createOrgInvErr:    nil,
	}

	opts := InviteUserOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		UserID:  *existingUser.ID,
	}

	ctx := context.Background()
	err := InviteUser(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to be called, but it was not")
	}
	if mockSvc.orgName != *existingOrg.Name {
		t.Fatalf("expected organization name %s, got %s", *existingOrg.Name, mockSvc.orgName)
	}
	if mockSvc.invitedUserID != *existingUser.ID {
		t.Fatalf("expected invited user ID %d, got %d", *existingUser.ID, mockSvc.invitedUserID)
	}
}

func TestInviteUserNonExistingOrg(t *testing.T) {
	mockSvc := &mockService{
		createOrgInvCalled: false,
		createOrgInvErr:    nil,
	}

	opts := InviteUserOptions{
		Service: mockSvc,
		OrgName: *nonExistingOrg.Name,
		UserID:  *existingUser.ID,
	}

	ctx := context.Background()
	err := InviteUser(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to be called, but it was not")
	}
}

func TestInviteUserNonExistingUser(t *testing.T) {
	mockSvc := &mockService{
		createOrgInvCalled: false,
		createOrgInvErr:    nil,
	}

	opts := InviteUserOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		UserID:  *nonExistingUser.ID,
	}

	ctx := context.Background()
	err := InviteUser(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to be called, but it was not")
	}
}

func TestInviteUserEmptyOrgName(t *testing.T) {
	mockSvc := &mockService{
		createOrgInvCalled: false,
		createOrgInvErr:    nil,
	}

	opts := InviteUserOptions{
		Service: mockSvc,
		OrgName: "",
		UserID:  *existingUser.ID,
	}

	ctx := context.Background()
	err := InviteUser(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to not be called, but it was")
	}
}

func TestInviteUserInvalidUserID(t *testing.T) {
	mockSvc := &mockService{
		createOrgInvCalled: false,
		createOrgInvErr:    nil,
	}

	opts := InviteUserOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		UserID:  -1,
	}

	ctx := context.Background()
	err := InviteUser(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to not be called, but it was")
	}
}

func TestInviteUserNilService(t *testing.T) {
	opts := InviteUserOptions{
		Service: nil,
		OrgName: *existingOrg.Name,
		UserID:  *existingUser.ID,
	}

	ctx := context.Background()
	err := InviteUser(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
}

func TestInviteUserWithServiceError(t *testing.T) {
	mockSvc := &mockService{
		createOrgInvCalled: false,
		createOrgInvErr:    errors.New("service error"),
	}

	opts := InviteUserOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		UserID:  *existingUser.ID,
	}

	ctx := context.Background()
	err := InviteUser(ctx, opts)
	if !errors.Is(err, mockSvc.createOrgInvErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.createOrgInvErr, err)
	}
	if !mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to be called, but it was not")
	}
}

// Test Get functionality

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

// Test ListMembers functionality

func TestListMembersSuccess(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListMembersOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		Role:    MemberRoleAll,
	}

	ctx := context.Background()
	members, err := ListMembers(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.listMembersCalled {
		t.Fatal("expected ListMembers to be called, but it was not")
	}
	if mockSvc.membersRole != string(MemberRoleAll) {
		t.Fatalf("expected role %s, got %s", MemberRoleAll, mockSvc.membersRole)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}
	if *members[0].Name != *existingUser.Name {
		t.Fatalf("expected member name %s, got %s", *existingUser.Name, *members[0].Name)
	}
}

func TestListMembersNonExistingOrg(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListMembersOptions{
		Service: mockSvc,
		OrgName: *nonExistingOrg.Name,
		Role:    MemberRoleAll,
	}

	ctx := context.Background()
	members, err := ListMembers(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.listMembersCalled {
		t.Fatal("expected ListMembers to be called, but it was not")
	}
	if members != nil {
		t.Fatalf("expected no members, got %v", members)
	}
}

func TestListMembersNilService(t *testing.T) {
	opts := ListMembersOptions{
		Service: nil,
		OrgName: *existingOrg.Name,
		Role:    MemberRoleAll,
	}

	ctx := context.Background()
	members, err := ListMembers(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if members != nil {
		t.Fatalf("expected members to be nil, got %v", members)
	}
}

func TestListMembersMissingOrg(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListMembersOptions{
		Service: mockSvc,
		OrgName: "",
		Role:    MemberRoleAll,
	}

	ctx := context.Background()
	_, err := ListMembers(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.listMembersCalled {
		t.Fatal("expected ListMembers to not be called, but it was")
	}
}

func TestListMembersInvalidRole(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListMembersOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		Role:    MemberRole("invalid"),
	}

	ctx := context.Background()
	_, err := ListMembers(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if mockSvc.listMembersCalled {
		t.Fatal("expected ListMembers to not be called, but it was")
	}
}
