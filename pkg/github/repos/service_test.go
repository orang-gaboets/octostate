package repos

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-github/v55/github"
	ggithub "github.com/orang-gaboets/repo-builder/pkg/github"
)

var (
	templateRepo = ggithub.Repository{
		Org:         "template-org",
		Name:        "template-name",
		Description: "template-desc",
		Private:     false,
		Topics:      []string{"template-topic"},
	}

	newRepo = ggithub.Repository{
		Org:         "org",
		Name:        "name",
		Description: "desc",
		Private:     false,
		Topics:      []string{"t1", "t2"},
	}

	invalidTemplateRepo = ggithub.Repository{
		Org:  "invalid-org",
		Name: "invalid-name",
	}

	existingRepo = ggithub.Repository{
		Org:         "existing-org",
		Name:        "existing-name",
		Description: "existing-desc",
	}
)

type mockService struct {
	createCalled  bool
	listCalled    bool
	replaceCalled bool
	listErr       error
	createErr     error
	replaceErr    error
	owner         string
	repoName      string
	repoDesc      string
	repoTopics    []string
	repoPrivate   bool
	templateName  string
	templateOwner string
}

func (m *mockService) CreateFromTemplate(ctx context.Context, owner, repo string, req *github.TemplateRepoRequest) (*github.Repository, *github.Response, error) {
	m.createCalled = true
	m.templateOwner = owner
	m.templateName = repo
	if owner != templateRepo.Org || repo != templateRepo.Name {
		return nil, nil, errors.New("invalid template repository")
	}
	if req != nil && req.Owner != nil && req.Name != nil && *req.Owner == existingRepo.Org && *req.Name == existingRepo.Name {
		return nil, nil, errors.New("repository already exists")
	}
	if req != nil {
		if req.Name != nil {
			m.repoName = *req.Name
		}
		if req.Description != nil {
			m.repoDesc = *req.Description
		}
		if req.Private != nil {
			m.repoPrivate = *req.Private
		}
		if req.Owner != nil {
			m.owner = *req.Owner
		}
	}
	if m.createErr != nil {
		return nil, nil, m.createErr
	}

	return &github.Repository{}, nil, nil
}

func (m *mockService) ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *github.Response, error) {
	m.replaceCalled = true
	m.owner = owner
	m.repoName = repo
	m.repoTopics = topics
	if owner != newRepo.Org || repo != newRepo.Name {
		return nil, nil, errors.New("invalid repository for topic replacement")
	}
	if m.replaceErr != nil {
		return nil, nil, m.replaceErr
	}
	return topics, nil, nil
}

func (m *mockService) ListAllTopics(ctx context.Context, owner, repo string) ([]string, *github.Response, error) {
	m.listCalled = true
	m.owner = owner
	m.repoName = repo
	if m.listErr != nil {
		return nil, nil, m.listErr
	}
	if repo == templateRepo.Name && owner == templateRepo.Org {
		return templateRepo.Topics, nil, nil
	} else if repo == newRepo.Name && owner == newRepo.Org {
		return newRepo.Topics, nil, nil
	}
	return nil, nil, errors.New("repository not found")
}

func TestCreateRepoSuccess(t *testing.T) {
	mockSvc := &mockService{
		createCalled:  false,
		replaceCalled: false,
		listCalled:    false,
		createErr:     nil,
		replaceErr:    nil,
		listErr:       nil,
	}

	opts := RepoCreationOptions{
		Service:      mockSvc,
		NewRepo:      newRepo,
		TemplateRepo: templateRepo,
	}

	combinedTopics := make(map[string]struct{})
	for _, topic := range templateRepo.Topics {
		combinedTopics[topic] = struct{}{}
	}
	for _, topic := range newRepo.Topics {
		combinedTopics[topic] = struct{}{}
	}
	var uniqueTopics []string
	for topic := range combinedTopics {
		uniqueTopics = append(uniqueTopics, topic)
	}

	ctx := context.Background()
	repo, err := CreateRepo(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.createCalled {
		t.Error("CreateFromTemplate was not called")
	}
	if !mockSvc.listCalled {
		t.Error("ListAllTopics was not called")
	}
	if !mockSvc.replaceCalled {
		t.Error("ReplaceAllTopics was not called")
	}
	if repo == nil {
		t.Fatal("expected a repository, got nil")
	}
	if mockSvc.repoName != newRepo.Name {
		t.Errorf("expected repo name %s, got %s", newRepo.Name, mockSvc.repoName)
	}
	if mockSvc.repoDesc != newRepo.Description {
		t.Errorf("expected repo description %s, got %s", newRepo.Description, mockSvc.repoDesc)
	}
	if mockSvc.repoPrivate != newRepo.Private {
		t.Errorf("expected repo private %v, got %v", newRepo.Private, mockSvc.repoPrivate)
	}
	if mockSvc.owner != newRepo.Org {
		t.Errorf("expected repo owner %s, got %s", newRepo.Org, mockSvc.owner)
	}
	if mockSvc.templateOwner != templateRepo.Org {
		t.Errorf("expected template owner %s, got %s", templateRepo.Org, mockSvc.templateOwner)
	}
	if mockSvc.templateName != templateRepo.Name {
		t.Errorf("expected template name %s, got %s", templateRepo.Name, mockSvc.templateName)
	}
	if len(mockSvc.repoTopics) != len(uniqueTopics) {
		t.Errorf("expected %d topics, got %d", len(uniqueTopics), len(mockSvc.repoTopics))
	}
	for _, topic := range uniqueTopics {
		found := false
		for _, t := range mockSvc.repoTopics {
			if t == topic {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected topic %s not found in topics", topic)
		}
	}
}

func TestCreateRepoInvalidTemplate(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		createErr:    nil,
	}
	opts := RepoCreationOptions{
		Service:      mockSvc,
		NewRepo:      newRepo,
		TemplateRepo: invalidTemplateRepo,
	}
	ctx := context.Background()
	_, err := CreateRepo(ctx, opts)
	if err == nil {
		t.Fatal("expected error for invalid template repository, got nil")
	}
	if !mockSvc.createCalled {
		t.Error("CreateFromTemplate was not called")
	}
}

func TestCreateRepoExistingRepo(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		createErr:    nil,
	}
	opts := RepoCreationOptions{
		Service:      mockSvc,
		NewRepo:      existingRepo,
		TemplateRepo: templateRepo,
	}
	ctx := context.Background()
	_, err := CreateRepo(ctx, opts)
	if err == nil {
		t.Fatal("expected error for existing repository, got nil")
	}
	if !mockSvc.createCalled {
		t.Error("CreateFromTemplate was not called")
	}
}

func TestCreateRepoErr(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		createErr:    errors.New("create error"),
	}
	opts := RepoCreationOptions{
		Service:      mockSvc,
		NewRepo:      newRepo,
		TemplateRepo: templateRepo,
	}
	ctx := context.Background()
	_, err := CreateRepo(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !mockSvc.createCalled {
		t.Error("CreateFromTemplate was not called")
	}
	if !errors.Is(err, mockSvc.createErr) {
		t.Errorf("expected error %v, got %v", mockSvc.createErr, err)
	}
}
