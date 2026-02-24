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
		Owner:       "template-org",
		Name:        "template-name",
		Description: "template-desc",
		Private:     false,
		Topics:      []string{"template-topic"},
	}

	newRepo = github.Repository{
		Owner:       "org",
		Name:        "name",
		Description: "desc",
		Private:     false,
		Topics:      []string{"t1", "t2"},
	}

	invalidTemplateRepo = github.Repository{
		Owner: "invalid-org",
		Name:  "invalid-name",
	}

	existingRepo = github.Repository{
		Owner:       "existing-org",
		Name:        "existing-name",
		Description: "existing-desc",
	}

	completeEditOptions = EditOptions{
		Repo:         existingRepo.Name,
		Owner:        existingRepo.Owner,
		Description:  github.Ptr("new description"),
		Homepage:     github.Ptr("https://example.com"),
		Private:      github.Ptr(true),
		IsTemplate:   github.Ptr(false),
		Archived:     github.Ptr(false),
		AllowForking: github.Ptr(true),
	}

	partialEditOptions = EditOptions{
		Repo:        existingRepo.Name,
		Owner:       existingRepo.Owner,
		Description: github.Ptr("partial description"),
		IsTemplate:  github.Ptr(true),
	}

	orgRepositories = [][]*gh.Repository{
		{
			{
				Name: github.Ptr("repo-one"),
				Owner: &gh.User{
					Login: github.Ptr(existingRepo.Owner),
				},
			},
			{
				Name: github.Ptr("repo-two"),
				Owner: &gh.User{
					Login: github.Ptr(existingRepo.Owner),
				},
			},
		},
		{
			{
				Name: github.Ptr("repo-three"),
				Owner: &gh.User{
					Login: github.Ptr(existingRepo.Owner),
				},
			},
		},
	}
)

type mockService struct {
	createCalled    bool
	deleteCalled    bool
	editCalled      bool
	getCalled       bool
	listCalled      bool
	replaceCalled   bool
	listByOrgCalled bool
	createErr       error
	deleteErr       error
	editErr         error
	getErr          error
	listErr         error
	replaceErr      error
	listByOrgErr    error
	owner           string
	repoName        string
	repoDesc        string
	repoTopics      []string
	repoPrivate     bool
	templateName    string
	templateOwner   string
	editOptions     EditOptions
	listByOrgType   string
	lastPage        int
	orgRepos        [][]*gh.Repository
}

func (m *mockService) CreateFromTemplate(_ context.Context, owner, repo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	m.createCalled = true
	m.templateOwner = owner
	m.templateName = repo
	if m.createErr != nil {
		return nil, nil, m.createErr
	}
	if owner != templateRepo.Owner || repo != templateRepo.Name {
		return nil, nil, fmt.Errorf("invalid template repository %s/%s: %w", owner, repo, github.ErrNotFound)
	}
	if req != nil && req.Owner != nil && req.Name != nil && *req.Owner == existingRepo.Owner && *req.Name == existingRepo.Name {
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
	if owner != existingRepo.Owner || repo != existingRepo.Name {
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
	if owner != existingRepo.Owner || repo != existingRepo.Name {
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

func (m *mockService) Get(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
	m.getCalled = true
	m.owner = owner
	m.repoName = repo
	if m.getErr != nil {
		return nil, nil, m.getErr
	}
	if owner != existingRepo.Owner || repo != existingRepo.Name {
		return nil, nil, fmt.Errorf("invalid repository %s/%s: %w", owner, repo, github.ErrNotFound)
	}
	return &gh.Repository{
		Owner:       &gh.User{Login: &owner},
		Name:        &repo,
		Description: github.Ptr(existingRepo.Description),
		Private:     github.Ptr(existingRepo.Private),
		Topics:      existingRepo.Topics,
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
	if owner != newRepo.Owner || repo != newRepo.Name {
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
	if repo == templateRepo.Name && owner == templateRepo.Owner {
		return templateRepo.Topics, nil, nil
	} else if repo == newRepo.Name && owner == newRepo.Owner {
		return newRepo.Topics, nil, nil
	}
	return nil, nil, fmt.Errorf("repository %s/%s not found: %w", owner, repo, github.ErrNotFound)
}

func (m *mockService) ListByOrg(_ context.Context, org string, opts *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
	m.listByOrgCalled = true
	m.owner = org
	if opts != nil {
		m.listByOrgType = opts.Type
	}
	if m.listByOrgErr != nil {
		return nil, nil, m.listByOrgErr
	}
	if org != existingRepo.Owner {
		return nil, nil, github.ErrNotFound
	}

	page := 1
	if opts != nil && opts.Page > 0 {
		page = opts.Page
	}
	m.lastPage = page

	if page-1 < len(m.orgRepos) {
		nextPage := 0
		if page < len(m.orgRepos) {
			nextPage = page + 1
		}
		return m.orgRepos[page-1], &gh.Response{NextPage: nextPage}, nil
	}

	return []*gh.Repository{}, &gh.Response{NextPage: 0}, nil
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
		Owner:         newRepo.Owner,
		Description:   &newRepo.Description,
		Private:       &newRepo.Private,
		Topics:        newRepo.Topics,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Owner,
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
	if mockSvc.owner != newRepo.Owner {
		t.Errorf("expected repo owner %s, got %s", newRepo.Owner, mockSvc.owner)
	}
	if mockSvc.templateOwner != templateRepo.Owner {
		t.Errorf("expected template owner %s, got %s", templateRepo.Owner, mockSvc.templateOwner)
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
		Owner:         newRepo.Owner,
		TemplateRepo:  invalidTemplateRepo.Name,
		TemplateOwner: invalidTemplateRepo.Owner,
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
		Owner:         existingRepo.Owner,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Owner,
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
		Owner:         newRepo.Owner,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Owner,
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
	invalidNewRepo.Owner = ""
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          invalidNewRepo.Name,
		Owner:         invalidNewRepo.Owner,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Owner,
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
		Owner:         invalidNewRepo.Owner,
		TemplateRepo:  templateRepo.Name,
		TemplateOwner: templateRepo.Owner,
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
	invalidTemplateRepo.Owner = ""
	opts := CreateFromTemplateOptions{
		Service:       mockSvc,
		Name:          newRepo.Name,
		Owner:         newRepo.Owner,
		TemplateRepo:  invalidTemplateRepo.Name,
		TemplateOwner: invalidTemplateRepo.Owner,
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
		Owner:         newRepo.Owner,
		TemplateRepo:  invalidTemplateRepo.Name,
		TemplateOwner: invalidTemplateRepo.Owner,
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
		Owner:   existingRepo.Owner,
	}

	ctx := context.Background()
	err := Delete(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.deleteCalled {
		t.Error("Delete was not called")
	}
	if mockSvc.owner != existingRepo.Owner {
		t.Errorf("expected owner %s, got %s", existingRepo.Owner, mockSvc.owner)
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
		Owner:   invalidTemplateRepo.Owner,
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
		Owner:   existingRepo.Owner,
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
		Owner:   invalidTemplateRepo.Owner,
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

func TestListOrgReposSuccess(t *testing.T) {
	mockSvc := &mockService{
		orgRepos: orgRepositories,
	}

	opts := ListOrgReposOptions{
		Service: mockSvc,
		Org:     existingRepo.Owner,
		Type:    "all",
	}

	ctx := context.Background()
	repos, err := ListOrgRepos(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !mockSvc.listByOrgCalled {
		t.Fatal("expected ListByOrg to be called, but it was not")
	}
	if len(repos) != 3 {
		t.Fatalf("expected 3 repositories, got %d", len(repos))
	}
	if mockSvc.listByOrgType != string(opts.Type) {
		t.Fatalf("expected repo type %s, got %s", opts.Type, mockSvc.listByOrgType)
	}
	if mockSvc.lastPage != 2 {
		t.Fatalf("expected to paginate to page 2, got page %d", mockSvc.lastPage)
	}
}

func TestListOrgReposMissingOrg(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListOrgReposOptions{
		Service: mockSvc,
		Org:     "",
	}

	ctx := context.Background()
	_, err := ListOrgRepos(ctx, opts)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
	if mockSvc.listByOrgCalled {
		t.Fatal("expected ListByOrg to not be called, but it was")
	}
}

func TestListOrgReposNilService(t *testing.T) {
	opts := ListOrgReposOptions{
		Service: nil,
		Org:     existingRepo.Owner,
	}

	ctx := context.Background()
	_, err := ListOrgRepos(ctx, opts)
	if !errors.Is(err, github.ErrNilService) {
		t.Fatalf("expected error %v, got %v", github.ErrNilService, err)
	}
}

func TestListOrgReposInvalidType(t *testing.T) {
	mockSvc := &mockService{}

	opts := ListOrgReposOptions{
		Service: mockSvc,
		Org:     existingRepo.Owner,
		Type:    "unsupported",
	}

	ctx := context.Background()
	_, err := ListOrgRepos(ctx, opts)
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("expected error %v, got %v", github.ErrValidationFailed, err)
	}
	if mockSvc.listByOrgCalled {
		t.Fatal("expected ListByOrg to not be called, but it was")
	}
}

func TestListOrgReposServiceError(t *testing.T) {
	mockSvc := &mockService{
		listByOrgErr: errors.New("service error"),
	}

	opts := ListOrgReposOptions{
		Service: mockSvc,
		Org:     existingRepo.Owner,
	}

	ctx := context.Background()
	_, err := ListOrgRepos(ctx, opts)
	if !errors.Is(err, mockSvc.listByOrgErr) {
		t.Fatalf("expected error %v, got %v", mockSvc.listByOrgErr, err)
	}
	if !mockSvc.listByOrgCalled {
		t.Fatal("expected ListByOrg to be called, but it was not")
	}
}
