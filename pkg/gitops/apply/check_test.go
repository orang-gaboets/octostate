package apply

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestCheckSkipsNonExecutableDriftWithoutMutations(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Owner:      "orang-gaboets",
			Name:       "octostate",
			Visibility: "private",
			Template: config.TemplateSpec{
				Owner: "orang-gaboets",
				Name:  "repo-template",
			},
		}},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   repositoryResourceID("orang-gaboets", "octostate"),
				Executable:   true,
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationDelete,
				ResourceID:   repositoryResourceID("orang-gaboets", "legacy"),
				Executable:   false,
			},
		},
	}
	plan.Normalize()

	repoSvc := &testRepoService{
		createFromTemplateFunc: func(context.Context, string, string, *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
			t.Fatal("create-from-template should not run during check")
			return nil, nil, nil
		},
		editFunc: func(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			t.Fatal("repository edit should not run during check")
			return nil, nil, nil
		},
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			switch repositoryResourceID(owner, repo) {
			case repositoryResourceID("orang-gaboets", "repo-template"):
				return githubRepository("orang-gaboets", "repo-template", true), nil, nil
			case repositoryResourceID("orang-gaboets", "octostate"):
				return nil, nil, githubNotFoundError("repository not found")
			default:
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
				return nil, nil, nil
			}
		},
	}

	result, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Organization != "orang-gaboets" {
		t.Fatalf("unexpected organization %q", result.Organization)
	}
	if !reflect.DeepEqual(result.CheckedActions, []gitopsplan.Action{plan.Actions[0]}) {
		t.Fatalf("unexpected checked actions: %#v", result.CheckedActions)
	}
	if !reflect.DeepEqual(result.SkippedActions, []gitopsplan.Action{plan.Actions[1]}) {
		t.Fatalf("unexpected skipped actions: %#v", result.SkippedActions)
	}
}

func TestCheckPreflightsTeamCreatesAndInviteDependenciesWithoutMutations(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams: []config.TeamSpec{
			{Slug: "app", Name: "App", Privacy: "closed", ParentSlug: "platform"},
			{Slug: "platform", Name: "Platform", Privacy: "closed"},
		},
		Invites: []config.InviteSpec{
			inviteByEmail("alice@example.com", "direct_member", "platform", "app"),
		},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("app"), Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("platform"), Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "email:alice@example.com", Executable: true},
		},
	}
	plan.Normalize()

	teamSvc := &testTeamService{
		createTeamFunc: func(context.Context, string, gh.NewTeam) (*gh.Team, *gh.Response, error) {
			t.Fatal("team creation should not run during check")
			return nil, nil, nil
		},
		getTeamBySlugFunc: func(_ context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
			if org != "orang-gaboets" {
				t.Fatalf("unexpected org %q", org)
			}
			switch slug {
			case "app", "platform":
				return nil, nil, githubNotFoundError("team not found")
			default:
				t.Fatalf("unexpected team lookup %q", slug)
				return nil, nil, nil
			}
		},
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			t.Fatal("invite creation should not run during check")
			return nil, nil, nil
		},
	}
	userSvc := &testUserService{
		getFunc: func(context.Context, string) (*gh.User, *gh.Response, error) {
			t.Fatal("username lookup should not run during email invite check")
			return nil, nil, errors.New("unexpected lookup")
		},
	}

	result, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withTeamService(teamSvc), withOrganizationService(orgSvc), withUserService(userSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := len(result.CheckedActions), len(plan.Actions); got != want {
		t.Fatalf("unexpected checked action count: got %d want %d", got, want)
	}
	if len(result.SkippedActions) != 0 {
		t.Fatalf("unexpected skipped actions: %#v", result.SkippedActions)
	}
}

func TestCheckRepositoryCreateFailsWhenTemplateIsNotTemplate(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Owner:      "orang-gaboets",
			Name:       "octostate",
			Visibility: "private",
			Template: config.TemplateSpec{
				Owner: "orang-gaboets",
				Name:  "repo-template",
			},
		}},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   repositoryResourceID("orang-gaboets", "octostate"),
			Executable:   true,
		}},
	}
	plan.Normalize()

	repoSvc := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			if repositoryResourceID(owner, repo) != repositoryResourceID("orang-gaboets", "repo-template") {
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
			}
			return githubRepository("orang-gaboets", "repo-template", false), nil, nil
		},
	}

	_, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(repoSvc)))
	if err == nil {
		t.Fatal("expected create failure")
	}
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "template repository orang-gaboets/repo-template is not marked as a template") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestCheckAllowsSamePlanRepositoryTemplateChains(t *testing.T) {
	templateRepo := config.RepositorySpec{
		Owner:      "orang-gaboets",
		Name:       "aaa-template",
		Visibility: "private",
		Template: config.TemplateSpec{
			Owner: "orang-gaboets",
			Name:  "starter-template",
		},
	}
	templateRepo.SetManagedIsTemplate(true)

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{
			templateRepo,
			{
				Owner:      "orang-gaboets",
				Name:       "zzz-octostate",
				Visibility: "private",
				Template: config.TemplateSpec{
					Owner: "orang-gaboets",
					Name:  "aaa-template",
				},
			},
		},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   repositoryResourceID("orang-gaboets", "aaa-template"),
				Executable:   true,
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   repositoryResourceID("orang-gaboets", "zzz-octostate"),
				Executable:   true,
			},
		},
	}
	plan.Normalize()

	var starterTemplateLookups int
	var repoTemplateLookups int
	var octostateLookups int
	repoSvc := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			switch repositoryResourceID(owner, repo) {
			case repositoryResourceID("orang-gaboets", "starter-template"):
				starterTemplateLookups++
				return githubRepository("orang-gaboets", "starter-template", true), nil, nil
			case repositoryResourceID("orang-gaboets", "aaa-template"):
				repoTemplateLookups++
				return nil, nil, githubNotFoundError("repository not found")
			case repositoryResourceID("orang-gaboets", "zzz-octostate"):
				octostateLookups++
				return nil, nil, githubNotFoundError("repository not found")
			default:
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
				return nil, nil, nil
			}
		},
	}

	result, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(result.CheckedActions), len(plan.Actions); got != want {
		t.Fatalf("unexpected checked action count: got %d want %d", got, want)
	}
	if starterTemplateLookups != 1 {
		t.Fatalf("expected exactly one live lookup for the starter template, got %d", starterTemplateLookups)
	}
	if repoTemplateLookups != 1 {
		t.Fatalf("expected exactly one live lookup for the same-plan template repo target, got %d", repoTemplateLookups)
	}
	if octostateLookups != 1 {
		t.Fatalf("expected exactly one lookup for the second repository target, got %d", octostateLookups)
	}
}

func TestCheckRejectsFutureSamePlanRepositoryTemplateChains(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{
			{
				Owner:      "orang-gaboets",
				Name:       "aaa-octostate",
				Visibility: "private",
				Template: config.TemplateSpec{
					Owner: "orang-gaboets",
					Name:  "zzz-template",
				},
			},
			{
				Owner:      "orang-gaboets",
				Name:       "zzz-template",
				Visibility: "private",
				Template: config.TemplateSpec{
					Owner: "orang-gaboets",
					Name:  "starter-template",
				},
			},
		},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   repositoryResourceID("orang-gaboets", "aaa-octostate"),
				Executable:   true,
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   repositoryResourceID("orang-gaboets", "zzz-template"),
				Executable:   true,
			},
		},
	}
	plan.Normalize()

	repoSvc := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			switch repositoryResourceID(owner, repo) {
			case repositoryResourceID("orang-gaboets", "starter-template"):
				return githubRepository("orang-gaboets", "starter-template", true), nil, nil
			case repositoryResourceID("orang-gaboets", "zzz-template"):
				return nil, nil, githubNotFoundError("repository not found")
			case repositoryResourceID("orang-gaboets", "aaa-octostate"):
				return nil, nil, githubNotFoundError("repository not found")
			default:
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
				return nil, nil, nil
			}
		},
	}

	_, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(repoSvc)))
	if err == nil {
		t.Fatal("expected future same-plan template chain failure")
	}
	if !errors.Is(err, githubpkg.ErrNotFound) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "template repository orang-gaboets/zzz-template") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestCheckRepositoryUpdateFailsWhenTargetMissing(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Owner:      "orang-gaboets",
			Name:       "octostate",
			Visibility: "private",
		}},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationUpdate,
			ResourceID:   repositoryResourceID("orang-gaboets", "octostate"),
			Executable:   true,
			Changes: []gitopsplan.FieldChange{{
				Field: "visibility",
				From:  "public",
				To:    "private",
			}},
		}},
	}
	plan.Normalize()

	repoSvc := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			if repositoryResourceID(owner, repo) != repositoryResourceID("orang-gaboets", "octostate") {
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
			}
			return nil, nil, githubNotFoundError("repository not found")
		},
	}

	_, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(repoSvc)))
	if err == nil {
		t.Fatal("expected update failure")
	}
	if !errors.Is(err, githubpkg.ErrNotFound) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckTeamUpdateFailsWhenTargetMissing(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams: []config.TeamSpec{{
			Slug:        "platform",
			Name:        "Platform",
			Privacy:     "closed",
			Description: "Updated",
		}},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeTeam,
			Operation:    gitopsplan.ActionOperationUpdate,
			ResourceID:   teamResourceID("platform"),
			Executable:   true,
			Changes: []gitopsplan.FieldChange{{
				Field: "description",
				From:  "",
				To:    "Updated",
			}},
		}},
	}
	plan.Normalize()

	teamSvc := &testTeamService{
		getTeamBySlugFunc: func(_ context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
			if org != "orang-gaboets" || slug != "platform" {
				t.Fatalf("unexpected team lookup %s/%s", org, slug)
			}
			return nil, nil, githubNotFoundError("team not found")
		},
	}

	_, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withTeamService(teamSvc)))
	if err == nil {
		t.Fatal("expected update failure")
	}
	if !errors.Is(err, githubpkg.ErrNotFound) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckInvitePreflightResolvesUsername(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []config.InviteSpec{
			inviteByUsername("alice", "direct_member"),
		},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeInvite,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   "username:alice",
			Executable:   true,
		}},
	}
	plan.Normalize()

	var lookedUp bool
	userSvc := &testUserService{
		getFunc: func(_ context.Context, username string) (*gh.User, *gh.Response, error) {
			lookedUp = true
			if username != "alice" {
				t.Fatalf("unexpected username %q", username)
			}
			return githubUser(42, "alice"), nil, nil
		},
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			t.Fatal("invite creation should not run during check")
			return nil, nil, nil
		},
	}

	result, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withUserService(userSvc), withOrganizationService(orgSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !lookedUp {
		t.Fatal("expected username lookup")
	}
	if got, want := len(result.CheckedActions), 1; got != want {
		t.Fatalf("unexpected checked actions: got %d want %d", got, want)
	}
}

func TestCheckInvitePreflightFailsWhenUsernameLookupFails(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []config.InviteSpec{
			inviteByUsername("alice", "direct_member"),
		},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeInvite,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   "username:alice",
			Executable:   true,
		}},
	}
	plan.Normalize()

	userSvc := &testUserService{
		getFunc: func(_ context.Context, username string) (*gh.User, *gh.Response, error) {
			if username != "alice" {
				t.Fatalf("unexpected username %q", username)
			}
			return nil, nil, githubNotFoundError("user not found")
		},
	}

	_, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withUserService(userSvc)))
	if err == nil {
		t.Fatal("expected invite failure")
	}
	if !errors.Is(err, githubpkg.ErrNotFound) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "username:alice") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestCheckTeamRepoPermissionAllowsSamePlanTeamAndRepositoryCreates(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Owner:      "orang-gaboets",
			Name:       "octostate",
			Visibility: "private",
			Template: config.TemplateSpec{
				Owner: "orang-gaboets",
				Name:  "repo-template",
			},
		}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Repositories: []config.TeamRepositorySpec{{
				Owner:      "orang-gaboets",
				Name:       "octostate",
				Permission: "push",
			}},
		}},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{ResourceType: gitopsplan.ActionResourceTypeRepository, Operation: gitopsplan.ActionOperationCreate, ResourceID: repositoryResourceID("orang-gaboets", "octostate"), Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("platform"), Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeTeamRepositoryPermission, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamRepoPermissionResourceID("platform", "orang-gaboets", "octostate"), Executable: true},
		},
	}
	plan.Normalize()

	var targetRepoLookups int
	repoSvc := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			switch repositoryResourceID(owner, repo) {
			case repositoryResourceID("orang-gaboets", "repo-template"):
				return githubRepository("orang-gaboets", "repo-template", true), nil, nil
			case repositoryResourceID("orang-gaboets", "octostate"):
				targetRepoLookups++
				return nil, nil, githubNotFoundError("repository not found")
			default:
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
				return nil, nil, nil
			}
		},
	}

	var targetTeamLookups int
	teamSvc := &testTeamService{
		getTeamBySlugFunc: func(_ context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
			if org != "orang-gaboets" {
				t.Fatalf("unexpected org %q", org)
			}
			switch slug {
			case "platform":
				targetTeamLookups++
				return nil, nil, githubNotFoundError("team not found")
			default:
				t.Fatalf("unexpected team lookup %q", slug)
				return nil, nil, nil
			}
		},
	}

	result, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(repoSvc), withTeamService(teamSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(result.CheckedActions), len(plan.Actions); got != want {
		t.Fatalf("unexpected checked action count: got %d want %d", got, want)
	}
	if targetRepoLookups != 1 {
		t.Fatalf("expected exactly one target repo lookup during create preflight, got %d", targetRepoLookups)
	}
	if targetTeamLookups != 1 {
		t.Fatalf("expected exactly one target team lookup during create preflight, got %d", targetTeamLookups)
	}
}

func TestCheckTeamRepoPermissionFailsWhenLiveRepositoryMissing(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Repositories: []config.TeamRepositorySpec{{
				Owner:      "orang-gaboets",
				Name:       "octostate",
				Permission: "push",
			}},
		}},
	}
	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		Teams: []state.Team{{
			ID:   10,
			Slug: "platform",
			Name: "Platform",
		}},
	}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeTeamRepositoryPermission,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   teamRepoPermissionResourceID("platform", "orang-gaboets", "octostate"),
			Executable:   true,
		}},
	}
	plan.Normalize()

	teamSvc := &testTeamService{
		getTeamBySlugFunc: func(_ context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
			if org != "orang-gaboets" || slug != "platform" {
				t.Fatalf("unexpected team lookup %s/%s", org, slug)
			}
			return githubTeam(10, "platform", "Platform", "orang-gaboets"), nil, nil
		},
	}
	repoSvc := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			if repositoryResourceID(owner, repo) != repositoryResourceID("orang-gaboets", "octostate") {
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
			}
			return nil, nil, githubNotFoundError("repository not found")
		},
	}

	_, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withTeamService(teamSvc), withRepoService(repoSvc)))
	if err == nil {
		t.Fatal("expected permission failure")
	}
	if !errors.Is(err, githubpkg.ErrNotFound) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckReusesVerifiedLiveTeamsAndRepositories(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Owner:      "orang-gaboets",
			Name:       "octostate",
			Visibility: "private",
		}},
		Teams: []config.TeamSpec{{
			Slug:        "platform",
			Name:        "Platform",
			Privacy:     "closed",
			Description: "Updated",
			Repositories: []config.TeamRepositorySpec{{
				Owner:      "orang-gaboets",
				Name:       "octostate",
				Permission: "push",
			}},
		}},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   repositoryResourceID("orang-gaboets", "octostate"),
				Executable:   true,
				Changes: []gitopsplan.FieldChange{{
					Field: "visibility",
					From:  "public",
					To:    "private",
				}},
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeTeam,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   teamResourceID("platform"),
				Executable:   true,
				Changes: []gitopsplan.FieldChange{{
					Field: "description",
					From:  "",
					To:    "Updated",
				}},
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeTeamRepositoryPermission,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   teamRepoPermissionResourceID("platform", "orang-gaboets", "octostate"),
				Executable:   true,
			},
		},
	}
	plan.Normalize()

	var repoLookups int
	repoSvc := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			repoLookups++
			if repositoryResourceID(owner, repo) != repositoryResourceID("orang-gaboets", "octostate") {
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
			}
			return githubRepository("orang-gaboets", "octostate", false), nil, nil
		},
	}

	var teamLookups int
	teamSvc := &testTeamService{
		getTeamBySlugFunc: func(_ context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
			teamLookups++
			if org != "orang-gaboets" || slug != "platform" {
				t.Fatalf("unexpected team lookup %s/%s", org, slug)
			}
			return githubTeam(10, "platform", "Platform", "orang-gaboets"), nil, nil
		},
	}

	result, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(repoSvc), withTeamService(teamSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := len(result.CheckedActions), len(plan.Actions); got != want {
		t.Fatalf("unexpected checked action count: got %d want %d", got, want)
	}
	if repoLookups != 1 {
		t.Fatalf("expected one repository lookup, got %d", repoLookups)
	}
	if teamLookups != 1 {
		t.Fatalf("expected one team lookup, got %d", teamLookups)
	}
}

func TestCheckFailsWhenCheckTeamDependenciesCannotBeResolved(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams: []config.TeamSpec{
			{Slug: "app", Name: "App", Privacy: "closed", ParentSlug: "platform"},
		},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("app"), Executable: true},
		},
	}
	plan.Normalize()

	_, err := Check(context.Background(), testApplyOptions(desired, actual, plan))
	if err == nil {
		t.Fatal("expected dependency error")
	}
	if !errors.Is(err, githubpkg.ErrNotFound) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "parent team platform") {
		t.Fatalf("unexpected error text: %v", err)
	}
}

func TestCheckAggregatesMultipleIndependentFailures(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{
			{
				Owner:      "orang-gaboets",
				Name:       "new-repo",
				Visibility: "private",
				Template: config.TemplateSpec{
					Owner: "orang-gaboets",
					Name:  "repo-template",
				},
			},
			{
				Owner:      "orang-gaboets",
				Name:       "existing-repo",
				Visibility: "private",
			},
		},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
		}},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{ResourceType: gitopsplan.ActionResourceTypeRepository, Operation: gitopsplan.ActionOperationCreate, ResourceID: repositoryResourceID("orang-gaboets", "new-repo"), Executable: true},
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   repositoryResourceID("orang-gaboets", "existing-repo"),
				Executable:   true,
				Changes: []gitopsplan.FieldChange{{
					Field: "visibility",
					From:  "public",
					To:    "private",
				}},
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeTeam,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   teamResourceID("platform"),
				Executable:   true,
				Changes: []gitopsplan.FieldChange{{
					Field: "description",
					From:  "",
					To:    "Updated",
				}},
			},
		},
	}
	plan.Normalize()

	repoSvc := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			switch repositoryResourceID(owner, repo) {
			case repositoryResourceID("orang-gaboets", "repo-template"):
				return githubRepository("orang-gaboets", "repo-template", false), nil, nil
			case repositoryResourceID("orang-gaboets", "existing-repo"):
				return nil, nil, githubNotFoundError("repository not found")
			default:
				t.Fatalf("unexpected repository lookup %s/%s", owner, repo)
				return nil, nil, nil
			}
		},
	}
	teamSvc := &testTeamService{
		getTeamBySlugFunc: func(_ context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
			if org != "orang-gaboets" || slug != "platform" {
				t.Fatalf("unexpected team lookup %s/%s", org, slug)
			}
			return nil, nil, githubNotFoundError("team not found")
		},
	}

	_, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(repoSvc), withTeamService(teamSvc)))
	if err == nil {
		t.Fatal("expected aggregated failure")
	}
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("expected invalid-field failure, got %v", err)
	}
	if !errors.Is(err, githubpkg.ErrNotFound) {
		t.Fatalf("expected not-found failure, got %v", err)
	}
	for _, want := range []string{
		"apply preflight failed for 3 action(s)",
		"create repository orang-gaboets/new-repo",
		"update repository orang-gaboets/existing-repo",
		"update team platform",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error to contain %q, got %v", want, err)
		}
	}
}

func githubRepository(owner, name string, isTemplate bool) *gh.Repository {
	return &gh.Repository{
		Name:       gh.String(name),
		Owner:      &gh.User{Login: gh.String(owner)},
		IsTemplate: gh.Bool(isTemplate),
	}
}

func githubTeam(id int64, slug, name, org string) *gh.Team {
	return &gh.Team{
		ID:   gh.Int64(id),
		Slug: gh.String(slug),
		Name: gh.String(name),
		Organization: &gh.Organization{
			Login: gh.String(org),
		},
	}
}

func githubUser(id int64, username string) *gh.User {
	return &gh.User{
		ID:    gh.Int64(id),
		Login: gh.String(username),
	}
}

func githubNotFoundError(message string) error {
	return githubAPIError(http.StatusNotFound, message)
}

func githubAPIError(statusCode int, message string) error {
	return &gh.ErrorResponse{
		Response: &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(strings.NewReader(message)),
		},
		Message: message,
	}
}
