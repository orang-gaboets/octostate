package apply

import (
	"context"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBoundApplyEntryPointsRejectTargetBeforeGitHubCalls(t *testing.T) {
	t.Parallel()
	reads, writes := 0, 0
	repository := &testRepoService{
		getFunc: func(context.Context, string, string) (*gh.Repository, *gh.Response, error) {
			reads++
			return nil, nil, nil
		},
		createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			writes++
			return nil, nil, nil
		},
	}
	desired := config.OrganizationConfig{
		Organization: "org-b",
		Repositories: []config.RepositorySpec{{Name: "service", Visibility: "private"}},
	}
	plan := &gitopsplan.Report{Organization: "org-b", Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeRepository,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   "org-b/service",
		Executable:   true,
	}}}
	opt := testApplyOptions(desired, &state.OrganizationState{Organization: "org-b"}, plan, withRepoService(repository))

	for _, expected := range []string{"org-a", ""} {
		check, err := CheckForOrganization(context.Background(), opt, expected)
		if err == nil || check != nil {
			t.Fatalf("expected preflight target rejection for %q, got result %#v and error %v", expected, check, err)
		}
		result, err := ExecuteForOrganization(context.Background(), opt, expected)
		if err == nil || result != nil {
			t.Fatalf("expected apply target rejection for %q, got result %#v and error %v", expected, result, err)
		}
	}
	if reads != 0 || writes != 0 {
		t.Fatalf("mismatched target made GitHub calls: reads=%d writes=%d", reads, writes)
	}

	// Matching values reach the existing apply path; an empty plan has no probes or writes.
	opt.Plan = &gitopsplan.Report{Organization: "org-b"}
	check, err := CheckForOrganization(context.Background(), opt, " ORG-B ")
	if err != nil || check == nil {
		t.Fatalf("expected matching target to preflight, got %#v and %v", check, err)
	}
	result, err := ExecuteForOrganization(context.Background(), opt, "org-b")
	if err != nil || result == nil {
		t.Fatalf("expected matching target to apply, got %#v and %v", result, err)
	}
	if strings.TrimSpace(result.Organization) != "org-b" || reads != 0 || writes != 0 {
		t.Fatalf("unexpected matching target result %#v, reads=%d writes=%d", result, reads, writes)
	}
}
