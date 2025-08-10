package auth

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
)

// MockClient is a lightweight Client implementation for tests.
type MockClient struct {
	TeamsService teams.Service
	RepoService  repos.Service
}

func (m MockClient) Repositories() repos.Service {
	return m.RepoService
}

func (m MockClient) Teams() teams.Service {
	return m.TeamsService
}

// MockRepoService is a mock implementation of repos.Service for testing purposes.
type MockRepoService struct{}

func (MockRepoService) CreateFromTemplate(_ context.Context, _, _ string, _ *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

func (MockRepoService) Delete(_ context.Context, _, _ string) (*gh.Response, error) {
	return nil, nil
}

func (MockRepoService) Edit(_ context.Context, _, _ string, _ *gh.Repository) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

func (MockRepoService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
	return topics, nil, nil
}

func (MockRepoService) ListAllTopics(_ context.Context, _, _ string) ([]string, *gh.Response, error) {
	return []string{}, nil, nil
}

// MockTeamsService is a mock implementation of teams.Service for testing purposes.
type MockTeamsService struct{}

func (MockTeamsService) CreateTeam(_ context.Context, _ string, _ gh.NewTeam) (*gh.Team, *gh.Response, error) {
	return &gh.Team{}, nil, nil
}

func (MockTeamsService) DeleteTeamBySlug(_ context.Context, _, _ string) (*gh.Response, error) {
	return nil, nil
}

func (MockTeamsService) GetTeamBySlug(_ context.Context, _, _ string) (*gh.Team, *gh.Response, error) {
	return &gh.Team{}, nil, nil
}

func mockNewPATClient(_ context.Context, _ string) Client {
	return MockClient{
		RepoService:  MockRepoService{},
		TeamsService: MockTeamsService{},
	}
}

func mockNewAppClient(_, _ int64, _ string) (Client, error) {
	return MockClient{
		RepoService:  MockRepoService{},
		TeamsService: MockTeamsService{},
	}, nil
}

func PrepareClient(t *testing.T) {
	t.Helper()
	t.Cleanup(ResetClients)

	SetNewPATClient(mockNewPATClient)
	SetNewAppClient(mockNewAppClient)
}
