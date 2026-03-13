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
	existingInviteEmail = "invitee@example.com"
)

type mockService struct {
	createOrgInvCalled bool
	getCalled          bool
	listMembersCalled  bool
	listPendingCalled  bool
	listInviteTeams    bool
	createOrgInvErr    error
	getErr             error
	listMembersErr     error
	listPendingErr     error
	listInviteTeamsErr error
	orgName            string
	orgID              int64
	orgDescription     string
	orgReposURL        string
	invitedUserID      int64
	invitedEmail       string
	invitedRole        string
	invitedTeamIDs     []int64
	membersRole        string
	invitationID       string
	listOptionsPage    int
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

	userIDProvided := invitationOptions.InviteeID != nil
	emailProvided := invitationOptions.Email != nil
	switch {
	case userIDProvided && emailProvided:
		return nil, nil, github.ErrValidationFailed
	case userIDProvided:
		if *invitationOptions.InviteeID != *existingUser.ID {
			return nil, nil, github.ErrNotFound
		}
		m.invitedUserID = *invitationOptions.InviteeID
	case emailProvided:
		if *invitationOptions.Email != existingInviteEmail {
			return nil, nil, github.ErrNotFound
		}
		m.invitedEmail = *invitationOptions.Email
	default:
		return nil, nil, github.ErrMissingRequiredField
	}

	m.orgName = org
	if invitationOptions.Role != nil {
		m.invitedRole = *invitationOptions.Role
	}
	m.invitedTeamIDs = append([]int64(nil), invitationOptions.TeamID...)

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

// ListPendingOrgInvitations returns mock pending invitations of an organization.
func (m *mockService) ListPendingOrgInvitations(_ context.Context, org string, opts *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
	m.listPendingCalled = true
	m.orgName = org
	if opts != nil {
		m.listOptionsPage = opts.Page
	}

	if m.listPendingErr != nil {
		return nil, nil, m.listPendingErr
	}

	if org != *existingOrg.Name {
		return nil, nil, github.ErrNotFound
	}

	if opts != nil && opts.Page == 2 {
		return []*gh.Invitation{
			{
				ID:    github.Ptr(int64(2)),
				Login: github.Ptr("second"),
				Role:  github.Ptr("direct_member"),
			},
		}, &gh.Response{NextPage: 0}, nil
	}

	return []*gh.Invitation{
		{
			ID:                github.Ptr(int64(1)),
			Login:             github.Ptr("monalisa"),
			Email:             github.Ptr("octocat@example.com"),
			Role:              github.Ptr("direct_member"),
			TeamCount:         github.Ptr(2),
			InvitationTeamURL: github.Ptr("https://api.github.com/organizations/1/invitations/1/teams"),
		},
	}, &gh.Response{NextPage: 2}, nil
}

// ListOrgInvitationTeams returns mock teams attached to an organization invitation.
func (m *mockService) ListOrgInvitationTeams(_ context.Context, org, invitationID string, opts *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	m.listInviteTeams = true
	m.orgName = org
	m.invitationID = invitationID
	if opts != nil {
		m.listOptionsPage = opts.Page
	}

	if m.listInviteTeamsErr != nil {
		return nil, nil, m.listInviteTeamsErr
	}

	if org != *existingOrg.Name {
		return nil, nil, github.ErrNotFound
	}

	if invitationID != "22" {
		return nil, nil, github.ErrNotFound
	}

	if opts != nil && opts.Page == 2 {
		return []*gh.Team{
			{
				ID:           github.Ptr(int64(2)),
				Slug:         github.Ptr("docs"),
				Name:         github.Ptr("Docs"),
				Organization: &gh.Organization{Login: existingOrg.Name},
			},
		}, &gh.Response{NextPage: 0}, nil
	}

	return []*gh.Team{
		{
			ID:           github.Ptr(int64(1)),
			Slug:         github.Ptr("platform"),
			Name:         github.Ptr("Platform"),
			Organization: &gh.Organization{Login: existingOrg.Name},
		},
	}, &gh.Response{NextPage: 2}, nil
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

func TestCreateInvitationByUserIDWithRoleAndTeams(t *testing.T) {
	mockSvc := &mockService{}
	userID := *existingUser.ID

	opts := CreateInvitationOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		UserID:  &userID,
		Role:    "direct_member",
		TeamIDs: []int64{11, 22},
	}

	err := CreateInvitation(context.Background(), opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to be called")
	}
	if mockSvc.invitedUserID != userID {
		t.Fatalf("expected invited user ID %d, got %d", userID, mockSvc.invitedUserID)
	}
	if mockSvc.invitedEmail != "" {
		t.Fatalf("expected no invited email, got %q", mockSvc.invitedEmail)
	}
	if mockSvc.invitedRole != "direct_member" {
		t.Fatalf("expected role %q, got %q", "direct_member", mockSvc.invitedRole)
	}
	if !equalInt64Slices(mockSvc.invitedTeamIDs, []int64{11, 22}) {
		t.Fatalf("expected team IDs %v, got %v", []int64{11, 22}, mockSvc.invitedTeamIDs)
	}
}

func TestCreateInvitationByEmail(t *testing.T) {
	mockSvc := &mockService{}

	opts := CreateInvitationOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		Email:   existingInviteEmail,
	}

	err := CreateInvitation(context.Background(), opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to be called")
	}
	if mockSvc.invitedEmail != existingInviteEmail {
		t.Fatalf("expected invited email %q, got %q", existingInviteEmail, mockSvc.invitedEmail)
	}
	if mockSvc.invitedUserID != 0 {
		t.Fatalf("expected no invited user ID, got %d", mockSvc.invitedUserID)
	}
}

func TestCreateInvitationRejectsMissingIdentity(t *testing.T) {
	mockSvc := &mockService{}
	opts := CreateInvitationOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
	}

	err := CreateInvitation(context.Background(), opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation not to be called")
	}
}

func TestCreateInvitationRejectsConflictingIdentity(t *testing.T) {
	mockSvc := &mockService{}
	userID := *existingUser.ID
	opts := CreateInvitationOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
		UserID:  &userID,
		Email:   existingInviteEmail,
	}

	err := CreateInvitation(context.Background(), opts)
	if !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
	if mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation not to be called")
	}
}

func TestCreateInvitationTrimsWhitespaceFields(t *testing.T) {
	mockSvc := &mockService{}
	userID := *existingUser.ID

	opts := CreateInvitationOptions{
		Service: mockSvc,
		OrgName: "  " + *existingOrg.Name + "  ",
		UserID:  &userID,
		Email:   "   ",
		Role:    " direct_member ",
	}

	err := CreateInvitation(context.Background(), opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.createOrgInvCalled {
		t.Fatal("expected CreateOrgInvitation to be called")
	}
	if mockSvc.orgName != *existingOrg.Name {
		t.Fatalf("expected trimmed organization name %q, got %q", *existingOrg.Name, mockSvc.orgName)
	}
	if mockSvc.invitedEmail != "" {
		t.Fatalf("expected trimmed empty email, got %q", mockSvc.invitedEmail)
	}
	if mockSvc.invitedRole != "direct_member" {
		t.Fatalf("expected trimmed role %q, got %q", "direct_member", mockSvc.invitedRole)
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

func equalInt64Slices(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

func TestListPendingInvitationsSuccess(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListPendingInvitationsOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
	}

	ctx := context.Background()
	invitations, err := ListPendingInvitations(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.listPendingCalled {
		t.Fatal("expected ListPendingOrgInvitations to be called, but it was not")
	}
	if mockSvc.orgName != *existingOrg.Name {
		t.Fatalf("expected organization name %s, got %s", *existingOrg.Name, mockSvc.orgName)
	}
	if len(invitations) != 2 {
		t.Fatalf("expected 2 invitations, got %d", len(invitations))
	}
	if invitations[0].Login == nil || *invitations[0].Login != "monalisa" {
		t.Fatalf("unexpected first invitation: %#v", invitations[0])
	}
	if invitations[1].Login == nil || *invitations[1].Login != "second" {
		t.Fatalf("unexpected second invitation: %#v", invitations[1])
	}
}

func TestListPendingInvitationsNonExistingOrg(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListPendingInvitationsOptions{
		Service: mockSvc,
		OrgName: *nonExistingOrg.Name,
	}

	ctx := context.Background()
	invitations, err := ListPendingInvitations(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.listPendingCalled {
		t.Fatal("expected ListPendingOrgInvitations to be called, but it was not")
	}
	if invitations != nil {
		t.Fatalf("expected invitations to be nil, got %#v", invitations)
	}
}

func TestListPendingInvitationsNilService(t *testing.T) {
	opts := ListPendingInvitationsOptions{
		Service: nil,
		OrgName: *existingOrg.Name,
	}

	ctx := context.Background()
	invitations, err := ListPendingInvitations(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if invitations != nil {
		t.Fatalf("expected invitations to be nil, got %#v", invitations)
	}
}

func TestListPendingInvitationsWithServiceError(t *testing.T) {
	serviceErr := errors.New("service error")
	mockSvc := &mockService{
		listPendingErr: serviceErr,
	}

	opts := ListPendingInvitationsOptions{
		Service: mockSvc,
		OrgName: *existingOrg.Name,
	}

	ctx := context.Background()
	invitations, err := ListPendingInvitations(ctx, opts)
	if !errors.Is(err, serviceErr) {
		t.Fatalf("expected error %v, got %v", serviceErr, err)
	}
	if !mockSvc.listPendingCalled {
		t.Fatal("expected ListPendingOrgInvitations to be called, but it was not")
	}
	if invitations != nil {
		t.Fatalf("expected invitations to be nil, got %#v", invitations)
	}
}

func TestListInvitationTeamsSuccess(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListInvitationTeamsOptions{
		Service:      mockSvc,
		OrgName:      *existingOrg.Name,
		InvitationID: 22,
	}

	ctx := context.Background()
	teams, err := ListInvitationTeams(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.listInviteTeams {
		t.Fatal("expected ListOrgInvitationTeams to be called, but it was not")
	}
	if mockSvc.invitationID != "22" {
		t.Fatalf("expected invitation ID 22, got %s", mockSvc.invitationID)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams, got %d", len(teams))
	}
	if teams[0].Slug != "platform" {
		t.Fatalf("unexpected first team: %#v", teams[0])
	}
	if teams[1].Slug != "docs" {
		t.Fatalf("unexpected second team: %#v", teams[1])
	}
}

func TestListInvitationTeamsNonExistingInvitation(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListInvitationTeamsOptions{
		Service:      mockSvc,
		OrgName:      *existingOrg.Name,
		InvitationID: 999,
	}

	ctx := context.Background()
	teams, err := ListInvitationTeams(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.listInviteTeams {
		t.Fatal("expected ListOrgInvitationTeams to be called, but it was not")
	}
	if teams != nil {
		t.Fatalf("expected teams to be nil, got %#v", teams)
	}
}

func TestListInvitationTeamsInvalidInvitationID(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListInvitationTeamsOptions{
		Service:      mockSvc,
		OrgName:      *existingOrg.Name,
		InvitationID: 0,
	}

	ctx := context.Background()
	teams, err := ListInvitationTeams(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.listInviteTeams {
		t.Fatal("expected ListOrgInvitationTeams to not be called, but it was")
	}
	if teams != nil {
		t.Fatalf("expected teams to be nil, got %#v", teams)
	}
}

func TestListInvitationTeamsWithServiceError(t *testing.T) {
	serviceErr := errors.New("service error")
	mockSvc := &mockService{
		listInviteTeamsErr: serviceErr,
	}

	opts := ListInvitationTeamsOptions{
		Service:      mockSvc,
		OrgName:      *existingOrg.Name,
		InvitationID: 22,
	}

	ctx := context.Background()
	teams, err := ListInvitationTeams(ctx, opts)
	if !errors.Is(err, serviceErr) {
		t.Fatalf("expected error %v, got %v", serviceErr, err)
	}
	if !mockSvc.listInviteTeams {
		t.Fatal("expected ListOrgInvitationTeams to be called, but it was not")
	}
	if teams != nil {
		t.Fatalf("expected teams to be nil, got %#v", teams)
	}
}
