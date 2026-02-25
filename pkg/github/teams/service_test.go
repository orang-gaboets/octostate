package teams

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

var (
	newCreatedTeamID int64 = 1

	existingTeam = github.Team{
		ID:          12345,
		Org:         "existing-org",
		Name:        "existing-team",
		Slug:        "existing-team",
		Description: "An existing team for testing",
		Privacy:     github.TeamPrivacyClosed,
		ParentTeam:  nil,
	}

	nonExistingTeam = github.Team{
		ID:          67890,
		Org:         "non-existing-org",
		Name:        "non-existing-team",
		Slug:        "non-existing-team",
		Description: "A non-existing team for testing",
		Privacy:     github.TeamPrivacyClosed,
		ParentTeam:  nil,
	}

	newTeam = github.Team{
		Org:         "new-org",
		Name:        "new-team",
		Description: "A new team for testing",
		Privacy:     github.TeamPrivacyClosed,
		ParentTeam:  nil,
	}

	newTeamWithParent = github.Team{
		Org:         "existing-org",
		Name:        "new-team-with-parent",
		Description: "A new team with an existing parent team",
		Privacy:     github.TeamPrivacyClosed,
		ParentTeam:  &existingTeam,
	}

	newTeamWithParentNotFound = github.Team{
		Org:         "existing-org",
		Name:        "new-team-with-parent-not-found",
		Description: "A new team with a non-existing parent team",
		Privacy:     github.TeamPrivacyClosed,
		ParentTeam: &github.Team{
			Org:  "existing-org",
			Name: "non-existing-parent-team",
			Slug: "non-existing-parent-team",
		},
	}

	newTeamWithParentOrgMismatch = github.Team{
		Org:         "new-org",
		Name:        "new-team-with-parent-org-mismatch",
		Description: "A new team with parent from different org",
		Privacy:     github.TeamPrivacyClosed,
		ParentTeam:  &existingTeam,
	}
)

type mockService struct {
	createCalled bool
	editCalled   bool
	deleteCalled bool
	getCalled    bool
	listCalled   bool
	createErr    error
	editErr      error
	deleteErr    error
	getErr       error
	listErr      error
	teamOrg      string
	teamName     string
	teamSlug     string
	teamDesc     string
	teamPrivacy  github.TeamPrivacy
	teamParentID *int64
	removeParent bool
}

// CreateTeam implements teams.Service for testing.
func (m *mockService) CreateTeam(_ context.Context, org string, team gh.NewTeam) (*gh.Team, *gh.Response, error) {
	m.createCalled = true
	m.teamOrg = org
	m.teamName = team.Name
	if m.createErr != nil {
		return nil, nil, m.createErr
	}
	if team.Description != nil {
		m.teamDesc = *team.Description
	}
	if team.Privacy != nil {
		m.teamPrivacy = github.TeamPrivacy(*team.Privacy)
	}
	if team.ParentTeamID != nil {
		if *team.ParentTeamID != existingTeam.ID {
			return nil, nil, github.WrapError(github.ErrNotFound, "parent team not found")
		}
		m.teamParentID = team.ParentTeamID
	}
	return &gh.Team{
		ID:          &newCreatedTeamID,
		Name:        &team.Name,
		Description: team.Description,
		Privacy:     team.Privacy,
	}, nil, m.createErr
}

// EditTeamBySlug implements teams.Service for testing.
func (m *mockService) EditTeamBySlug(_ context.Context, org, slug string, team gh.NewTeam, removeParent bool) (*gh.Team, *gh.Response, error) {
	m.editCalled = true
	m.removeParent = removeParent
	m.teamOrg = org
	m.teamSlug = slug
	m.teamName = team.Name
	if m.editErr != nil {
		return nil, nil, m.editErr
	}
	if org != existingTeam.Org || slug != existingTeam.Slug {
		return nil, nil, github.WrapError(github.ErrNotFound, "team not found")
	}
	if team.Description != nil {
		m.teamDesc = *team.Description
	}
	if team.Privacy != nil {
		m.teamPrivacy = github.TeamPrivacy(*team.Privacy)
	}
	m.teamParentID = team.ParentTeamID
	return &gh.Team{
		ID:          &existingTeam.ID,
		Name:        github.Ptr(team.Name),
		Description: team.Description,
		Privacy:     team.Privacy,
	}, nil, nil
}

// DeleteTeamBySlug implements teams.Service for testing.
func (m *mockService) DeleteTeamBySlug(_ context.Context, org, slug string) (*gh.Response, error) {
	m.deleteCalled = true
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	if org != existingTeam.Org || slug != existingTeam.Slug {
		return nil, github.WrapError(github.ErrNotFound, "team not found")
	}
	m.teamOrg = org
	m.teamSlug = slug
	return &gh.Response{}, nil
}

// GetTeamBySlug implements teams.Service for testing.
func (m *mockService) GetTeamBySlug(_ context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
	m.getCalled = true
	if m.getErr != nil {
		return nil, nil, m.getErr
	}
	if org != existingTeam.Org || slug != existingTeam.Slug {
		return nil, nil, github.WrapError(github.ErrNotFound, "team not found")
	}
	m.teamOrg = org
	m.teamSlug = slug
	m.teamName = existingTeam.Name
	m.teamDesc = existingTeam.Description
	m.teamPrivacy = existingTeam.Privacy
	return &gh.Team{
		ID:          &existingTeam.ID,
		Name:        &existingTeam.Name,
		Description: &existingTeam.Description,
		Privacy:     github.Ptr(existingTeam.Privacy.String()),
	}, nil, nil
}

// ListTeams implements teams.Service for testing.
func (m *mockService) ListTeams(_ context.Context, org string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	m.listCalled = true
	if m.listErr != nil {
		return nil, nil, m.listErr
	}
	if org != existingTeam.Org {
		return nil, nil, github.WrapError(github.ErrNotFound, "team not found")
	}
	m.teamOrg = org
	return []*gh.Team{
		{
			ID:          &existingTeam.ID,
			Name:        &existingTeam.Name,
			Description: &existingTeam.Description,
			Privacy:     github.Ptr(existingTeam.Privacy.String()),
		},
	}, &gh.Response{NextPage: 0}, nil
}

// Test CreateTeam functionality

func TestCreateTeamNoParentSuccess(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		getCalled:    false,
		createErr:    nil,
		getErr:       nil,
	}

	opts := CreateTeamOptions{
		Service:        mockSvc,
		Name:           newTeam.Name,
		Org:            newTeam.Org,
		Description:    &newTeam.Description,
		Privacy:        &newTeam.Privacy,
		ParentTeamSlug: nil,
	}

	ctx := context.Background()
	_, err := CreateTeam(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mockSvc.createCalled {
		t.Fatal("expected CreateTeam to be called")
	}
	if mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug not to be called")
	}
	if mockSvc.teamParentID != nil {
		t.Fatalf("expected no parent team ID, got: %d", mockSvc.teamParentID)
	}
	if mockSvc.teamDesc != newTeam.Description {
		t.Fatalf("expected team description to match, got: %s", mockSvc.teamDesc)
	}
	if mockSvc.teamPrivacy != newTeam.Privacy {
		t.Fatalf("expected team privacy to match, got: %s", mockSvc.teamPrivacy)
	}
	if mockSvc.teamOrg != newTeam.Org {
		t.Fatalf("expected team org to match, got: %s", mockSvc.teamOrg)
	}
	if mockSvc.teamName != newTeam.Name {
		t.Fatalf("expected team name to match, got: %s", mockSvc.teamName)
	}
}

func TestCreateTeamWithParentSuccess(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		getCalled:    false,
		createErr:    nil,
		getErr:       nil,
	}

	opts := CreateTeamOptions{
		Service:        mockSvc,
		Name:           newTeamWithParent.Name,
		Org:            newTeamWithParent.Org,
		Description:    &newTeamWithParent.Description,
		Privacy:        &newTeamWithParent.Privacy,
		ParentTeamSlug: &newTeamWithParent.ParentTeam.Slug,
	}

	ctx := context.Background()
	_, err := CreateTeam(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mockSvc.createCalled {
		t.Fatal("expected CreateTeam to be called")
	}
	if !mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug not to be called")
	}
	if mockSvc.teamParentID == nil || *mockSvc.teamParentID != existingTeam.ID {
		t.Fatalf("expected parent team ID to match, got: %d", mockSvc.teamParentID)
	}
	if mockSvc.teamDesc != newTeamWithParent.Description {
		t.Fatalf("expected team description to match, got: %s", mockSvc.teamDesc)
	}
	if mockSvc.teamPrivacy != newTeamWithParent.Privacy {
		t.Fatalf("expected team privacy to match, got: %s", mockSvc.teamPrivacy)
	}
	if mockSvc.teamOrg != newTeamWithParent.Org {
		t.Fatalf("expected team org to match, got: %s", mockSvc.teamOrg)
	}
	if mockSvc.teamName != newTeamWithParent.Name {
		t.Fatalf("expected team name to match, got: %s", mockSvc.teamName)
	}
}

func TestCreateTeamWithParentNotFound(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		getCalled:    false,
		createErr:    nil,
		getErr:       nil,
	}

	opts := CreateTeamOptions{
		Service:        mockSvc,
		Name:           newTeamWithParentNotFound.Name,
		Org:            newTeamWithParentNotFound.Org,
		Description:    &newTeamWithParentNotFound.Description,
		Privacy:        &newTeamWithParentNotFound.Privacy,
		ParentTeamSlug: &newTeamWithParentNotFound.ParentTeam.Slug,
	}

	ctx := context.Background()
	_, err := CreateTeam(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug to be called")
	}
	if mockSvc.createCalled {
		t.Fatal("expected CreateTeam not to be called")
	}
}

func TestCreateTeamServiceNil(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		getCalled:    false,
	}

	opts := CreateTeamOptions{
		Service:        nil,
		Name:           newTeam.Name,
		Org:            newTeam.Org,
		Description:    &newTeam.Description,
		Privacy:        &newTeam.Privacy,
		ParentTeamSlug: nil,
	}

	ctx := context.Background()
	_, err := CreateTeam(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug not to be called")
	}
	if mockSvc.createCalled {
		t.Fatal("expected CreateTeam not to be called")
	}
}

func TestCreateTeamServiceErrorWithNoGet(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		getCalled:    false,
		createErr:    errors.New("service error"),
	}

	opts := CreateTeamOptions{
		Service:        mockSvc,
		Name:           newTeam.Name,
		Org:            newTeam.Org,
		Description:    &newTeam.Description,
		Privacy:        &newTeam.Privacy,
		ParentTeamSlug: nil,
	}

	ctx := context.Background()
	_, err := CreateTeam(ctx, opts)
	if !errors.Is(err, mockSvc.createErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.createErr, err)
	}
	if !mockSvc.createCalled {
		t.Fatal("expected CreateTeam to be called")
	}
	if mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug to be called")
	}
}

func TestCreateTeamServiceErrorWithGet(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		getCalled:    false,
		createErr:    errors.New("service error"),
		getErr:       nil,
	}

	opts := CreateTeamOptions{
		Service:        mockSvc,
		Name:           newTeamWithParent.Name,
		Org:            newTeamWithParent.Org,
		ParentTeamSlug: &newTeamWithParent.ParentTeam.Slug,
		Description:    &newTeamWithParent.Description,
		Privacy:        &newTeamWithParent.Privacy,
	}

	ctx := context.Background()
	_, err := CreateTeam(ctx, opts)
	if !errors.Is(err, mockSvc.createErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.createErr, err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug to be called")
	}
	if !mockSvc.createCalled {
		t.Fatal("expected CreateTeam to be called")
	}
}

func TestCreateTeamGetServiceError(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		getCalled:    false,
		createErr:    nil,
		getErr:       errors.New("service error"),
	}

	opts := CreateTeamOptions{
		Service:        mockSvc,
		Name:           newTeamWithParent.Name,
		Org:            newTeamWithParent.Org,
		ParentTeamSlug: &newTeamWithParent.ParentTeam.Slug,
		Description:    &newTeamWithParent.Description,
		Privacy:        &newTeamWithParent.Privacy,
	}

	ctx := context.Background()
	_, err := CreateTeam(ctx, opts)
	if !errors.Is(err, mockSvc.getErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.getErr, err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug to be called")
	}
	if mockSvc.createCalled {
		t.Fatal("expected CreateTeam not to be called")
	}
}

// Test DeleteTeamBySlug functionality

func TestDeleteTeamBySlugSuccess(t *testing.T) {
	mockSvc := &mockService{
		deleteCalled: false,
		deleteErr:    nil,
	}

	opts := DeleteTeamBySlugOptions{
		Service: mockSvc,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	}

	ctx := context.Background()
	err := DeleteTeamBySlug(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mockSvc.deleteCalled {
		t.Fatal("expected DeleteTeamBySlug to be called")
	}
	if mockSvc.teamOrg != existingTeam.Org {
		t.Fatalf("expected team org to match, got: %s", mockSvc.teamOrg)
	}
	if mockSvc.teamSlug != existingTeam.Slug {
		t.Fatalf("expected team slug to match, got: %s", mockSvc.teamSlug)
	}
}

func TestDeleteTeamBySlugNotFound(t *testing.T) {
	mockSvc := &mockService{
		deleteCalled: false,
		deleteErr:    nil,
	}

	opts := DeleteTeamBySlugOptions{
		Service: mockSvc,
		Org:     nonExistingTeam.Org,
		Slug:    nonExistingTeam.Slug,
	}

	ctx := context.Background()
	err := DeleteTeamBySlug(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.deleteCalled {
		t.Fatal("expected DeleteTeamBySlug to be called")
	}
}

func TestDeleteTeamBySlugServiceNil(t *testing.T) {
	mockSvc := &mockService{
		deleteCalled: false,
	}

	opts := DeleteTeamBySlugOptions{
		Service: nil,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	}

	ctx := context.Background()
	err := DeleteTeamBySlug(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if mockSvc.deleteCalled {
		t.Fatal("expected DeleteTeamBySlug not to be called")
	}
}

func TestDeleteTeamBySlugServiceError(t *testing.T) {
	mockSvc := &mockService{
		deleteCalled: false,
		deleteErr:    errors.New("service error"),
	}

	opts := DeleteTeamBySlugOptions{
		Service: mockSvc,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	}

	ctx := context.Background()
	err := DeleteTeamBySlug(ctx, opts)
	if !errors.Is(err, mockSvc.deleteErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.deleteErr, err)
	}
	if !mockSvc.deleteCalled {
		t.Fatal("expected DeleteTeamBySlug to be called")
	}
}

// Test GetTeamBySlug functionality

func TestGetTeamBySlugSuccess(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetTeamBySlugOptions{
		Service: mockSvc,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	}

	ctx := context.Background()
	_, err := GetTeamBySlug(ctx, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug to be called")
	}
	if mockSvc.teamOrg != existingTeam.Org {
		t.Fatalf("expected team org to match, got: %s", mockSvc.teamOrg)
	}
	if mockSvc.teamSlug != existingTeam.Slug {
		t.Fatalf("expected team slug to match, got: %s", mockSvc.teamSlug)
	}
	if mockSvc.teamName != existingTeam.Name {
		t.Fatalf("expected team name to match, got: %s", mockSvc.teamName)
	}
	if mockSvc.teamDesc != existingTeam.Description {
		t.Fatalf("expected team description to match, got: %s", mockSvc.teamDesc)
	}
	if mockSvc.teamPrivacy != existingTeam.Privacy {
		t.Fatalf("expected team privacy to match, got: %s", mockSvc.teamPrivacy)
	}
}

func TestGetTeamBySlugNotFound(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    nil,
	}

	opts := GetTeamBySlugOptions{
		Service: mockSvc,
		Org:     nonExistingTeam.Org,
		Slug:    nonExistingTeam.Slug,
	}

	ctx := context.Background()
	_, err := GetTeamBySlug(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug to be called")
	}
}

func TestGetTeamBySlugServiceNil(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
	}

	opts := GetTeamBySlugOptions{
		Service: nil,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	}

	ctx := context.Background()
	_, err := GetTeamBySlug(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug not to be called")
	}
}

func TestGetTeamBySlugServiceError(t *testing.T) {
	mockSvc := &mockService{
		getCalled: false,
		getErr:    errors.New("service error"),
	}

	opts := GetTeamBySlugOptions{
		Service: mockSvc,
		Org:     existingTeam.Org,
		Slug:    existingTeam.Slug,
	}

	ctx := context.Background()
	_, err := GetTeamBySlug(ctx, opts)
	if !errors.Is(err, mockSvc.getErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.getErr, err)
	}
	if !mockSvc.getCalled {
		t.Fatal("expected GetTeamBySlug to be called")
	}
}

// Test ListTeams functionality

func TestListTeamsSuccess(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListTeamsOptions{
		Service: mockSvc,
		Org:     existingTeam.Org,
	}

	ctx := context.Background()
	teams, err := ListTeams(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.listCalled {
		t.Fatal("expected ListTeams to be called")
	}
	if mockSvc.teamOrg != existingTeam.Org {
		t.Fatalf("expected org %s, got %s", existingTeam.Org, mockSvc.teamOrg)
	}
	if len(teams) != 1 {
		t.Fatalf("expected 1 team, got %d", len(teams))
	}
	if teams[0].Name != existingTeam.Name {
		t.Fatalf("expected team name %s, got %s", existingTeam.Name, teams[0].Name)
	}
}

func TestListTeamsNonExistingOrg(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListTeamsOptions{
		Service: mockSvc,
		Org:     nonExistingTeam.Org,
	}

	ctx := context.Background()
	teams, err := ListTeams(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.listCalled {
		t.Fatal("expected ListTeams to be called")
	}
	if teams != nil {
		t.Fatalf("expected no teams, got %v", teams)
	}
}

func TestListTeamsNilService(t *testing.T) {
	opts := ListTeamsOptions{
		Service: nil,
		Org:     existingTeam.Org,
	}

	ctx := context.Background()
	teams, err := ListTeams(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
	if teams != nil {
		t.Fatalf("expected teams to be nil, got %v", teams)
	}
}

func TestListTeamsMissingOrg(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListTeamsOptions{
		Service: mockSvc,
		Org:     "",
	}

	ctx := context.Background()
	_, err := ListTeams(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.listCalled {
		t.Fatal("expected ListTeams not to be called")
	}
}

func TestListTeamsServiceError(t *testing.T) {
	mockSvc := &mockService{
		listErr: errors.New("service error"),
	}

	opts := ListTeamsOptions{
		Service: mockSvc,
		Org:     existingTeam.Org,
	}

	ctx := context.Background()
	teams, err := ListTeams(ctx, opts)
	if !errors.Is(err, mockSvc.listErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.listErr, err)
	}
	if !mockSvc.listCalled {
		t.Fatal("expected ListTeams to be called")
	}
	if teams != nil {
		t.Fatalf("expected teams to be nil, got %v", teams)
	}
}
