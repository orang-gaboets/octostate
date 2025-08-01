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

func TestCreateRepoSuccess(t *testing.T) {
	svc := &mockRepoService{}
	opts := RepoCreationOptions{
		Org:          "org",
		Name:         "name",
		Description:  "desc",
		Private:      false,
		Topics:       []string{"t1", "t2"},
		TemplateName: "tmpl",
		TemplateOrg:  "template-org",
		Service:      svc,
	}
	newRepo, err := CreateRepo(context.Background(), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newRepo == nil {
		t.Fatal("expected non-nil repository")
	}
	if !svc.createCalled || !svc.replaceCalled {
		t.Fatalf("expected service methods to be called")
	}
	if svc.lastTemplateOwner != opts.TemplateOrg || svc.lastTemplate != opts.TemplateName || svc.lastOwner != opts.Org || svc.lastName != opts.Name || svc.lastDesc != opts.Description || svc.lastPrivate != opts.Private {
		t.Fatalf("parameters not passed correctly")
	}
	if len(svc.lastTopics) != 2 {
		t.Fatalf("topics not set")
	}
}

func TestCreateRepoCreateError(t *testing.T) {
	svc := &mockRepoService{createErr: errors.New("boom")}
	opts := RepoCreationOptions{
		Org:          "org",
		TemplateName: "tmpl",
		Name:         "name",
		Service:      svc,
	}
	newRepo, err := CreateRepo(context.Background(), opts)
	if err == nil {
		t.Fatalf("expected error")
	}
	if newRepo != nil {
		t.Fatalf("expected nil repository on error")
	}
	if !errors.Is(err, svc.createErr) {
		t.Fatalf("expected wrapped error")
	}
}
