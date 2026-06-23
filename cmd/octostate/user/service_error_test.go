package user_test

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	usercmd "github.com/orang-gaboets/octostate/cmd/octostate/user"
)

var errUserCommandDependency = errors.New("user command dependency failed")

type failingUserService struct {
	auth.MockUserService
}

func (failingUserService) Get(context.Context, string) (*gh.User, *gh.Response, error) {
	return nil, nil, errUserCommandDependency
}

func (failingUserService) GetByID(context.Context, int64) (*gh.User, *gh.Response, error) {
	return nil, nil, errUserCommandDependency
}

func TestGetUserByUsernameCmdPropagatesServiceError(t *testing.T) {
	cmd := usercmd.GetUserByUsernameCmd(failingUserService{})
	cmd.SetArgs([]string{"--username", "u"})
	if err := cmd.Execute(); !errors.Is(err, errUserCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}

func TestGetUserByIDCmdPropagatesServiceError(t *testing.T) {
	cmd := usercmd.GetUserByIDCmd(failingUserService{})
	cmd.SetArgs([]string{"--id", "123"})
	if err := cmd.Execute(); !errors.Is(err, errUserCommandDependency) {
		t.Fatalf("expected dependency error, got %v", err)
	}
}
