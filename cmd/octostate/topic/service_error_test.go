package topic_test

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	topicscmd "github.com/orang-gaboets/octostate/cmd/octostate/topic"
)

var errTopicCommandDependency = errors.New("topic command dependency failed")

type failingTopicService struct {
	auth.MockRepoService
	listErr    error
	replaceErr error
}

func (s failingTopicService) ListAllTopics(context.Context, string, string) ([]string, *gh.Response, error) {
	if s.listErr != nil {
		return nil, nil, s.listErr
	}
	return []string{"existing"}, nil, nil
}

func (s failingTopicService) ReplaceAllTopics(context.Context, string, string, []string) ([]string, *gh.Response, error) {
	if s.replaceErr != nil {
		return nil, nil, s.replaceErr
	}
	return []string{"updated"}, nil, nil
}

func TestListAllTopicsCmdPropagatesServiceError(t *testing.T) {
	cmd := topicscmd.ListAllTopicsCmd(failingTopicService{listErr: errTopicCommandDependency})
	cmd.SetArgs([]string{"--org", "o", "--name", "n"})
	if err := cmd.Execute(); !errors.Is(err, errTopicCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestAddTopicsCmdPropagatesListServiceError(t *testing.T) {
	cmd := topicscmd.AddTopicsCmd(failingTopicService{listErr: errTopicCommandDependency})
	cmd.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "new"})
	if err := cmd.Execute(); !errors.Is(err, errTopicCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestAddTopicsCmdPropagatesReplaceServiceError(t *testing.T) {
	cmd := topicscmd.AddTopicsCmd(failingTopicService{replaceErr: errTopicCommandDependency})
	cmd.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "new"})
	if err := cmd.Execute(); !errors.Is(err, errTopicCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestReplaceAllTopicsCmdPropagatesServiceError(t *testing.T) {
	cmd := topicscmd.ReplaceAllTopicsCmd(failingTopicService{replaceErr: errTopicCommandDependency})
	cmd.SetArgs([]string{"--org", "o", "--name", "n", "--topics", "new"})
	if err := cmd.Execute(); !errors.Is(err, errTopicCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}
