package auth

import (
	"context"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/github/organizations"
	"github.com/orang-gaboets/repo-builder/pkg/github/repos"
	"github.com/orang-gaboets/repo-builder/pkg/github/teams"
	"github.com/orang-gaboets/repo-builder/pkg/github/users"
)

// MockClient is a lightweight Client implementation for tests.
type MockClient struct {
	OrganizationsService organizations.Service
	TeamsService         teams.Service
	ReposService         repos.Service
	UsersService         users.Service
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

// Users returns the mock Users service.
func (m MockClient) Users() users.Service {
	return m.UsersService
}

// MockOrganizationService is a mock implementation of organizations.Service for testing purposes.
type MockOrganizationService struct{}

// CreateOrgInvitation mocks the creation of an organization invitation.
func (MockOrganizationService) CreateOrgInvitation(_ context.Context, _ string, _ *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	return &gh.Invitation{}, nil, nil
}

// Get mocks the retrieval organization details by name.
func (MockOrganizationService) Get(_ context.Context, _ string) (*gh.Organization, *gh.Response, error) {
	return &gh.Organization{}, nil, nil
}

// ListMembers mocks the listing of organization members.
func (MockOrganizationService) ListMembers(_ context.Context, _ string, _ *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
	return []*gh.User{}, nil, nil
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

// Get mocks the retrieval of a repository.
func (MockRepoService) Get(_ context.Context, _, _ string) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

// ListByOrg mocks the listing of repositories for an organization.
func (MockRepoService) ListByOrg(_ context.Context, _ string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
	return []*gh.Repository{}, nil, nil
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

// EditTeamBySlug mocks the editing of a team by slug.
func (MockTeamsService) EditTeamBySlug(_ context.Context, _, _ string, team gh.NewTeam, _ bool) (*gh.Team, *gh.Response, error) {
	return &gh.Team{
		Name:        github.Ptr(team.Name),
		Description: team.Description,
		Privacy:     team.Privacy,
	}, nil, nil
}

// DeleteTeamBySlug mocks the deletion of a team by its slug.
func (MockTeamsService) DeleteTeamBySlug(_ context.Context, _, _ string) (*gh.Response, error) {
	return nil, nil
}

// GetTeamBySlug mocks the retrieval of a team by its slug.
func (MockTeamsService) GetTeamBySlug(_ context.Context, _, _ string) (*gh.Team, *gh.Response, error) {
	return &gh.Team{}, nil, nil
}

// AddTeamMembershipBySlug mocks adding or updating a team membership by team slug.
func (MockTeamsService) AddTeamMembershipBySlug(_ context.Context, _, _, _ string, opts *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
	role := "member"
	if opts != nil && opts.Role != "" {
		role = opts.Role
	}
	return &gh.Membership{
		State: github.Ptr("active"),
		Role:  github.Ptr(role),
	}, nil, nil
}

// RemoveTeamMembershipBySlug mocks removing a team membership by team slug.
func (MockTeamsService) RemoveTeamMembershipBySlug(_ context.Context, _, _, _ string) (*gh.Response, error) {
	return &gh.Response{}, nil
}

// ListTeamReposBySlug mocks listing repositories accessible by a team.
func (MockTeamsService) ListTeamReposBySlug(_ context.Context, _, _ string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
	return []*gh.Repository{}, &gh.Response{NextPage: 0}, nil
}

// ListTeamMembersBySlug mocks listing members of a team by slug.
func (MockTeamsService) ListTeamMembersBySlug(_ context.Context, _, _ string, _ *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
	return []*gh.User{}, nil, nil
}

// ListTeams mocks the listing of teams in an organization.
func (MockTeamsService) ListTeams(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	return []*gh.Team{}, nil, nil
}

// MockUserService is a mock implementation of users.Service for testing purposes.
type MockUserService struct{}

// Get mocks the retrieval of user details by username.
func (MockUserService) Get(_ context.Context, _ string) (*gh.User, *gh.Response, error) {
	return &gh.User{
		ID: github.Ptr(int64(12345)),
	}, nil, nil
}

// GetByID mocks the retrieval of user details by ID.
func (MockUserService) GetByID(_ context.Context, _ int64) (*gh.User, *gh.Response, error) {
	return &gh.User{}, nil, nil
}

func mockNewPATClient(_ context.Context, _ string) Client {
	return MockClient{
		OrganizationsService: MockOrganizationService{},
		ReposService:         MockRepoService{},
		TeamsService:         MockTeamsService{},
		UsersService:         MockUserService{},
	}
}

func mockNewAppClient(_, _ int64, _ string) (Client, error) {
	return MockClient{
		OrganizationsService: MockOrganizationService{},
		ReposService:         MockRepoService{},
		TeamsService:         MockTeamsService{},
		UsersService:         MockUserService{},
	}, nil
}

// PrepareClient sets up the mock clients for testing.
func PrepareClient(t *testing.T) {
	t.Helper()
	t.Cleanup(ResetClients)

	SetNewPATClient(mockNewPATClient)
	SetNewAppClient(mockNewAppClient)
}
