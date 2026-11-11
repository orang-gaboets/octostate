package repos

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"

	"github.com/orang-gaboets/octostate/pkg/github"
)

var errRunaway = errors.New("pagination did not stop")

// stuckPager always reports page 2 as next, so the second read repeats a
// page. A correct loop stops there; a loop that trusts NextPage runs until
// the runaway guard trips.
type stuckPager struct {
	Service
	calls int
	err   error
}

func (s *stuckPager) ListByOrg(ctx context.Context, _ string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
	s.calls++
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if s.calls == 2 && s.err != nil {
		return nil, nil, s.err
	}
	if s.calls > 10 {
		return nil, nil, errRunaway
	}
	return []*gh.Repository{{Name: gh.Ptr("repo")}}, &gh.Response{NextPage: 2}, nil
}

func TestListOrgReposRejectsNonAdvancingPagination(t *testing.T) {
	t.Parallel()

	svc := &stuckPager{}
	got, err := ListOrgRepos(context.Background(), ListOrgReposOptions{Service: svc, Org: "acme"})
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("error = %v, want %v", err, github.ErrValidationFailed)
	}
	if got != nil || svc.calls != 2 {
		t.Fatalf("got %d repos after %d calls, want none after 2", len(got), svc.calls)
	}
}

func TestListOrgReposPropagatesLaterPageError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("page 2 failed")
	svc := &stuckPager{err: wantErr}
	got, err := ListOrgRepos(context.Background(), ListOrgReposOptions{Service: svc, Org: "acme"})
	if !errors.Is(err, wantErr) || got != nil {
		t.Fatalf("got %d repos, error %v; want none and %v", len(got), err, wantErr)
	}
}

func TestListOrgReposStopsOnCancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	svc := &stuckPager{}
	got, err := ListOrgRepos(ctx, ListOrgReposOptions{Service: svc, Org: "acme"})
	if !errors.Is(err, context.Canceled) || got != nil || svc.calls != 1 {
		t.Fatalf("got %d repos after %d calls, error %v; want none after 1 and context.Canceled", len(got), svc.calls, err)
	}
}
