package topics

import (
	"context"
	"errors"
	"fmt"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

var (
	existingRepo = github.Repository{
		Org:    "existing-org",
		Name:   "existing-name",
		Topics: []string{"existing-topic"},
	}
	nonExistentRepo = github.Repository{
		Org:  "non-existent-org",
		Name: "non-existent-name",
	}
	replacedTopics     = []string{"new-topic-1", "new-topic-2"}
	emptyTopics        = []string{}
	addDuplicateTopics = []string{
		"existing-topic",
		"new-topic-1",
		"new-topic-2",
		"new-topic-1",
	}
)

type mockService struct {
	listCalled    bool
	replaceCalled bool
	listErr       error
	replaceErr    error
	repoName      string
	repoOrg       string
	repoTopics    []string
}

func (m *mockService) ListAllTopics(_ context.Context, owner, repo string) ([]string, *gh.Response, error) {
	m.listCalled = true
	m.repoOrg = owner
	m.repoName = repo
	if m.listErr != nil {
		return nil, nil, m.listErr
	} else if owner == existingRepo.Org && repo == existingRepo.Name {
		return existingRepo.Topics, nil, nil
	}
	return nil, nil, fmt.Errorf("repository %s/%s not found: %w", owner, repo, github.ErrNotFound)
}

func (m *mockService) ReplaceAllTopics(_ context.Context, owner, repo string, topics []string) ([]string, *gh.Response, error) {
	m.replaceCalled = true
	m.repoOrg = owner
	m.repoName = repo
	m.repoTopics = topics
	if m.replaceErr != nil {
		return nil, nil, m.replaceErr
	} else if owner == existingRepo.Org && repo == existingRepo.Name {
		return topics, nil, nil
	}
	return nil, nil, fmt.Errorf("repository %s/%s not found: %w", owner, repo, github.ErrNotFound)
}

func TestListAllTopicsSuccess(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled: false,
	}

	option := ListAllTopicsOptions{
		Repo:    existingRepo,
		Service: service,
	}

	topics, err := ListAllTopics(ctx, option)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !service.listCalled {
		t.Fatal("expected ListAllTopics to be called")
	}

	if len(topics) != len(existingRepo.Topics) {
		t.Fatalf("expected %d topics, got %d", len(existingRepo.Topics), len(topics))
	}

	for _, topic := range topics {
		found := false
		for _, existingTopic := range existingRepo.Topics {
			if topic == existingTopic {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected topic %s not found in existing topics", topic)
		}
	}
}

func TestListAllTopicsNotFound(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled: false,
	}

	option := ListAllTopicsOptions{
		Repo:    nonExistentRepo,
		Service: service,
	}

	_, err := ListAllTopics(ctx, option)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}

	if !service.listCalled {
		t.Fatal("expected ListAllTopics to be called")
	}
}

func TestReplaceAllTopicsSuccess(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		replaceCalled: false,
	}

	option := ReplaceAllTopicsOptions{
		Repo:    existingRepo,
		Service: service,
		Topics:  replacedTopics,
	}

	topics, err := ReplaceAllTopics(ctx, option)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !service.replaceCalled {
		t.Fatal("expected ReplaceAllTopics to be called")
	}

	if len(topics) != len(replacedTopics) {
		t.Fatalf("expected %d topics, got %d", len(replacedTopics), len(topics))
	}

	for _, topic := range topics {
		found := false
		for _, replacedTopic := range replacedTopics {
			if topic == replacedTopic {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected topic %s not found in replaced topics", topic)
		}
	}
}

func TestReplaceAllTopicsEmpty(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		replaceCalled: false,
	}

	option := ReplaceAllTopicsOptions{
		Repo:    existingRepo,
		Service: service,
		Topics:  emptyTopics,
	}

	_, err := ReplaceAllTopics(ctx, option)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}

	if service.replaceCalled {
		t.Fatal("expected ReplaceAllTopics not to be called")
	}
}

func TestReplaceAllTopicsNotFound(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		replaceCalled: false,
	}

	option := ReplaceAllTopicsOptions{
		Repo:    nonExistentRepo,
		Service: service,
		Topics:  replacedTopics,
	}

	_, err := ReplaceAllTopics(ctx, option)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}

	if !service.replaceCalled {
		t.Fatal("expected ReplaceAllTopics to be called")
	}
}

func TestAddTopicsSuccess(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled:    false,
		replaceCalled: false,
	}

	option := AddTopicsOptions{
		Repo:    existingRepo,
		Service: service,
		Topics:  replacedTopics,
	}

	cleanedSet := make(map[string]struct{})
	for _, t := range existingRepo.Topics {
		if v := t; v != "" {
			cleanedSet[v] = struct{}{}
		}
	}
	for _, t := range replacedTopics {
		if v := t; v != "" {
			cleanedSet[v] = struct{}{}
		}
	}
	cleaned := make([]string, 0, len(cleanedSet))
	for topic := range cleanedSet {
		cleaned = append(cleaned, topic)
	}
	topics, err := AddTopics(ctx, option)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !service.listCalled {
		t.Fatal("expected ListAllTopics to be called")
	}
	if !service.replaceCalled {
		t.Fatal("expected ReplaceAllTopics to be called")
	}
	if len(topics) != len(cleaned) {
		t.Fatalf("expected %d topics, got %d", len(cleaned), len(topics))
	}
	for _, topic := range topics {
		found := false
		for _, t := range cleaned {
			if topic == t {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected topic %s not found in cleaned topics", topic)
		}
	}
}

func TestAddTopicsDuplicate(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled:    false,
		replaceCalled: false,
	}
	option := AddTopicsOptions{
		Repo:    existingRepo,
		Service: service,
		Topics:  addDuplicateTopics,
	}

	cleanedSet := make(map[string]struct{})
	for _, t := range existingRepo.Topics {
		if v := t; v != "" {
			cleanedSet[v] = struct{}{}
		}
	}
	for _, t := range addDuplicateTopics {
		if v := t; v != "" {
			cleanedSet[v] = struct{}{}
		}
	}
	cleaned := make([]string, 0, len(cleanedSet))
	for topic := range cleanedSet {
		cleaned = append(cleaned, topic)
	}

	topics, err := AddTopics(ctx, option)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !service.listCalled {
		t.Fatal("expected ListAllTopics to be called")
	}
	if !service.replaceCalled {
		t.Fatal("expected ReplaceAllTopics to be called")
	}
	if len(topics) != len(cleaned) {
		t.Fatalf("expected %d topics, got %d", len(cleaned), len(topics))
	}
	for _, topic := range topics {
		found := false
		for _, t := range cleaned {
			if topic == t {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected topic %s not found in cleaned topics", topic)
		}
	}
}

func TestAddTopicsEmpty(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled:    false,
		replaceCalled: false,
	}

	option := AddTopicsOptions{
		Repo:    existingRepo,
		Service: service,
		Topics:  emptyTopics,
	}

	_, err := AddTopics(ctx, option)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}

	if service.listCalled {
		t.Fatal("expected ListAllTopics not to be called")
	}
	if service.replaceCalled {
		t.Fatal("expected ReplaceAllTopics not to be called")
	}
}

func TestAddTopicsNotFound(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled:    false,
		replaceCalled: false,
	}

	option := AddTopicsOptions{
		Repo:    nonExistentRepo,
		Service: service,
		Topics:  replacedTopics,
	}

	_, err := AddTopics(ctx, option)
	if !errors.Is(err, github.ErrNotFound) {
		t.Fatalf("expected error %v, got %v", github.ErrNotFound, err)
	}

	if !service.listCalled {
		t.Fatal("expected ListAllTopics to be called")
	}
	if service.replaceCalled {
		t.Fatal("expected ReplaceAllTopics not to be called")
	}
}
