package apply

import (
	"context"
	"errors"
	"reflect"
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
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			t.Fatal("invite creation should not run during check")
			return nil, nil, nil
		},
	}
	userSvc := &testUserService{
		getFunc: func(context.Context, string) (*gh.User, *gh.Response, error) {
			t.Fatal("username lookup should not run during check")
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
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: %v", err)
	}
}
