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

var (
	templateRepo = Repository{
		Org:         "template-org",
		Name:        "template-name",
		Description: "template-desc",
		Private:     false,
		Topics:      []string{"template-topic"},
	}

	newRepo = Repository{
		Org:         "org",
		Name:        "name",
		Description: "desc",
		Private:     false,
		Topics:      []string{"t1", "t2"},
	}
)

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
	if m.createErr != nil {
		return nil, nil, m.createErr
	} else if repo == templateRepo.Name && owner == templateRepo.Org {
		return templateRepo.Topics, nil, nil
	} else if repo == newRepo.Name && owner == newRepo.Org {
		return newRepo.Topics, nil, nil
	}
	return nil, nil, errors.New("repository not found")
}

func TestCreateRepoSuccess(t *testing.T) {
	svc := &mockRepoService{}
	opts := RepoCreationOptions{
		NewRepo:      newRepo,
		TemplateRepo: templateRepo,
		Service:      svc,
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
	if len(svc.lastTopics) != len(uniqueTopics) {
		t.Fatalf("expected %d topics, got %d", len(uniqueTopics), len(svc.lastTopics))
	}
	for _, topic := range uniqueTopics {
		found := false
		for _, t := range svc.lastTopics {
			if t == topic {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected topic %s not found in last topics", topic)
		}
	}
}

func TestCreateRepoCreateError(t *testing.T) {
	svc := &mockRepoService{createErr: errors.New("boom")}
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

func TestListAllTopicsSuccess(t *testing.T) {
	svc := &mockRepoService{}
	topics, err := ListAllTopics(context.Background(), svc, templateRepo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(topics) != len(templateRepo.Topics) {
		t.Fatalf("expected %d topics, got %d", len(templateRepo.Topics), len(topics))
	}
	for _, topic := range templateRepo.Topics {
		found := false
		for _, t := range topics {
			if t == topic {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected topic %s not found in topics", topic)
		}
	}
}

func TestListAllTopicsCreateError(t *testing.T) {
	svc := &mockRepoService{createErr: errors.New("boom")}
	_, err := ListAllTopics(context.Background(), svc, templateRepo)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, svc.createErr) {
		t.Fatalf("expected wrapped error")
	}
}

func TestReplaceAllTopicsSuccess(t *testing.T) {
	svc := &mockRepoService{}
	topics, err := ReplaceAllTopics(context.Background(), svc, newRepo, []string{"new-topic"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(topics) != 1 || topics[0] != "new-topic" {
		t.Fatalf("expected topics to be set to ['new-topic'], got %v", topics)
	}
	if !svc.replaceCalled {
		t.Fatal("expected ReplaceAllTopics to be called")
	}
}

func TestReplaceAllTopicsNoTopics(t *testing.T) {
	svc := &mockRepoService{}
	_, err := ReplaceAllTopics(context.Background(), svc, newRepo, []string{})
	if err == nil {
		t.Fatal("expected error for no topics")
	}
	if err.Error() != "no topics to set" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestAddTopicsSuccess(t *testing.T) {
	svc := &mockRepoService{}
	newTopics := []string{"new-topic"}
	topics, err := AddTopics(context.Background(), svc, newRepo, newTopics)
	// Make a set to join newRepo.Topics and the new topics
	combinedTopics := make(map[string]struct{})
	for _, topic := range newRepo.Topics {
		combinedTopics[topic] = struct{}{}
	}
	for _, topic := range newTopics {
		combinedTopics[topic] = struct{}{}
	}
	var uniqueTopics []string
	for topic := range combinedTopics {
		uniqueTopics = append(uniqueTopics, topic)
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.replaceCalled {
		t.Fatal("expected ReplaceAllTopics to be called")
	}
	for _, topic := range uniqueTopics {
		found := false
		for _, t := range topics {
			if t == topic {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected topic %s not found in topics", topic)
		}
	}
}
