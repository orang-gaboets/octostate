package repos

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

var (
	templateRepo = github.Repository{
		Org:         "template-org",
		Name:        "template-name",
		Description: "template-desc",
		Private:     false,
		Topics:      []string{"template-topic"},
	}

	newRepo = github.Repository{
		Org:         "org",
		Name:        "name",
		Description: "desc",
		Private:     false,
		Topics:      []string{"t1", "t2"},
	}

	invalidTemplateRepo = github.Repository{
		Org:  "invalid-org",
		Name: "invalid-name",
	}

	existingRepo = github.Repository{
		Org:         "existing-org",
		Name:        "existing-name",
		Description: "existing-desc",
	}

	completeEditOptions = EditOptions{
		Repo:         existingRepo.Name,
		Owner:        existingRepo.Org,
		Description:  github.Ptr("new description"),
		Homepage:     github.Ptr("https://example.com"),
		Private:      github.Ptr(true),
		IsTemplate:   github.Ptr(false),
		Archived:     github.Ptr(false),
		AllowForking: github.Ptr(true),
	}

	partialEditOptions = EditOptions{
		Repo:        existingRepo.Name,
		Owner:       existingRepo.Org,
		Description: github.Ptr("partial description"),
		IsTemplate:  github.Ptr(true),
	}
)

type mockService struct {
	createCalled  bool
	deleteCalled  bool
	editCalled    bool
	listCalled    bool
	replaceCalled bool
	createErr     error
	deleteErr     error
	editErr       error
	listErr       error
	replaceErr    error
	owner         string
	repoName      string
	repoDesc      string
	repoTopics    []string
	repoPrivate   bool
	templateName  string
	templateOwner string
	editOptions   EditOptions
}

func (m *mockService) CreateFromTemplate(_ context.Context, owner, repo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	m.createCalled = true
	m.templateOwner = owner
	m.templateName = repo
	if m.createErr != nil {
		return nil, nil, m.createErr
	}
	if owner != templateRepo.Org || repo != templateRepo.Name {
		return nil, nil, fmt.Errorf("invalid template repository %s/%s: %w", owner, repo, github.ErrNotFound)
	}
	if req != nil && req.Owner != nil && req.Name != nil && *req.Owner == existingRepo.Org && *req.Name == existingRepo.Name {
		return nil, nil, fmt.Errorf("repository %s/%s already exists: %w", *req.Owner, *req.Name, github.ErrValidationFailed)
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

	return &gh.Repository{}, nil, nil
}

func (m *mockService) Delete(_ context.Context, owner, repo string) (*gh.Response, error) {
	m.deleteCalled = true
	m.owner = owner
	m.repoName = repo
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	if owner != existingRepo.Org || repo != existingRepo.Name {
		return nil, fmt.Errorf("invalid repository %s/%s: %w", owner, repo, github.ErrNotFound)
	}
	return nil, nil
}

func (m *mockService) Edit(_ context.Context, owner, repo string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
	m.editCalled = true
	m.editOptions = EditOptions{
		Service:      m,
		Owner:        owner,
		Repo:         repo,
		Description:  repository.Description,
		Homepage:     repository.Homepage,
		Private:      repository.Private,
		IsTemplate:   repository.IsTemplate,
		Archived:     repository.Archived,
		AllowForking: repository.AllowForking,
	}
	if m.editErr != nil {
		return nil, nil, m.editErr
	}
	if owner != existingRepo.Org || repo != existingRepo.Name {
		return nil, nil, fmt.Errorf("invalid repository %s/%s: %w", owner, repo, github.ErrNotFound)
	}
	return &gh.Repository{
		Owner:        &gh.User{Login: &owner},
		Name:         &repo,
		Description:  repository.Description,
		Homepage:     repository.Homepage,
		Private:      repository.Private,
		IsTemplate:   repository.IsTemplate,
		Archived:     repository.Archived,
		AllowForking: repository.AllowForking,
		Topics:       repository.Topics,
	}, nil, nil
}

func (m *mockService) ReplaceAllTopics(_ context.Context, owner, repo string, topics []string) ([]string, *gh.Response, error) {
	m.replaceCalled = true
	m.owner = owner
	m.repoName = repo
	m.repoTopics = topics
	if m.replaceErr != nil {
		return nil, nil, m.replaceErr
	}
	if owner != newRepo.Org || repo != newRepo.Name {
		return nil, nil, fmt.Errorf("invalid repository %s/%s: %w", owner, repo, github.ErrNotFound)
	}
	return topics, nil, nil
}

func (m *mockService) ListAllTopics(_ context.Context, owner, repo string) ([]string, *gh.Response, error) {
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
	return nil, nil, fmt.Errorf("repository %s/%s not found: %w", owner, repo, github.ErrNotFound)
}

// Test CreateFromTemplate functionality

func TestCreateFromTemplateSuccess(t *testing.T) {
	mockSvc := &mockService{
		createCalled:  false,
		replaceCalled: false,
		listCalled:    false,
		createErr:     nil,
		replaceErr:    nil,
		listErr:       nil,
	}

	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          newRepo.Name,
		Owner:         newRepo.Org,
		Description:   &newRepo.Description,
		Private:       &newRepo.Private,
		Topics:        newRepo.Topics,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Org,
	}

	uniqueTopics := github.MergeUnique(newRepo.Topics, templateRepo.Topics)

	ctx := context.Background()
	repo, err := CreateFromTemplate(ctx, opts)
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
	if !reflect.DeepEqual(mockSvc.repoTopics, uniqueTopics) {
		t.Errorf("expected topics %v, got %v", uniqueTopics, mockSvc.repoTopics)
	}
}

func TestCreateFromTemplateInvalidTemplate(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		createErr:    nil,
	}
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          newRepo.Name,
		Owner:         newRepo.Org,
		TemplateRepo:  invalidTemplateRepo.Name,
		TemplateOwner: invalidTemplateRepo.Org,
	}
	ctx := context.Background()
	_, err := CreateFromTemplate(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.createCalled {
		t.Error("CreateFromTemplate was not called")
	}
}

func TestCreateFromTemplateExistingRepo(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		createErr:    nil,
	}
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          existingRepo.Name,
		Owner:         existingRepo.Org,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Org,
	}
	ctx := context.Background()
	_, err := CreateFromTemplate(ctx, opts)
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("expected error %v, got %v", github.ErrValidationFailed, err)
	}
	if !mockSvc.createCalled {
		t.Error("CreateFromTemplate was not called")
	}
}

func TestCreateFromTemplateErr(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
		createErr:    errors.New("create error"),
	}
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          newRepo.Name,
		Owner:         newRepo.Org,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Org,
	}
	ctx := context.Background()
	_, err := CreateFromTemplate(ctx, opts)
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

func TestCreateFromTemplateMissingNewRepoOrg(t *testing.T) {
	mockSvc := &mockService{}
	invalidNewRepo := newRepo
	invalidNewRepo.Org = ""
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          invalidNewRepo.Name,
		Owner:         invalidNewRepo.Org,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Org,
	}
	ctx := context.Background()
	_, err := CreateFromTemplate(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.createCalled {
		t.Error("CreateFromTemplate was called")
	}
}

func TestCreateFromTemplateMissingNewRepoName(t *testing.T) {
	mockSvc := &mockService{}
	invalidNewRepo := newRepo
	invalidNewRepo.Name = ""
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          invalidNewRepo.Name,
		Owner:         invalidNewRepo.Org,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Org,
	}
	ctx := context.Background()
	_, err := CreateFromTemplate(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.createCalled {
		t.Error("CreateFromTemplate was called")
	}
}

func TestCreateFromTemplateMissingTemplateRepoOrg(t *testing.T) {
	mockSvc := &mockService{}
	invalidTemplateRepo := templateRepo
	invalidTemplateRepo.Org = ""
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          newRepo.Name,
		Owner:         newRepo.Org,
		TemplateRepo:  invalidTemplateRepo.Name,
		TemplateOwner: invalidTemplateRepo.Org,
	}
	ctx := context.Background()
	_, err := CreateFromTemplate(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.createCalled {
		t.Error("CreateFromTemplate was called")
	}
}

func TestCreateFromTemplateMissingTemplateRepoName(t *testing.T) {
	mockSvc := &mockService{
		createCalled: false,
	}
	invalidTemplateRepo := templateRepo
	invalidTemplateRepo.Name = ""
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          newRepo.Name,
		Owner:         newRepo.Org,
		TemplateRepo:  invalidTemplateRepo.Name,
		TemplateOwner: invalidTemplateRepo.Org,
	}
	ctx := context.Background()
	_, err := CreateFromTemplate(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.createCalled {
		t.Error("CreateFromTemplate was called")
	}
}

// Test Delete functionality

func TestDeleteSuccess(t *testing.T) {
	mockSvc := &mockService{
		deleteCalled: false,
		deleteErr:    nil,
	}

	opts := DeleteOptions{
		Service: mockSvc,
		Repo:    existingRepo.Name,
		Owner:   existingRepo.Org,
	}

	ctx := context.Background()
	err := Delete(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.deleteCalled {
		t.Error("Delete was not called")
	}
	if mockSvc.owner != existingRepo.Org {
		t.Errorf("expected owner %s, got %s", existingRepo.Org, mockSvc.owner)
	}
	if mockSvc.repoName != existingRepo.Name {
		t.Errorf("expected repo name %s, got %s", existingRepo.Name, mockSvc.repoName)
	}
}

func TestDeleteInvalidRepo(t *testing.T) {
	mockSvc := &mockService{
		deleteCalled: false,
		deleteErr:    nil,
	}

	opts := DeleteOptions{
		Service: mockSvc,
		Repo:    invalidTemplateRepo.Name,
		Owner:   invalidTemplateRepo.Org,
	}

	ctx := context.Background()
	err := Delete(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.deleteCalled {
		t.Error("Delete was not called")
	}
}

func TestDeleteErr(t *testing.T) {
	mockSvc := &mockService{
		deleteCalled: false,
		deleteErr:    errors.New("delete error"),
	}

	opts := DeleteOptions{
		Service: mockSvc,
		Repo:    existingRepo.Name,
		Owner:   existingRepo.Org,
	}

	ctx := context.Background()
	err := Delete(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !mockSvc.deleteCalled {
		t.Error("Delete was not called")
	}
	if !errors.Is(err, mockSvc.deleteErr) {
		t.Errorf("expected error %v, got %v", mockSvc.deleteErr, err)
	}
}

// Test Edit functionality

func TestEditAllFieldsSuccess(t *testing.T) {
	mockSvc := &mockService{
		editCalled: false,
		editErr:    nil,
	}

	opts := completeEditOptions
	opts.Service = mockSvc

	ctx := context.Background()
	repo, err := Edit(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.editCalled {
		t.Error("Edit was not called")
	}
	if repo == nil {
		t.Fatal("expected a repository, got nil")
	}
	if repo.GetDescription() != *opts.Description {
		t.Errorf("expected description %s, got %s", *opts.Description, repo.GetDescription())
	}
	if repo.GetHomepage() != *opts.Homepage {
		t.Errorf("expected homepage %s, got %s", *opts.Homepage, repo.GetHomepage())
	}
	if repo.GetPrivate() != *opts.Private {
		t.Errorf("expected private %v, got %v", *opts.Private, repo.GetPrivate())
	}
	if repo.GetIsTemplate() != *opts.IsTemplate {
		t.Errorf("expected is_template %v, got %v", *opts.IsTemplate, repo.GetIsTemplate())
	}
	if repo.GetArchived() != *opts.Archived {
		t.Errorf("expected archived %v, got %v", *opts.Archived, repo.GetArchived())
	}
	if repo.GetAllowForking() != *opts.AllowForking {
		t.Errorf("expected allow_forking %v, got %v", *opts.AllowForking, repo.GetAllowForking())
	}
}

func TestEditPartialFieldsSuccess(t *testing.T) {
	mockSvc := &mockService{
		editCalled: false,
		editErr:    nil,
	}

	opts := partialEditOptions
	opts.Service = mockSvc

	ctx := context.Background()
	repo, err := Edit(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.editCalled {
		t.Error("Edit was not called")
	}
	if repo == nil {
		t.Fatal("expected a repository, got nil")
	}
	if repo.GetDescription() != *opts.Description {
		t.Errorf("expected description %s, got %s", *opts.Description, repo.GetDescription())
	}
	if repo.GetIsTemplate() != *opts.IsTemplate {
		t.Errorf("expected is_template %v, got %v", *opts.IsTemplate, repo.GetIsTemplate())
	}
}

func TestEditInvalidRepo(t *testing.T) {
	mockSvc := &mockService{
		editCalled: false,
		editErr:    nil,
	}

	opts := EditOptions{
		Service: mockSvc,
		Repo:    invalidTemplateRepo.Name,
		Owner:   invalidTemplateRepo.Org,
	}

	ctx := context.Background()
	_, err := Edit(ctx, opts)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}
	if !mockSvc.editCalled {
		t.Error("Edit was not called")
	}
}

func TestEditErr(t *testing.T) {
	mockSvc := &mockService{
		editCalled: false,
		editErr:    errors.New("edit error"),
	}

	opts := completeEditOptions
	opts.Service = mockSvc

	ctx := context.Background()
	_, err := Edit(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !mockSvc.editCalled {
		t.Error("Edit was not called")
	}
	if !errors.Is(err, mockSvc.editErr) {
		t.Errorf("expected error %v, got %v", mockSvc.editErr, err)
	}
}
