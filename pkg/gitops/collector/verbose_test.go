package collector

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	ghlogging "github.com/orang-gaboets/octostate/pkg/github/logging"
)

func TestCollectVerboseLogsStageBoundaries(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name    string
		listErr error
		want    []string
		notWant string
	}{
		{name: "success", want: []string{"collect orang-gaboets: started", "collect orang-gaboets: completed in ", "repositories=1 teams=0"}, notWant: "failed"},
		{name: "failure", listErr: errors.New("list failed"), want: []string{"collect orang-gaboets: started", "collect orang-gaboets: failed after "}, notWant: "completed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repoSvc := &repositoryServiceStub{
				listByOrgFunc: func(context.Context, string, *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
					if tt.listErr != nil {
						return nil, nil, tt.listErr
					}
					return []*gh.Repository{{Owner: &gh.User{Login: githubpkg.Ptr("orang-gaboets")}, Name: githubpkg.Ptr("octostate")}}, &gh.Response{}, nil
				},
			}
			var logs bytes.Buffer
			ctx := ghlogging.WithVerbose(context.Background(), true, &logs)
			_, err := CollectOrganizationForMaterialize(ctx, CollectOrganizationOptions{OrgName: "orang-gaboets", RepositoryService: repoSvc})
			if (err != nil) != (tt.listErr != nil) {
				t.Fatalf("unexpected error: %v", err)
			}
			got := logs.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("verbose output missing %q:\n%s", want, got)
				}
			}
			if strings.Contains(got, tt.notWant) {
				t.Errorf("verbose output unexpectedly contains %q:\n%s", tt.notWant, got)
			}
		})
	}
}
