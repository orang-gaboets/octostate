package topics

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
	existingRepo = github.Repository{
		Owner:  "existing-org",
		Name:   "existing-name",
		Topics: []string{"existing-topic"},
	}
	nonExistentRepo = github.Repository{
		Owner: "non-existent-org",
		Name:  "non-existent-name",
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
	} else if owner == existingRepo.Owner && repo == existingRepo.Name {
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
	} else if owner == existingRepo.Owner && repo == existingRepo.Name {
		return topics, nil, nil
	}
	return nil, nil, fmt.Errorf("repository %s/%s not found: %w", owner, repo, github.ErrNotFound)
}

// Test ListAllTopics functionality

func TestListAllTopicsSuccess(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled: false,
	}

	option := ListAllTopicsOptions{
		Repo:    existingRepo.Name,
		Owner:   existingRepo.Owner,
		Service: service,
	}

	topics, err := ListAllTopics(ctx, option)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !service.listCalled {
		t.Fatal("expected ListAllTopics to be called")
	}
	expected := github.Unique(existingRepo.Topics)
	if !reflect.DeepEqual(topics, expected) {
		t.Fatalf("expected topics %v, got %v", expected, topics)
	}
}

func TestListAllTopicsNotFound(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled: false,
	}

	option := ListAllTopicsOptions{
		Repo:    nonExistentRepo.Name,
		Owner:   nonExistentRepo.Owner,
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
		Repo:    existingRepo.Name,
		Owner:   existingRepo.Owner,
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

// Test ReplaceAllTopics functionality

func TestReplaceAllTopicsEmpty(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		replaceCalled: false,
	}

	option := ReplaceAllTopicsOptions{
		Repo:    existingRepo.Name,
		Owner:   existingRepo.Owner,
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
		Repo:    nonExistentRepo.Name,
		Owner:   nonExistentRepo.Owner,
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

// Test AddTopics functionality

func TestAddTopicsSuccess(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled:    false,
		replaceCalled: false,
	}

	option := AddTopicsOptions{
		Repo:    existingRepo.Name,
		Owner:   existingRepo.Owner,
		Service: service,
		Topics:  replacedTopics,
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

	expected := github.MergeUnique(existingRepo.Topics, replacedTopics)
	if !reflect.DeepEqual(topics, expected) {
		t.Fatalf("expected topics %v, got %v", expected, topics)
	}
}

func TestAddTopicsDuplicate(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled:    false,
		replaceCalled: false,
	}
	option := AddTopicsOptions{
		Repo:    existingRepo.Name,
		Owner:   existingRepo.Owner,
		Service: service,
		Topics:  addDuplicateTopics,
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
	expected := github.MergeUnique(existingRepo.Topics, addDuplicateTopics)
	if !reflect.DeepEqual(topics, expected) {
		t.Fatalf("expected topics %v, got %v", expected, topics)
	}
}

func TestAddTopicsEmpty(t *testing.T) {
	ctx := context.Background()
	service := &mockService{
		listCalled:    false,
		replaceCalled: false,
	}

	option := AddTopicsOptions{
		Repo:    existingRepo.Name,
		Owner:   existingRepo.Owner,
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
		Repo:    nonExistentRepo.Name,
		Owner:   nonExistentRepo.Owner,
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
