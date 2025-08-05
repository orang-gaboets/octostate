package builder

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-github/v55/github"
)

type mockRepoService struct {
	createCalled      bool
	replaceCalled     bool
	createErr         error
	replaceErr        error
	lastOwner         string
	lastTemplate      string
	lastTemplateOwner string
	lastName          string
	lastDesc          string
	lastTopics        []string
	lastPrivate       bool
}

func (m *mockRepoService) CreateFromTemplate(ctx context.Context, owner, repo string, req *github.TemplateRepoRequest) (*github.Repository, *github.Response, error) {
	m.createCalled = true
	m.lastTemplateOwner = owner
	m.lastTemplate = repo
	if req != nil {
		if req.Name != nil {
			m.lastName = *req.Name
		}
		if req.Description != nil {
			m.lastDesc = *req.Description
		}
		if req.Private != nil {
			m.lastPrivate = *req.Private
		}
		if req.Owner != nil {
			m.lastOwner = *req.Owner
		}
	}
	return &github.Repository{}, nil, m.createErr
}

func (m *mockRepoService) ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *github.Response, error) {
	m.replaceCalled = true
	m.lastTopics = topics
	return topics, nil, m.replaceErr
}

func (m *mockRepoService) ListAllTopics(ctx context.Context, owner, repo string) ([]string, *github.Response, error) {
	if m.createCalled {
		return m.lastTopics, nil, nil
	}
	return nil, nil, errors.New("not called")
}

func TestCreateRepoSuccess(t *testing.T) {
	svc := &mockRepoService{}
	newRepo := Repository{
		Org:         "org",
		Name:        "name",
		Description: "desc",
		Private:     false,
		Topics:      []string{"t1", "t2"},
	}
	templateRepo := Repository{
		Org:         "template-org",
		Name:        "template-name",
		Description: "template-desc",
		Private:     false,
		Topics:      []string{"template-topic"},
	}
	opts := RepoCreationOptions{
		NewRepo:      newRepo,
		TemplateRepo: templateRepo,
		Service:      svc,
	}
	newMockRepo, err := CreateRepo(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newMockRepo == nil {
		t.Fatal("expected non-nil repository")
	}
	if !svc.createCalled || !svc.replaceCalled {
		t.Fatalf("expected service methods to be called")
	}
	if svc.lastTemplateOwner != opts.TemplateRepo.Org || svc.lastTemplate != opts.TemplateRepo.Name || svc.lastOwner != opts.NewRepo.Org || svc.lastName != opts.NewRepo.Name || svc.lastDesc != opts.NewRepo.Description || svc.lastPrivate != opts.NewRepo.Private {
		t.Fatalf("parameters not passed correctly")
	}
	if len(svc.lastTopics) != 2 {
		t.Fatalf("topics not set")
	}
}

func TestCreateRepoCreateError(t *testing.T) {
	svc := &mockRepoService{createErr: errors.New("boom")}
	newRepo := Repository{
		Org:         "org",
		Name:        "name",
		Description: "desc",
		Private:     false,
		Topics:      []string{"t1", "t2"},
	}
	templateRepo := Repository{
		Org:         "template-org",
		Name:        "template-name",
		Description: "template-desc",
		Private:     false,
		Topics:      []string{"template-topic"},
	}
	opts := RepoCreationOptions{
		NewRepo:      newRepo,
		TemplateRepo: templateRepo,
		Service:      svc,
	}
	newMockRepo, err := CreateRepo(context.Background(), opts)
	if err == nil {
		t.Fatalf("expected error")
	}
	if newMockRepo != nil {
		t.Fatalf("expected nil repository on error")
	}
	if !errors.Is(err, svc.createErr) {
		t.Fatalf("expected wrapped error")
	}
}
