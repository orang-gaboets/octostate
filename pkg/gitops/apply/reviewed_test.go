package apply

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

func TestCheckReviewedRejectsWhitespaceExpectedOrganizationBeforePlanning(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []config.InviteSpec{{
			UserID: config.OptionalInt64{Present: true, Value: 123},
			Role:   "direct_member",
		}},
	}
	identity, err := config.DesiredStateIdentity(desired)
	if err != nil {
		t.Fatalf("compute desired identity: %v", err)
	}
	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		PendingInvitations: []state.PendingInvitation{{
			ID: 10, Username: "octocat", Role: "direct_member",
		}},
	}
	userLookups := 0
	options := reviewedTestOptions(desired, actual, identity)
	options.ExpectedOrganization = " \t "
	options.UserService = &testUserService{
		getByIDFunc: func(context.Context, int64) (*gh.User, *gh.Response, error) {
			userLookups++
			return &gh.User{Login: gh.Ptr("octocat")}, nil, nil
		},
	}

	_, err = CheckReviewed(context.Background(), options)
	var expectedOrgErr *config.ExpectedOrganizationError
	if !errors.As(err, &expectedOrgErr) || expectedOrgErr.Kind != config.ExpectedOrganizationErrorMissingExpected {
		t.Fatalf("expected missing expected-organization error, got %v", err)
	}
	if userLookups != 0 {
		t.Fatalf("planner ran before expected-organization validation: user lookups=%d", userLookups)
	}
}

func TestExecuteReviewedRejectsWhitespaceExpectedOrganizationBeforeWrites(t *testing.T) {
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
	writes := 0
	options := reviewedTestOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, identity)
	options.ExpectedOrganization = " \t "
	options.RepositoryService = &testRepoService{
		createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			writes++
			return &gh.Repository{}, nil, nil
		},
	}

	_, err = ExecuteReviewed(context.Background(), options)
	var expectedOrgErr *config.ExpectedOrganizationError
	if !errors.As(err, &expectedOrgErr) || expectedOrgErr.Kind != config.ExpectedOrganizationErrorMissingExpected {
		t.Fatalf("expected missing expected-organization error, got %v", err)
	}
	if writes != 0 {
		t.Fatalf("expected no writes after expected-organization validation failure, got %d", writes)
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

func TestExecuteReviewedRejectsIdentityMismatchBeforeWrites(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Name: "service", Visibility: "private",
		}},
	}
	writes := 0
	options := reviewedTestOptions(
		desired,
		&state.OrganizationState{Organization: "orang-gaboets"},
		"octostate-config-v1:sha256:"+strings.Repeat("0", 64),
	)
	options.RepositoryService = &testRepoService{
		createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			writes++
			return &gh.Repository{}, nil, nil
		},
	}

	_, err := ExecuteReviewed(context.Background(), options)
	var identityErr *config.ExpectedDesiredStateIdentityError
	if !errors.As(err, &identityErr) || identityErr.Kind != config.ExpectedDesiredStateIdentityErrorMismatch {
		t.Fatalf("expected desired-state identity mismatch, got %v", err)
	}
	if writes != 0 {
		t.Fatalf("expected no writes after identity mismatch, got %d", writes)
	}
}

func TestExecuteReviewedRejectsChangedManagedRepositoryBooleanBeforeWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		update func(*config.RepositorySpec)
	}{
		{name: "allow_forking", update: func(repo *config.RepositorySpec) { repo.AllowForking = true }},
		{name: "archived", update: func(repo *config.RepositorySpec) { repo.Archived = true }},
		{name: "is_template", update: func(repo *config.RepositorySpec) { repo.IsTemplate = true }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "organization.yaml")
			contents := []byte("organization: orang-gaboets\nrepositories:\n  - name: service\n    visibility: private\n    allow_forking: false\n    archived: false\n    is_template: false\n")
			if err := os.WriteFile(path, contents, 0o600); err != nil {
				t.Fatalf("write organization config: %v", err)
			}
			desired, err := config.LoadFile(path)
			if err != nil {
				t.Fatalf("load organization config: %v", err)
			}
			reviewedIdentity, err := config.DesiredStateIdentity(desired)
			if err != nil {
				t.Fatalf("identify reviewed config: %v", err)
			}
			tt.update(&desired.Repositories[0])

			writes := 0
			options := reviewedTestOptions(
				desired,
				&state.OrganizationState{Organization: "orang-gaboets"},
				reviewedIdentity,
			)
			options.RepositoryService = &testRepoService{
				createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
					writes++
					return &gh.Repository{}, nil, nil
				},
			}

			_, err = ExecuteReviewed(context.Background(), options)
			var identityErr *config.ExpectedDesiredStateIdentityError
			if !errors.As(err, &identityErr) || identityErr.Kind != config.ExpectedDesiredStateIdentityErrorMismatch {
				t.Fatalf("expected changed %s to reject the reviewed identity, got %v", tt.name, err)
			}
			if writes != 0 {
				t.Fatalf("expected no writes after changing %s, got %d", tt.name, writes)
			}
		})
	}
}

func TestExecuteReviewedRejectsChangedManagedRepositoryStringBeforeWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		setManaged func(*config.RepositorySpec)
		change     func(*config.RepositorySpec)
	}{
		{
			name:       "description",
			setManaged: func(repo *config.RepositorySpec) { repo.SetManagedDescription("reviewed description") },
			change:     func(repo *config.RepositorySpec) { repo.Description = "changed description" },
		},
		{
			name:       "homepage",
			setManaged: func(repo *config.RepositorySpec) { repo.SetManagedHomepage("https://reviewed.example") },
			change:     func(repo *config.RepositorySpec) { repo.Homepage = "https://changed.example" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			desired := config.OrganizationConfig{
				Organization: "orang-gaboets",
				Repositories: []config.RepositorySpec{{Name: "service", Visibility: "private"}},
			}
			tt.setManaged(&desired.Repositories[0])
			reviewedIdentity, err := config.DesiredStateIdentity(desired)
			if err != nil {
				t.Fatalf("identify reviewed config: %v", err)
			}
			tt.change(&desired.Repositories[0])

			writes := 0
			options := reviewedTestOptions(
				desired,
				&state.OrganizationState{Organization: "orang-gaboets"},
				reviewedIdentity,
			)
			options.RepositoryService = &testRepoService{
				createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
					writes++
					return &gh.Repository{}, nil, nil
				},
			}

			_, err = ExecuteReviewed(context.Background(), options)
			var identityErr *config.ExpectedDesiredStateIdentityError
			if !errors.As(err, &identityErr) || identityErr.Kind != config.ExpectedDesiredStateIdentityErrorMismatch {
				t.Fatalf("expected changed %s to reject reviewed identity before writes, got %v (writes=%d)", tt.name, err, writes)
			}
			if writes != 0 {
				t.Fatalf("expected no repository writes after changing %s, got %d", tt.name, writes)
			}
		})
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
