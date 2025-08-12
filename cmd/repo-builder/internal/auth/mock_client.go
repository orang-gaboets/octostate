package auth

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// MockClient is a lightweight Client implementation for tests.
type MockClient struct {
	OrganizationsService organizations.Service
	TeamsService         teams.Service
	ReposService         repos.Service
}

// Organizations returns the mock Organizations service.
func (m MockClient) Organizations() organizations.Service {
	return m.OrganizationsService
}

// Repositories returns the mock Repositories service.
func (m MockClient) Repositories() repos.Service {
	return m.ReposService
}

// Teams returns the mock Teams service.
func (m MockClient) Teams() teams.Service {
	return m.TeamsService
}

// MockRepoService is a mock implementation of repos.Service for testing purposes.
type MockRepoService struct{}

// CreateFromTemplate mocks the creation of a repository from a template.
func (MockRepoService) CreateFromTemplate(_ context.Context, _, _ string, _ *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

// Delete mocks the deletion of a repository.
func (MockRepoService) Delete(_ context.Context, _, _ string) (*gh.Response, error) {
	return nil, nil
}

// Edit mocks the editing of a repository.
func (MockRepoService) Edit(_ context.Context, _, _ string, _ *gh.Repository) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

// ReplaceAllTopics mocks the replacement of all topics for a repository.
func (MockRepoService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
	return topics, nil, nil
}

// ListAllTopics mocks the listing of all topics for a repository.
func (MockRepoService) ListAllTopics(_ context.Context, _, _ string) ([]string, *gh.Response, error) {
	return []string{}, nil, nil
}

// MockTeamsService is a mock implementation of teams.Service for testing purposes.
type MockTeamsService struct{}

// CreateTeam mocks the creation of a new team.
func (MockTeamsService) CreateTeam(_ context.Context, _ string, _ gh.NewTeam) (*gh.Team, *gh.Response, error) {
	return &gh.Team{}, nil, nil
}

// DeleteTeamBySlug mocks the deletion of a team by its slug.
func (MockTeamsService) DeleteTeamBySlug(_ context.Context, _, _ string) (*gh.Response, error) {
	return nil, nil
}

// GetTeamBySlug mocks the retrieval of a team by its slug.
func (MockTeamsService) GetTeamBySlug(_ context.Context, _, _ string) (*gh.Team, *gh.Response, error) {
	return &gh.Team{}, nil, nil
}

// MockOrganizationService is a mock implementation of organizations.Service for testing purposes.
type MockOrganizationService struct{}

// Get retrieves organization details by name.
func (MockOrganizationService) Get(_ context.Context, _ string) (*gh.Organization, *gh.Response, error) {
	return &gh.Organization{}, nil, nil
}

func mockNewPATClient(_ context.Context, _ string) Client {
	return MockClient{
		OrganizationsService: MockOrganizationService{},
		ReposService:         MockRepoService{},
		TeamsService:         MockTeamsService{},
	}
}

func mockNewAppClient(_, _ int64, _ string) (Client, error) {
	return MockClient{
		OrganizationsService: MockOrganizationService{},
		ReposService:         MockRepoService{},
		TeamsService:         MockTeamsService{},
	}, nil
}

// PrepareClient sets up the mock clients for testing.
func PrepareClient(t *testing.T) {
	t.Helper()
	t.Cleanup(ResetClients)

	SetNewPATClient(mockNewPATClient)
	SetNewAppClient(mockNewAppClient)
}
