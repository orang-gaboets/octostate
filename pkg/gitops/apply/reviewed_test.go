package apply

import (
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestCheckReviewedRejectsIdentityBeforePlanning(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []config.InviteSpec{{
			UserID: config.OptionalInt64{Present: true, Value: 123},
			Role:   "direct_member",
		}},
	}
	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		PendingInvitations: []state.PendingInvitation{{
			ID: 10, Username: "octocat", Role: "direct_member",
		}},
	}
	userLookups := 0
	options := reviewedTestOptions(desired, actual, "octostate-config-v1:sha256:"+strings.Repeat("0", 64))
	options.UserService = &testUserService{
		getByIDFunc: func(context.Context, int64) (*gh.User, *gh.Response, error) {
			userLookups++
			return &gh.User{Login: gh.Ptr("octocat")}, nil, nil
		},
	}

	_, err := CheckReviewed(context.Background(), options)
	var identityErr *config.ExpectedDesiredStateIdentityError
	if !errors.As(err, &identityErr) || identityErr.Kind != config.ExpectedDesiredStateIdentityErrorMismatch {
		t.Fatalf("expected identity mismatch, got %v", err)
	}
	if userLookups != 0 {
		t.Fatalf("planner ran before identity check: user lookups=%d", userLookups)
	}
}

func TestCheckReviewedBuildsFreshPlanFromProvidedActualState(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Name: "service", Visibility: "private",
		}},
	}
	identity, err := config.DesiredStateIdentity(desired)
	if err != nil {
		t.Fatalf("compute desired identity: %v", err)
	}

	missingRepoService := &testRepoService{
		getFunc: func(context.Context, string, string) (*gh.Repository, *gh.Response, error) {
			return nil, nil, githubNotFoundError("repository not found")
		},
	}
	missingActual := &state.OrganizationState{Organization: "orang-gaboets"}
	missingOptions := reviewedTestOptions(desired, missingActual, identity)
	missingOptions.RepositoryService = missingRepoService
	missingResult, err := CheckReviewed(context.Background(), missingOptions)
	if err != nil {
		t.Fatalf("check missing repository state: %v", err)
	}
	if missingResult.PlanSummary.ExecutableActions != 1 || len(missingResult.CheckedActions) != 1 {
		t.Fatalf("expected a fresh create plan from missing live state, got %#v", missingResult)
	}

	presentActual := &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{{Owner: "orang-gaboets", Name: "service", Visibility: "private"}},
	}
	presentOptions := reviewedTestOptions(desired, presentActual, identity)
	presentResult, err := CheckReviewed(context.Background(), presentOptions)
	if err != nil {
		t.Fatalf("check present repository state: %v", err)
	}
	if presentResult.PlanSummary.ExecutableActions != 0 || len(presentResult.CheckedActions) != 0 {
		t.Fatalf("expected no plan actions from current matching live state, got %#v", presentResult)
	}
}

func TestExecuteReviewedBuildsFreshPlanAndExecutesIt(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Name: "service", Visibility: "private",
		}},
	}
	identity, err := config.DesiredStateIdentity(desired)
	if err != nil {
		t.Fatalf("compute desired identity: %v", err)
	}

	creates := 0
	options := reviewedTestOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, identity)
	options.RepositoryService = &testRepoService{
		createFunc: func(_ context.Context, owner string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
			creates++
			if owner != "orang-gaboets" || repository.GetName() != "service" {
				t.Fatalf("unexpected create request owner=%q repository=%#v", owner, repository)
			}
			return repository, nil, nil
		},
	}

	result, err := ExecuteReviewed(context.Background(), options)
	if err != nil {
		t.Fatalf("execute reviewed apply: %v", err)
	}
	if creates != 1 || len(result.Executed) != 1 {
		t.Fatalf("expected the freshly planned repository create, creates=%d result=%#v", creates, result)
	}
}

func reviewedTestOptions(desired config.OrganizationConfig, actual *state.OrganizationState, identity string) ReviewedOptions {
	return ReviewedOptions{
		Desired:                desired,
		Actual:                 actual,
		ExpectedConfigIdentity: identity,
		OrganizationService:    &testOrganizationService{},
		RepositoryService:      &testRepoService{},
		TeamService:            &testTeamService{},
		UserService:            &testUserService{},
	}
}
