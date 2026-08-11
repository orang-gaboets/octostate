package apply

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	gh "github.com/google/go-github/v88/github"
	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/internal/testconfig"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestExecuteSkipsNonExecutableDrift(t *testing.T) {
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationDelete,
				ResourceID:   "orang-gaboets/extra-repo",
				Executable:   false,
				Message:      "extra repository orang-gaboets/extra-repo would require deletion",
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeTeamMember,
				Operation:    gitopsplan.ActionOperationRemove,
				ResourceID:   "platform/alice",
				Executable:   false,
				Message:      "extra team membership platform/alice would require removal",
			},
		},
	}
	plan.Normalize()

	result, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
		Organization: "orang-gaboets",
	}, &state.OrganizationState{Organization: "orang-gaboets"}, plan))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Executed) != 0 {
		t.Fatalf("expected no executed actions, got %#v", result.Executed)
	}
	if !reflect.DeepEqual(result.SkippedDrift, plan.Actions) {
		t.Fatalf("unexpected skipped drift:\n got %#v\nwant %#v", result.SkippedDrift, plan.Actions)
	}
}

func TestExecuteRejectsInvalidDesiredConfig(t *testing.T) {
	t.Parallel()

	plan := &gitopsplan.Report{Organization: "orang-gaboets"}
	plan.Normalize()

	_, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Owner:      "shared-platform",
			Name:       "octostate",
			Visibility: "private",
		}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Repositories: []config.TeamRepositorySpec{{
				Owner:      "other-org",
				Name:       "octostate-infra",
				Permission: "push",
			}},
		}},
	}, &state.OrganizationState{Organization: "orang-gaboets"}, plan))

	assertValidationErrorHasIssue(t, err, "repositories[0].owner", config.ValidationIssueCodeRepositoryOwnerScope)
	assertValidationErrorHasIssue(t, err, "teams[0].repositories[0].owner", config.ValidationIssueCodeRepositoryOwnerScope)
}

func TestCheckAndExecuteResolveUnnormalizedRepositoryOwners(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: " org-a ",
		Repositories: []config.RepositorySpec{{
			Name:       "service",
			Visibility: "private",
		}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Repositories: []config.TeamRepositorySpec{{
				Owner:      " ORG-A ",
				Name:       "service",
				Permission: "push",
			}},
		}},
	}
	plan := &gitopsplan.Report{
		Organization: "org-a",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   repositoryResourceID("org-a", "service"),
				Executable:   true,
				Changes:      []gitopsplan.FieldChange{{Field: "visibility", From: "public", To: "private"}},
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeTeamRepositoryPermission,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   teamRepoPermissionResourceID("platform", "org-a", "service"),
				Executable:   true,
				Changes:      []gitopsplan.FieldChange{{Field: "permission", From: "pull", To: "push"}},
			},
		},
	}
	plan.Normalize()
	actual := &state.OrganizationState{
		Organization: "org-a",
		Repositories: []state.Repository{{Owner: "org-a", Name: "service", Visibility: "public"}},
		Teams:        []state.Team{{ID: 1, Slug: "platform", Name: "Platform", Privacy: "closed"}},
	}

	var checkOwners []string
	checkRepoService := &testRepoService{
		getFunc: func(_ context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
			checkOwners = append(checkOwners, owner)
			if repo != "service" {
				t.Fatalf("unexpected check repository %q", repo)
			}
			return githubRepository("org-a", "service", false), nil, nil
		},
	}
	checkTeamService := &testTeamService{
		getTeamBySlugFunc: func(_ context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
			return githubTeam(1, slug, "Platform", org), nil, nil
		},
	}
	if _, err := Check(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(checkRepoService), withTeamService(checkTeamService))); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reflect.DeepEqual(checkOwners, []string{"org-a"}) {
		t.Fatalf("Check used unexpected repository owners: %#v", checkOwners)
	}

	var executeRepoOwners []string
	executeRepoService := &testRepoService{
		editFunc: func(_ context.Context, owner, repo string, _ *gh.Repository) (*gh.Repository, *gh.Response, error) {
			executeRepoOwners = append(executeRepoOwners, owner)
			if repo != "service" {
				t.Fatalf("unexpected execute repository %q", repo)
			}
			return githubRepository("org-a", "service", false), nil, nil
		},
	}
	var executeTeamOwners []string
	executeTeamService := &testTeamService{
		addTeamRepoBySlugFunc: func(_ context.Context, org, slug, owner, repo string, _ *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
			if org != "org-a" || slug != "platform" || repo != "service" {
				t.Fatalf("unexpected team repository target: %s/%s/%s/%s", org, slug, owner, repo)
			}
			executeTeamOwners = append(executeTeamOwners, owner)
			return &gh.Response{}, nil
		},
	}
	if _, err := Execute(context.Background(), testApplyOptions(desired, actual, plan, withRepoService(executeRepoService), withTeamService(executeTeamService))); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !reflect.DeepEqual(executeRepoOwners, []string{"org-a"}) {
		t.Fatalf("Execute used unexpected repository owners: %#v", executeRepoOwners)
	}
	if !reflect.DeepEqual(executeTeamOwners, []string{"org-a"}) {
		t.Fatalf("Execute used unexpected team repository owners: %#v", executeTeamOwners)
	}
}

func TestExecuteOrganizationMemberCreateAndUpdate(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []config.OrganizationMemberSpec{
			{Username: "alice", Role: "member"},
			{Username: "bob", Role: "admin"},
		},
	}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeOrganizationMember,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   organizationMemberResourceID("alice"),
				Executable:   true,
				Message:      "create organization member alice",
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeOrganizationMember,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   organizationMemberResourceID("bob"),
				Executable:   true,
				Message:      "update organization member bob",
				Changes: []gitopsplan.FieldChange{{
					Field: "role",
					From:  "member",
					To:    "admin",
				}},
			},
		},
	}
	plan.Normalize()

	var calls []struct {
		user string
		role string
	}
	orgSvc := &testOrganizationService{
		editOrgMembershipFunc: func(_ context.Context, user, org string, membership *gh.Membership) (*gh.Membership, *gh.Response, error) {
			if org != "orang-gaboets" {
				t.Fatalf("unexpected organization %q", org)
			}
			if membership == nil || membership.Role == nil {
				t.Fatalf("expected membership role payload, got %#v", membership)
			}
			calls = append(calls, struct {
				user string
				role string
			}{user: user, role: *membership.Role})
			return membership, nil, nil
		},
	}

	result, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withOrganizationService(orgSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantCalls := []struct {
		user string
		role string
	}{
		{user: "alice", role: "member"},
		{user: "bob", role: "admin"},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Fatalf("unexpected membership calls: got %#v want %#v", calls, wantCalls)
	}
	if !reflect.DeepEqual(result.Executed, plan.Actions) {
		t.Fatalf("unexpected executed actions:\n got %#v\nwant %#v", result.Executed, plan.Actions)
	}
}

func TestExecuteOrganizationMemberUnsupportedOperationFails(t *testing.T) {
	t.Parallel()

	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeOrganizationMember,
			Operation:    gitopsplan.ActionOperationDelete,
			ResourceID:   organizationMemberResourceID("alice"),
			Executable:   true,
			Message:      "delete organization member alice",
		}},
	}
	plan.Normalize()

	_, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []config.OrganizationMemberSpec{{
			Username: "alice",
			Role:     "member",
		}},
	}, &state.OrganizationState{Organization: "orang-gaboets"}, plan))
	if !strings.Contains(err.Error(), `unsupported organization member operation "delete"`) {
		t.Fatalf("expected unsupported organization member operation error, got %v", err)
	}
}

func TestExecuteRepositoryCreateAppliesExactSettingsAndTopics(t *testing.T) {
	t.Parallel()

	desiredRepo := config.RepositorySpec{
		Owner:        "orang-gaboets",
		Name:         "octostate",
		Visibility:   "private",
		Description:  "GitOps CLI",
		Homepage:     "https://example.com/octostate",
		Topics:       []string{"gitops"},
		AllowForking: false,
		Archived:     false,
		IsTemplate:   false,
		Template: config.TemplateSpec{
			Owner:              "orang-gaboets",
			Name:               "repo-template",
			IncludeAllBranches: true,
		},
	}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   repositoryResourceID(desiredRepo.Owner, desiredRepo.Name),
			Executable:   true,
			Message:      "create repository orang-gaboets/octostate",
		}},
	}
	plan.Normalize()

	var createCalls int
	var editCalls int
	var replaceTopicsCalls [][]string
	var listTemplateTopicsCalls int
	var createReq *gh.TemplateRepoRequest

	repoSvc := &testRepoService{
		createFromTemplateFunc: func(_ context.Context, templateOwner, templateRepo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
			createCalls++
			if templateOwner != desiredRepo.Template.Owner || templateRepo != desiredRepo.Template.Name {
				t.Fatalf("unexpected template target %s/%s", templateOwner, templateRepo)
			}
			createReq = req
			return &gh.Repository{}, nil, nil
		},
		listAllTopicsFunc: func(context.Context, string, string) ([]string, *gh.Response, error) {
			listTemplateTopicsCalls++
			return []string{"template-topic"}, nil, nil
		},
		editFunc: func(_ context.Context, owner, repo string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
			editCalls++
			if owner != desiredRepo.Owner || repo != desiredRepo.Name {
				t.Fatalf("unexpected edit target %s/%s", owner, repo)
			}
			if repository == nil || repository.Homepage == nil || *repository.Homepage != desiredRepo.Homepage {
				t.Fatalf("unexpected repository edit payload: %#v", repository)
			}
			if repository.AllowForking != nil {
				t.Fatalf("expected allow_forking to be omitted for private repository edit, got %#v", repository)
			}
			return &gh.Repository{}, nil, nil
		},
		replaceAllTopicsFunc: func(_ context.Context, owner, repo string, topics []string) ([]string, *gh.Response, error) {
			if owner != desiredRepo.Owner || repo != desiredRepo.Name {
				t.Fatalf("unexpected topics target %s/%s", owner, repo)
			}
			replaceTopicsCalls = append(replaceTopicsCalls, append([]string(nil), topics...))
			return topics, nil, nil
		},
	}

	result, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{desiredRepo},
	}, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if createCalls != 1 {
		t.Fatalf("expected create from template once, got %d", createCalls)
	}
	if editCalls != 1 {
		t.Fatalf("expected one repository edit call, got %d", editCalls)
	}
	if createReq == nil || createReq.Private == nil || !*createReq.Private || createReq.Name == nil || *createReq.Name != desiredRepo.Name {
		t.Fatalf("unexpected template request: %#v", createReq)
	}
	if listTemplateTopicsCalls != 0 {
		t.Fatalf("expected no template topic listing during apply create, got %d", listTemplateTopicsCalls)
	}
	if len(replaceTopicsCalls) == 0 {
		t.Fatal("expected topic replacement calls")
	}
	if len(replaceTopicsCalls) != 1 {
		t.Fatalf("expected exactly one topic replacement during apply create, got %d", len(replaceTopicsCalls))
	}
	if got := replaceTopicsCalls[len(replaceTopicsCalls)-1]; !reflect.DeepEqual(got, desiredRepo.Topics) {
		t.Fatalf("unexpected final topics replacement: got %#v want %#v", got, desiredRepo.Topics)
	}
	if len(result.Executed) != 1 || result.Executed[0].ResourceID != plan.Actions[0].ResourceID {
		t.Fatalf("unexpected executed actions: %#v", result.Executed)
	}
}

func TestExecuteRepositoryCreateOmitsUnmanagedOptionalFields(t *testing.T) {
	t.Parallel()

	desired := testconfig.LoadDesiredConfig(t, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: private
    topics: [gitops]
    template:
      owner: orang-gaboets
      name: repo-template
teams: []
invites: []
`)
	desiredRepo := desired.Repositories[0]
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   repositoryResourceID(desiredRepo.Owner, desiredRepo.Name),
			Executable:   true,
			Message:      "create repository orang-gaboets/octostate",
		}},
	}
	plan.Normalize()

	var createReq *gh.TemplateRepoRequest
	var editReq *gh.Repository
	repoSvc := &testRepoService{
		createFromTemplateFunc: func(_ context.Context, _, _ string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
			createReq = req
			return &gh.Repository{}, nil, nil
		},
		editFunc: func(_ context.Context, _, _ string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
			editReq = repository
			return &gh.Repository{}, nil, nil
		},
		replaceAllTopicsFunc: func(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
			return topics, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createReq == nil {
		t.Fatal("expected template creation request")
	}
	if createReq.Description != nil {
		t.Fatalf("expected unmanaged description to be omitted from create request, got %#v", createReq.Description)
	}
	if editReq == nil {
		t.Fatal("expected follow-up repository edit request")
	}
	if editReq.Private == nil || !*editReq.Private {
		t.Fatalf("expected private repository edit payload, got %#v", editReq)
	}
	if editReq.Description != nil || editReq.Homepage != nil || editReq.Archived != nil || editReq.IsTemplate != nil || editReq.AllowForking != nil {
		t.Fatalf("expected unmanaged optional fields to be omitted from edit payload, got %#v", editReq)
	}
}

func TestExecuteRepositoryCreateManagedZeroValuesAreApplied(t *testing.T) {
	t.Parallel()

	desired := testconfig.LoadDesiredConfig(t, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: public
    description: ""
    homepage: ""
    topics: [gitops]
    allow_forking: false
    archived: false
    is_template: false
    template:
      owner: orang-gaboets
      name: repo-template
teams: []
invites: []
`)
	desiredRepo := desired.Repositories[0]
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   repositoryResourceID(desiredRepo.Owner, desiredRepo.Name),
			Executable:   true,
			Message:      "create repository orang-gaboets/octostate",
		}},
	}
	plan.Normalize()

	var createReq *gh.TemplateRepoRequest
	var editReq *gh.Repository
	repoSvc := &testRepoService{
		createFromTemplateFunc: func(_ context.Context, _, _ string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
			createReq = req
			return &gh.Repository{}, nil, nil
		},
		editFunc: func(_ context.Context, _, _ string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
			editReq = repository
			return &gh.Repository{}, nil, nil
		},
		replaceAllTopicsFunc: func(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
			return topics, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if createReq == nil || createReq.Description == nil || *createReq.Description != "" {
		t.Fatalf("expected explicit empty description on create request, got %#v", createReq)
	}
	if editReq == nil {
		t.Fatal("expected follow-up repository edit request")
	}
	if editReq.Private == nil || *editReq.Private {
		t.Fatalf("expected public repository edit payload, got %#v", editReq)
	}
	if editReq.Description == nil || *editReq.Description != "" {
		t.Fatalf("expected explicit empty description in edit payload, got %#v", editReq)
	}
	if editReq.Homepage == nil || *editReq.Homepage != "" {
		t.Fatalf("expected explicit empty homepage in edit payload, got %#v", editReq)
	}
	if editReq.AllowForking == nil || *editReq.AllowForking {
		t.Fatalf("expected explicit false allow_forking in edit payload, got %#v", editReq)
	}
	if editReq.Archived == nil || *editReq.Archived {
		t.Fatalf("expected explicit false archived in edit payload, got %#v", editReq)
	}
	if editReq.IsTemplate == nil || *editReq.IsTemplate {
		t.Fatalf("expected explicit false is_template in edit payload, got %#v", editReq)
	}
}

func TestExecuteRepositoryUpdateTopicsOnlySkipsEdit(t *testing.T) {
	t.Parallel()

	desiredRepo := config.RepositorySpec{
		Owner:      "orang-gaboets",
		Name:       "octostate",
		Visibility: "private",
		Topics:     []string{"gitops", "go"},
	}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationUpdate,
			ResourceID:   repositoryResourceID(desiredRepo.Owner, desiredRepo.Name),
			Executable:   true,
			Message:      "update repository orang-gaboets/octostate",
			Changes: []gitopsplan.FieldChange{{
				Field: "topics",
			}},
		}},
	}
	plan.Normalize()

	editCalled := false
	var replacedTopics []string
	repoSvc := &testRepoService{
		editFunc: func(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			editCalled = true
			return &gh.Repository{}, nil, nil
		},
		replaceAllTopicsFunc: func(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
			replacedTopics = append([]string(nil), topics...)
			return topics, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{desiredRepo},
	}, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if editCalled {
		t.Fatal("edit should not be called for a topics-only update")
	}
	if !reflect.DeepEqual(replacedTopics, desiredRepo.Topics) {
		t.Fatalf("unexpected topics replacement: got %#v want %#v", replacedTopics, desiredRepo.Topics)
	}
}

func TestExecuteRepositoryUpdatePrivateRepoIgnoresAllowForkingChange(t *testing.T) {
	t.Parallel()

	desiredRepo := config.RepositorySpec{
		Owner:        "orang-gaboets",
		Name:         "octostate",
		Visibility:   "private",
		Description:  "Updated description",
		AllowForking: false,
	}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationUpdate,
			ResourceID:   repositoryResourceID(desiredRepo.Owner, desiredRepo.Name),
			Executable:   true,
			Message:      "update repository orang-gaboets/octostate",
			Changes: []gitopsplan.FieldChange{
				{Field: "allow_forking", From: true, To: false},
				{Field: "description", From: "", To: desiredRepo.Description},
			},
		}},
	}
	plan.Normalize()

	editCalled := false
	repoSvc := &testRepoService{
		editFunc: func(_ context.Context, owner, repo string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
			editCalled = true
			if owner != desiredRepo.Owner || repo != desiredRepo.Name {
				t.Fatalf("unexpected edit target %s/%s", owner, repo)
			}
			if repository == nil || repository.Description == nil || *repository.Description != desiredRepo.Description {
				t.Fatalf("unexpected repository edit payload: %#v", repository)
			}
			if repository.AllowForking != nil {
				t.Fatalf("expected allow_forking to be omitted for private repository update, got %#v", repository)
			}
			return &gh.Repository{}, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{desiredRepo},
	}, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !editCalled {
		t.Fatal("expected repository edit to be called")
	}
}

func TestExecuteRepositoryUpdateManagedZeroValuesAreApplied(t *testing.T) {
	t.Parallel()

	desired := testconfig.LoadDesiredConfig(t, `
organization: orang-gaboets
repositories:
  - name: octostate
    visibility: public
    description: ""
    homepage: ""
    topics: [gitops]
    allow_forking: false
    archived: false
    is_template: false
teams: []
invites: []
`)
	desiredRepo := desired.Repositories[0]
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationUpdate,
			ResourceID:   repositoryResourceID(desiredRepo.Owner, desiredRepo.Name),
			Executable:   true,
			Message:      "update repository orang-gaboets/octostate",
			Changes: []gitopsplan.FieldChange{
				{Field: "description", From: "CLI", To: ""},
				{Field: "homepage", From: "https://example.com/octostate", To: ""},
				{Field: "allow_forking", From: true, To: false},
				{Field: "archived", From: true, To: false},
				{Field: "is_template", From: true, To: false},
			},
		}},
	}
	plan.Normalize()

	var editReq *gh.Repository
	repoSvc := &testRepoService{
		editFunc: func(_ context.Context, _, _ string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
			editReq = repository
			return &gh.Repository{}, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if editReq == nil {
		t.Fatal("expected repository edit request")
	}
	if editReq.Private != nil {
		t.Fatalf("did not expect visibility update in edit payload, got %#v", editReq)
	}
	if editReq.Description == nil || *editReq.Description != "" {
		t.Fatalf("expected explicit empty description in edit payload, got %#v", editReq)
	}
	if editReq.Homepage == nil || *editReq.Homepage != "" {
		t.Fatalf("expected explicit empty homepage in edit payload, got %#v", editReq)
	}
	if editReq.AllowForking == nil || *editReq.AllowForking {
		t.Fatalf("expected explicit false allow_forking in edit payload, got %#v", editReq)
	}
	if editReq.Archived == nil || *editReq.Archived {
		t.Fatalf("expected explicit false archived in edit payload, got %#v", editReq)
	}
	if editReq.IsTemplate == nil || *editReq.IsTemplate {
		t.Fatalf("expected explicit false is_template in edit payload, got %#v", editReq)
	}
}

func TestExecuteRepositoryUpdateToTemplateRunsBeforeLaterSamePlanCreate(t *testing.T) {
	t.Parallel()

	templateRepo := config.RepositorySpec{
		Owner:      "orang-gaboets",
		Name:       "zzz-template",
		Visibility: "private",
	}
	templateRepo.SetManagedIsTemplate(true)

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{
			{
				Owner:      "orang-gaboets",
				Name:       "aaa-app",
				Visibility: "private",
				Template: config.TemplateSpec{
					Owner: "orang-gaboets",
					Name:  "zzz-template",
				},
			},
			templateRepo,
		},
	}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   repositoryResourceID("orang-gaboets", "zzz-template"),
				Executable:   true,
				Changes: []gitopsplan.FieldChange{{
					Field: "is_template",
					From:  false,
					To:    true,
				}},
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   repositoryResourceID("orang-gaboets", "aaa-app"),
				Executable:   true,
			},
		},
	}
	plan.Normalize()

	var events []string
	repoSvc := &testRepoService{
		editFunc: func(_ context.Context, owner, repo string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
			events = append(events, "edit:"+owner+"/"+repo)
			if owner == "orang-gaboets" && repo == "zzz-template" {
				if repository == nil || repository.IsTemplate == nil || !*repository.IsTemplate {
					t.Fatalf("expected template repository edit to set is_template=true, got %#v", repository)
				}
			}
			return &gh.Repository{}, nil, nil
		},
		createFromTemplateFunc: func(_ context.Context, templateOwner, templateRepo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
			if req == nil || req.Name == nil {
				t.Fatalf("expected repository name in create request, got %#v", req)
			}
			events = append(events, "create:"+templateOwner+"/"+templateRepo+"->"+*req.Name)
			return &gh.Repository{}, nil, nil
		},
		replaceAllTopicsFunc: func(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
			return topics, nil, nil
		},
	}

	result, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantEvents := []string{
		"edit:orang-gaboets/zzz-template",
		"create:orang-gaboets/zzz-template->aaa-app",
		"edit:orang-gaboets/aaa-app",
	}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("unexpected call sequence:\n got %#v\nwant %#v", events, wantEvents)
	}
	if !reflect.DeepEqual(result.Executed, plan.Actions) {
		t.Fatalf("unexpected executed actions:\n got %#v\nwant %#v", result.Executed, plan.Actions)
	}
}

func TestExecuteRepositoryUpdateFailsOnUnknownChangeField(t *testing.T) {
	t.Parallel()

	desiredRepo := config.RepositorySpec{
		Owner:      "orang-gaboets",
		Name:       "octostate",
		Visibility: "private",
	}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationUpdate,
			ResourceID:   repositoryResourceID(desiredRepo.Owner, desiredRepo.Name),
			Executable:   true,
			Changes: []gitopsplan.FieldChange{{
				Field: "template",
			}},
		}},
	}
	plan.Normalize()

	repoSvc := &testRepoService{
		editFunc: func(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			t.Fatal("repository edit should not be called for unsupported planner changes")
			return nil, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{desiredRepo},
	}, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), `unsupported repository change field "template"`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExecuteRepositoryCreateWithoutTemplateFailsBeforeWrites(t *testing.T) {
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   repositoryResourceID("orang-gaboets", "octostate"),
			Executable:   true,
			Message:      "create repository orang-gaboets/octostate",
		}},
	}
	plan.Normalize()

	repoSvc := &testRepoService{
		createFromTemplateFunc: func(context.Context, string, string, *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
			t.Fatal("create from template should not be called when template is missing")
			return nil, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []config.RepositorySpec{{
			Owner:      "orang-gaboets",
			Name:       "octostate",
			Visibility: "private",
		}},
	}, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc)))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cannot be created without a template") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteTeamCreatesResolveDependenciesAndInviteUsesCreatedTeamIDs(t *testing.T) {
	createdIDs := map[string]int64{}
	teamNames := map[string]string{
		"app":      "App",
		"platform": "Platform",
	}
	var invitationOpts *gh.CreateOrgInvitationOptions

	teamSvc := &testTeamService{
		getTeamBySlugFunc: func(_ context.Context, _, slug string) (*gh.Team, *gh.Response, error) {
			id, ok := createdIDs[slug]
			if !ok {
				return nil, nil, errors.New("team not created yet")
			}
			name := teamNames[slug]
			if name == "" {
				name = slug
			}
			return &gh.Team{ID: githubpkg.Ptr(id), Slug: githubpkg.Ptr(slug), Name: githubpkg.Ptr(name)}, nil, nil
		},
		createTeamFunc: func(_ context.Context, _ string, team gh.NewTeam) (*gh.Team, *gh.Response, error) {
			slug := strings.ToLower(strings.ReplaceAll(team.Name, " ", "-"))
			id := int64(len(createdIDs) + 100)
			createdIDs[slug] = id
			return &gh.Team{ID: githubpkg.Ptr(id), Slug: githubpkg.Ptr(slug), Name: githubpkg.Ptr(team.Name)}, nil, nil
		},
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(_ context.Context, org string, opts *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			if org != "orang-gaboets" {
				t.Fatalf("unexpected organization %q", org)
			}
			invitationOpts = opts
			return &gh.Invitation{}, nil, nil
		},
	}

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
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("app"), Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("platform"), Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "email:alice@example.com", Executable: true},
		},
	}
	plan.Normalize()

	result, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc), withOrganizationService(orgSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Executed) != 3 {
		t.Fatalf("unexpected executed action count: %#v", result.Executed)
	}
	if invitationOpts == nil {
		t.Fatal("expected invitation to be created")
	}
	if invitationOpts.Email == nil || *invitationOpts.Email != "alice@example.com" {
		t.Fatalf("unexpected invitation email: %#v", invitationOpts)
	}
	if invitationOpts.Role == nil || *invitationOpts.Role != "direct_member" {
		t.Fatalf("unexpected invitation role: %#v", invitationOpts)
	}
	if !reflect.DeepEqual(invitationOpts.TeamID, []int64{createdIDs["platform"], createdIDs["app"]}) {
		t.Fatalf("unexpected invitation team IDs: got %#v want %#v", invitationOpts.TeamID, []int64{createdIDs["platform"], createdIDs["app"]})
	}
}

func TestExecuteInviteCreateByUsernameResolvesUserID(t *testing.T) {
	var invitedUserID *int64
	userSvc := &testUserService{
		getFunc: func(_ context.Context, username string) (*gh.User, *gh.Response, error) {
			if username != "octocat" {
				t.Fatalf("unexpected username %q", username)
			}
			return &gh.User{ID: githubpkg.Ptr(int64(42))}, nil, nil
		},
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(_ context.Context, _ string, opts *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			if opts == nil || opts.InviteeID == nil {
				t.Fatalf("unexpected invitation options: %#v", opts)
			}
			invitedUserID = githubpkg.Ptr(*opts.InviteeID)
			return &gh.Invitation{}, nil, nil
		},
	}

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites:      []config.InviteSpec{inviteByUsername("octocat", "direct_member")},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeInvite,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   "username:octocat",
		Executable:   true,
	}}}
	plan.Normalize()

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withOrganizationService(orgSvc), withUserService(userSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if invitedUserID == nil || *invitedUserID != 42 {
		t.Fatalf("unexpected invited user ID: %#v", invitedUserID)
	}
}

func TestExecuteInviteCreateByUserIDAndEmail(t *testing.T) {
	tests := []struct {
		name   string
		invite config.InviteSpec
		id     int64
		email  string
	}{
		{name: "user ID", invite: inviteByUserID(88, "direct_member"), id: 88},
		{name: "email", invite: inviteByEmail("bob@example.com", "direct_member"), email: "bob@example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured *gh.CreateOrgInvitationOptions
			userSvc := &testUserService{
				getFunc: func(context.Context, string) (*gh.User, *gh.Response, error) {
					t.Fatal("username lookup should not run for email or user_id invites")
					return nil, nil, nil
				},
			}
			orgSvc := &testOrganizationService{
				createOrgInvitationFunc: func(_ context.Context, _ string, opts *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
					captured = opts
					return &gh.Invitation{}, nil, nil
				},
			}
			resourceID, err := desiredInviteResourceID(tt.invite)
			if err != nil {
				t.Fatalf("unexpected resource ID error: %v", err)
			}
			plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{{
				ResourceType: gitopsplan.ActionResourceTypeInvite,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   resourceID,
				Executable:   true,
			}}}
			plan.Normalize()

			_, err = Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
				Organization: "orang-gaboets",
				Invites:      []config.InviteSpec{tt.invite},
			}, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withOrganizationService(orgSvc), withUserService(userSvc)))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if captured == nil {
				t.Fatal("expected invitation request")
			}
			if tt.id > 0 {
				if captured.InviteeID == nil || *captured.InviteeID != tt.id {
					t.Fatalf("unexpected invitee ID: %#v", captured.InviteeID)
				}
			}
			if tt.email != "" {
				if captured.Email == nil || *captured.Email != tt.email {
					t.Fatalf("unexpected email: %#v", captured.Email)
				}
			}
		})
	}
}

func TestExecuteInviteUsernamePreResolutionStartsAtFirstInviteBoundary(t *testing.T) {
	var (
		mu     sync.Mutex
		events []string
	)
	recordEvent := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	}

	userSvc := &testUserService{
		getFunc: func(_ context.Context, username string) (*gh.User, *gh.Response, error) {
			recordEvent("lookup:" + username)
			return &gh.User{ID: githubpkg.Ptr(int64(42))}, nil, nil
		},
	}
	orgSvc := &testOrganizationService{
		editOrgMembershipFunc: func(context.Context, string, string, *gh.Membership) (*gh.Membership, *gh.Response, error) {
			recordEvent("member")
			return &gh.Membership{}, nil, nil
		},
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			recordEvent("invite")
			return &gh.Invitation{}, nil, nil
		},
	}

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []config.OrganizationMemberSpec{{
			Username: "alice",
			Role:     "member",
		}},
		Invites: []config.InviteSpec{inviteByUsername("octocat", "direct_member")},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeOrganizationMember, Operation: gitopsplan.ActionOperationCreate, ResourceID: organizationMemberResourceID("alice"), Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "username:octocat", Executable: true},
	}}
	plan.Normalize()

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withOrganizationService(orgSvc), withUserService(userSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantEvents := []string{"member", "lookup:octocat", "invite"}
	if !reflect.DeepEqual(events, wantEvents) {
		t.Fatalf("unexpected event order:\n got %#v\nwant %#v", events, wantEvents)
	}
}

func TestExecuteInviteUsernamePreResolutionBoundsConcurrency(t *testing.T) {
	usernames := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta", "theta", "iota", "kappa"}
	desiredInvites := make([]config.InviteSpec, 0, len(usernames))
	actions := make([]gitopsplan.Action, 0, len(usernames))
	userIDs := make(map[string]int64, len(usernames))
	for i, username := range usernames {
		invite := inviteByUsername(username, "direct_member")
		desiredInvites = append(desiredInvites, invite)
		resourceID, err := desiredInviteResourceID(invite)
		if err != nil {
			t.Fatalf("unexpected resource ID error: %v", err)
		}
		actions = append(actions, gitopsplan.Action{
			ResourceType: gitopsplan.ActionResourceTypeInvite,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   resourceID,
			Executable:   true,
		})
		userIDs[inviteUsernameKey(username)] = int64(i + 1)
	}

	var (
		lookupCalls     atomic.Int64
		currentLookups  atomic.Int64
		maxLookups      atomic.Int64
		invitationCalls atomic.Int64
	)
	started := make(chan struct{}, len(usernames))
	release := make(chan struct{})

	userSvc := &testUserService{
		getFunc: func(ctx context.Context, username string) (*gh.User, *gh.Response, error) {
			lookupCalls.Add(1)
			current := currentLookups.Add(1)
			updateAtomicMax(&maxLookups, current)
			defer currentLookups.Add(-1)

			started <- struct{}{}
			select {
			case <-release:
			case <-ctx.Done():
				return nil, nil, ctx.Err()
			}

			userID := userIDs[inviteUsernameKey(username)]
			return &gh.User{ID: githubpkg.Ptr(userID)}, nil, nil
		},
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(_ context.Context, _ string, opts *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			if opts == nil || opts.InviteeID == nil || *opts.InviteeID <= 0 {
				t.Fatalf("unexpected invitation options: %#v", opts)
			}
			invitationCalls.Add(1)
			return &gh.Invitation{}, nil, nil
		},
	}

	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: actions}
	plan.Normalize()

	done := make(chan error, 1)
	go func() {
		_, execErr := Execute(context.Background(), testApplyOptions(config.OrganizationConfig{
			Organization: "orang-gaboets",
			Invites:      desiredInvites,
		}, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withOrganizationService(orgSvc), withUserService(userSvc)))
		done <- execErr
	}()

	waitForSignals(t, started, inviteUsernameResolutionConcurrency, "concurrent username lookups to start")
	if got := maxLookups.Load(); got > inviteUsernameResolutionConcurrency {
		t.Fatalf("unexpected concurrent username lookups: got %d want <= %d", got, inviteUsernameResolutionConcurrency)
	}

	close(release)

	if err := waitForError(t, done, "execute to finish"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got, want := lookupCalls.Load(), int64(len(usernames)); got != want {
		t.Fatalf("unexpected username lookup count: got %d want %d", got, want)
	}
	if got, want := invitationCalls.Load(), int64(len(desiredInvites)); got != want {
		t.Fatalf("unexpected invitation count: got %d want %d", got, want)
	}
	if got := maxLookups.Load(); got == 0 || got > inviteUsernameResolutionConcurrency {
		t.Fatalf("unexpected maximum concurrent username lookups: %d", got)
	}
}

func TestExecuteInviteUsernamePreResolutionReturnsFirstSeenLookupError(t *testing.T) {
	firstErr := errors.New("first lookup failed")
	secondErr := errors.New("second lookup failed")
	firstReady := make(chan struct{})
	releaseFirst := make(chan struct{})

	userSvc := &testUserService{
		getFunc: func(_ context.Context, username string) (*gh.User, *gh.Response, error) {
			switch inviteUsernameKey(username) {
			case "first":
				close(firstReady)
				<-releaseFirst
				return nil, nil, firstErr
			case "second":
				<-firstReady
				close(releaseFirst)
				return nil, nil, secondErr
			default:
				return nil, nil, errors.New("unexpected username")
			}
		},
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			t.Fatal("invite creation should not run after pre-resolution failure")
			return nil, nil, nil
		},
	}

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []config.InviteSpec{
			inviteByUsername("first", "direct_member"),
			inviteByUsername("second", "direct_member"),
		},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "username:first", Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "username:second", Executable: true},
	}}
	plan.Normalize()

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withOrganizationService(orgSvc), withUserService(userSvc)))
	if !errors.Is(err, firstErr) {
		t.Fatalf("unexpected error: got %v want %v", err, firstErr)
	}
	if !strings.Contains(err.Error(), "create invite username:first") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExecuteInviteUsernamePreResolutionCancelsSiblingLookups(t *testing.T) {
	wantErr := errors.New("lookup failed")
	secondStarted := make(chan struct{})
	secondCanceled := make(chan struct{})

	userSvc := &testUserService{
		getFunc: func(ctx context.Context, username string) (*gh.User, *gh.Response, error) {
			switch inviteUsernameKey(username) {
			case "first":
				<-secondStarted
				return nil, nil, wantErr
			case "second":
				close(secondStarted)
				<-ctx.Done()
				close(secondCanceled)
				return nil, nil, ctx.Err()
			default:
				return nil, nil, errors.New("unexpected username")
			}
		},
	}
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			t.Fatal("invite creation should not run when pre-resolution fails")
			return nil, nil, nil
		},
	}

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites: []config.InviteSpec{
			inviteByUsername("first", "direct_member"),
			inviteByUsername("second", "direct_member"),
		},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "username:first", Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "username:second", Executable: true},
	}}
	plan.Normalize()

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withOrganizationService(orgSvc), withUserService(userSvc)))
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}

	waitForSignal(t, secondCanceled, "sibling lookup cancellation")
}

func TestExecuteTeamUpdateClearsParent(t *testing.T) {
	removedParent := false
	teamSvc := &testTeamService{
		editTeamBySlugFunc: func(_ context.Context, org, slug string, team gh.NewTeam, removeParent bool) (*gh.Team, *gh.Response, error) {
			if org != "orang-gaboets" || slug != "platform" {
				t.Fatalf("unexpected edit target %s/%s", org, slug)
			}
			removedParent = removeParent
			return &gh.Team{ID: githubpkg.Ptr(int64(10)), Slug: githubpkg.Ptr(slug), Name: githubpkg.Ptr(team.Name)}, nil, nil
		},
	}

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams: []config.TeamSpec{{
			Slug:       "platform",
			Name:       "Platform",
			Privacy:    "closed",
			ParentSlug: "",
		}},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeTeam,
		Operation:    gitopsplan.ActionOperationUpdate,
		ResourceID:   teamResourceID("platform"),
		Executable:   true,
		Changes:      []gitopsplan.FieldChange{{Field: "name"}, {Field: "parent_slug"}},
	}}}
	plan.Normalize()

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets", Teams: []state.Team{{ID: 10, Slug: "platform"}}}, plan, withTeamService(teamSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !removedParent {
		t.Fatal("expected removeParent to be true")
	}
}

func TestExecuteTeamUpdateFailsOnUnknownChangeField(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
		}},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeTeam,
		Operation:    gitopsplan.ActionOperationUpdate,
		ResourceID:   teamResourceID("platform"),
		Executable:   true,
		Changes:      []gitopsplan.FieldChange{{Field: "members"}},
	}}}
	plan.Normalize()

	teamSvc := &testTeamService{
		editTeamBySlugFunc: func(context.Context, string, string, gh.NewTeam, bool) (*gh.Team, *gh.Response, error) {
			t.Fatal("team edit should not be called for unsupported planner changes")
			return nil, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{
		Organization: "orang-gaboets",
		Teams:        []state.Team{{ID: 10, Slug: "platform"}},
	}, plan, withTeamService(teamSvc)))
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), `unsupported team change field "members"`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExecuteTeamMembershipAndRepoPermissionCreateAndUpdate(t *testing.T) {
	membershipCalls := 0
	repoPermissionCalls := 0
	teamSvc := &testTeamService{
		addTeamMembershipBySlugFunc: func(_ context.Context, org, slug, user string, opts *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
			membershipCalls++
			if org != "orang-gaboets" || slug != "platform" || user != "alice" || opts == nil || opts.Role != "maintainer" {
				return nil, nil, errors.New("unexpected membership call")
			}
			return &gh.Membership{}, nil, nil
		},
		addTeamRepoBySlugFunc: func(_ context.Context, org, slug, owner, repo string, opts *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
			repoPermissionCalls++
			if org != "orang-gaboets" || slug != "platform" || owner != "orang-gaboets" || repo != "octostate" || opts == nil || opts.Permission != "push" {
				return nil, errors.New("unexpected repo permission call")
			}
			return &gh.Response{}, nil
		},
	}

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []config.OrganizationMemberSpec{{
			Username: "alice",
			Role:     "member",
		}},
		Teams: []config.TeamSpec{{
			Slug:         "platform",
			Name:         "Platform",
			Privacy:      "closed",
			Members:      []config.TeamMemberSpec{{Username: "alice", Role: "maintainer"}},
			Repositories: []config.TeamRepositorySpec{{Owner: "orang-gaboets", Name: "octostate", Permission: "push"}},
		}},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeTeamMember, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamMemberResourceID("platform", "alice"), Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeTeamMember, Operation: gitopsplan.ActionOperationUpdate, ResourceID: teamMemberResourceID("platform", "alice"), Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeTeamRepositoryPermission, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamRepoPermissionResourceID("platform", "orang-gaboets", "octostate"), Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeTeamRepositoryPermission, Operation: gitopsplan.ActionOperationUpdate, ResourceID: teamRepoPermissionResourceID("platform", "orang-gaboets", "octostate"), Executable: true},
	}}
	plan.Normalize()

	result, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if membershipCalls != 2 {
		t.Fatalf("expected two membership calls, got %d", membershipCalls)
	}
	if repoPermissionCalls != 2 {
		t.Fatalf("expected two repository permission calls, got %d", repoPermissionCalls)
	}
	if len(result.Executed) != 4 {
		t.Fatalf("unexpected executed actions: %#v", result.Executed)
	}
}

func TestExecuteTeamMembershipRejectsUnsupportedOperation(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Members: []config.OrganizationMemberSpec{{
			Username: "alice",
			Role:     "member",
		}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Members: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
		}},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeTeamMember,
		Operation:    gitopsplan.ActionOperationRemove,
		ResourceID:   teamMemberResourceID("platform", "alice"),
		Executable:   true,
	}}}
	plan.Normalize()

	teamSvc := &testTeamService{
		addTeamMembershipBySlugFunc: func(context.Context, string, string, string, *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
			t.Fatal("team membership API should not be called for unsupported operations")
			return nil, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc)))
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), `unsupported team member operation "remove"`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExecuteTeamRepositoryPermissionRejectsUnsupportedOperation(t *testing.T) {
	t.Parallel()

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
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{{
		ResourceType: gitopsplan.ActionResourceTypeTeamRepositoryPermission,
		Operation:    gitopsplan.ActionOperationRemove,
		ResourceID:   teamRepoPermissionResourceID("platform", "orang-gaboets", "octostate"),
		Executable:   true,
	}}}
	plan.Normalize()

	teamSvc := &testTeamService{
		addTeamRepoBySlugFunc: func(context.Context, string, string, string, string, *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
			t.Fatal("team repository permission API should not be called for unsupported operations")
			return nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withTeamService(teamSvc)))
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), `unsupported team repository permission operation "remove"`) {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestExecuteStopsOnFirstFailure(t *testing.T) {
	wantErr := errors.New("invite failed")
	orgSvc := &testOrganizationService{
		createOrgInvitationFunc: func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
			return nil, nil, wantErr
		},
	}
	teamSvc := &testTeamService{
		addTeamMembershipBySlugFunc: func(context.Context, string, string, string, *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
			t.Fatal("membership update should not run after earlier failure")
			return nil, nil, nil
		},
	}

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Invites:      []config.InviteSpec{inviteByEmail("alice@example.com", "direct_member")},
		Members: []config.OrganizationMemberSpec{{
			Username: "alice",
			Role:     "member",
		}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Members: []config.TeamMemberSpec{{Username: "alice", Role: "maintainer"}},
		}},
	}
	plan := &gitopsplan.Report{Organization: "orang-gaboets", Actions: []gitopsplan.Action{
		{ResourceType: gitopsplan.ActionResourceTypeInvite, Operation: gitopsplan.ActionOperationCreate, ResourceID: "email:alice@example.com", Executable: true},
		{ResourceType: gitopsplan.ActionResourceTypeTeamMember, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamMemberResourceID("platform", "alice"), Executable: true},
	}}
	plan.Normalize()

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withOrganizationService(orgSvc), withTeamService(teamSvc)))
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
}

func TestExecuteFailsWhenExecutableTeamCreatesAreNotContiguous(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "orang-gaboets",
		Teams: []config.TeamSpec{
			{Slug: "app", Name: "App", Privacy: "closed", ParentSlug: "platform"},
			{Slug: "platform", Name: "Platform", Privacy: "closed"},
		},
		Repositories: []config.RepositorySpec{{
			Owner: "orang-gaboets",
			Name:  "octostate",
			Template: config.TemplateSpec{
				Owner: "orang-gaboets",
				Name:  "repo-template",
			},
			Visibility: "private",
		}},
	}
	plan := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("platform"), Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeRepository, Operation: gitopsplan.ActionOperationCreate, ResourceID: repositoryResourceID("orang-gaboets", "octostate"), Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationCreate, ResourceID: teamResourceID("app"), Executable: true},
		},
	}

	repoSvc := &testRepoService{
		createFromTemplateFunc: func(context.Context, string, string, *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
			t.Fatal("executor should reject invalid action ordering before applying writes")
			return nil, nil, nil
		},
	}
	teamSvc := &testTeamService{
		createTeamFunc: func(context.Context, string, gh.NewTeam) (*gh.Team, *gh.Response, error) {
			t.Fatal("executor should reject invalid action ordering before applying writes")
			return nil, nil, nil
		},
	}

	_, err := Execute(context.Background(), testApplyOptions(desired, &state.OrganizationState{Organization: "orang-gaboets"}, plan, withRepoService(repoSvc), withTeamService(teamSvc)))
	if !errors.Is(err, githubpkg.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "executable team create actions must be contiguous") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func testApplyOptions(desired config.OrganizationConfig, actual *state.OrganizationState, plan *gitopsplan.Report, tweaks ...func(*Options)) Options {
	opts := Options{
		Desired:             desired,
		Actual:              actual,
		Plan:                plan,
		OrganizationService: &testOrganizationService{},
		RepositoryService:   &testRepoService{},
		TeamService:         &testTeamService{},
		UserService:         &testUserService{},
	}
	for _, tweak := range tweaks {
		tweak(&opts)
	}
	return opts
}

func withOrganizationService(service organizations.Service) func(*Options) {
	return func(opt *Options) { opt.OrganizationService = service }
}

func withRepoService(service interface {
	CreateFromTemplate(context.Context, string, string, *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error)
	Delete(context.Context, string, string) (*gh.Response, error)
	Edit(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error)
	Get(context.Context, string, string) (*gh.Repository, *gh.Response, error)
	ListByOrg(context.Context, string, *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error)
	ReplaceAllTopics(context.Context, string, string, []string) ([]string, *gh.Response, error)
	ListAllTopics(context.Context, string, string) ([]string, *gh.Response, error)
}) func(*Options) {
	return func(opt *Options) { opt.RepositoryService = service }
}

func withTeamService(service interface {
	CreateTeam(context.Context, string, gh.NewTeam) (*gh.Team, *gh.Response, error)
	EditTeamBySlug(context.Context, string, string, gh.NewTeam, bool) (*gh.Team, *gh.Response, error)
	DeleteTeamBySlug(context.Context, string, string) (*gh.Response, error)
	GetTeamBySlug(context.Context, string, string) (*gh.Team, *gh.Response, error)
	AddTeamMembershipBySlug(context.Context, string, string, string, *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error)
	RemoveTeamMembershipBySlug(context.Context, string, string, string) (*gh.Response, error)
	ListTeamReposBySlug(context.Context, string, string, *gh.ListOptions) ([]*gh.Repository, *gh.Response, error)
	AddTeamRepoBySlug(context.Context, string, string, string, string, *gh.TeamAddTeamRepoOptions) (*gh.Response, error)
	RemoveTeamRepoBySlug(context.Context, string, string, string, string) (*gh.Response, error)
	ListTeamMembersBySlug(context.Context, string, string, *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error)
	ListTeams(context.Context, string, *gh.ListOptions) ([]*gh.Team, *gh.Response, error)
}) func(*Options) {
	return func(opt *Options) { opt.TeamService = service }
}

func withUserService(service interface {
	Get(context.Context, string) (*gh.User, *gh.Response, error)
	GetByID(context.Context, int64) (*gh.User, *gh.Response, error)
}) func(*Options) {
	return func(opt *Options) { opt.UserService = service }
}

func inviteByEmail(email, role string, teamSlugs ...string) config.InviteSpec {
	return config.InviteSpec{
		Email:     config.OptionalString{Present: true, Value: email},
		Role:      role,
		TeamSlugs: append([]string(nil), teamSlugs...),
	}
}

func inviteByUsername(username, role string) config.InviteSpec {
	return config.InviteSpec{
		Username: config.OptionalString{Present: true, Value: username},
		Role:     role,
	}
}

func inviteByUserID(userID int64, role string) config.InviteSpec {
	return config.InviteSpec{
		UserID: config.OptionalInt64{Present: true, Value: userID},
		Role:   role,
	}
}

type testOrganizationService struct {
	createOrgInvitationFunc    func(context.Context, string, *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error)
	editOrgMembershipFunc      func(context.Context, string, string, *gh.Membership) (*gh.Membership, *gh.Response, error)
	getFunc                    func(context.Context, string) (*gh.Organization, *gh.Response, error)
	listMembersFunc            func(context.Context, string, *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error)
	listPendingInvitationsFunc func(context.Context, string, *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error)
	listOrgInvitationTeamsFunc func(context.Context, string, string, *gh.ListOptions) ([]*gh.Team, *gh.Response, error)
}

func (m *testOrganizationService) CreateOrgInvitation(ctx context.Context, org string, opts *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	if m.createOrgInvitationFunc != nil {
		return m.createOrgInvitationFunc(ctx, org, opts)
	}
	return &gh.Invitation{}, nil, nil
}
func (m *testOrganizationService) EditOrgMembership(ctx context.Context, user, org string, membership *gh.Membership) (*gh.Membership, *gh.Response, error) {
	if m.editOrgMembershipFunc != nil {
		return m.editOrgMembershipFunc(ctx, user, org, membership)
	}
	return &gh.Membership{}, nil, nil
}
func (m *testOrganizationService) Get(ctx context.Context, org string) (*gh.Organization, *gh.Response, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, org)
	}
	return &gh.Organization{}, nil, nil
}
func (m *testOrganizationService) ListMembers(ctx context.Context, org string, opts *gh.ListMembersOptions) ([]*gh.User, *gh.Response, error) {
	if m.listMembersFunc != nil {
		return m.listMembersFunc(ctx, org, opts)
	}
	return nil, nil, nil
}
func (m *testOrganizationService) ListPendingOrgInvitations(ctx context.Context, org string, opts *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
	if m.listPendingInvitationsFunc != nil {
		return m.listPendingInvitationsFunc(ctx, org, opts)
	}
	return nil, nil, nil
}
func (m *testOrganizationService) ListOrgInvitationTeams(ctx context.Context, org, invitationID string, opts *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	if m.listOrgInvitationTeamsFunc != nil {
		return m.listOrgInvitationTeamsFunc(ctx, org, invitationID, opts)
	}
	return nil, nil, nil
}

type testRepoService struct {
	createFromTemplateFunc func(context.Context, string, string, *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error)
	deleteFunc             func(context.Context, string, string) (*gh.Response, error)
	editFunc               func(context.Context, string, string, *gh.Repository) (*gh.Repository, *gh.Response, error)
	getFunc                func(context.Context, string, string) (*gh.Repository, *gh.Response, error)
	listByOrgFunc          func(context.Context, string, *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error)
	replaceAllTopicsFunc   func(context.Context, string, string, []string) ([]string, *gh.Response, error)
	listAllTopicsFunc      func(context.Context, string, string) ([]string, *gh.Response, error)
}

func (m *testRepoService) CreateFromTemplate(ctx context.Context, templateOwner, templateRepo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	if m.createFromTemplateFunc != nil {
		return m.createFromTemplateFunc(ctx, templateOwner, templateRepo, req)
	}
	return &gh.Repository{}, nil, nil
}
func (m *testRepoService) Delete(ctx context.Context, owner, repo string) (*gh.Response, error) {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, owner, repo)
	}
	return &gh.Response{}, nil
}
func (m *testRepoService) Edit(ctx context.Context, owner, repo string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
	if m.editFunc != nil {
		return m.editFunc(ctx, owner, repo, repository)
	}
	return &gh.Repository{}, nil, nil
}
func (m *testRepoService) Get(ctx context.Context, owner, repo string) (*gh.Repository, *gh.Response, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, owner, repo)
	}
	return &gh.Repository{}, nil, nil
}
func (m *testRepoService) ListByOrg(ctx context.Context, org string, opts *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
	if m.listByOrgFunc != nil {
		return m.listByOrgFunc(ctx, org, opts)
	}
	return nil, nil, nil
}
func (m *testRepoService) ReplaceAllTopics(ctx context.Context, owner, repo string, topics []string) ([]string, *gh.Response, error) {
	if m.replaceAllTopicsFunc != nil {
		return m.replaceAllTopicsFunc(ctx, owner, repo, topics)
	}
	return topics, nil, nil
}
func (m *testRepoService) ListAllTopics(ctx context.Context, owner, repo string) ([]string, *gh.Response, error) {
	if m.listAllTopicsFunc != nil {
		return m.listAllTopicsFunc(ctx, owner, repo)
	}
	return nil, nil, nil
}

type testTeamService struct {
	createTeamFunc              func(context.Context, string, gh.NewTeam) (*gh.Team, *gh.Response, error)
	editTeamBySlugFunc          func(context.Context, string, string, gh.NewTeam, bool) (*gh.Team, *gh.Response, error)
	deleteTeamBySlugFunc        func(context.Context, string, string) (*gh.Response, error)
	getTeamBySlugFunc           func(context.Context, string, string) (*gh.Team, *gh.Response, error)
	addTeamMembershipBySlugFunc func(context.Context, string, string, string, *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error)
	removeTeamMembershipFunc    func(context.Context, string, string, string) (*gh.Response, error)
	listTeamReposBySlugFunc     func(context.Context, string, string, *gh.ListOptions) ([]*gh.Repository, *gh.Response, error)
	addTeamRepoBySlugFunc       func(context.Context, string, string, string, string, *gh.TeamAddTeamRepoOptions) (*gh.Response, error)
	removeTeamRepoBySlugFunc    func(context.Context, string, string, string, string) (*gh.Response, error)
	listTeamMembersBySlugFunc   func(context.Context, string, string, *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error)
	listTeamsFunc               func(context.Context, string, *gh.ListOptions) ([]*gh.Team, *gh.Response, error)
}

func (m *testTeamService) CreateTeam(ctx context.Context, org string, team gh.NewTeam) (*gh.Team, *gh.Response, error) {
	if m.createTeamFunc != nil {
		return m.createTeamFunc(ctx, org, team)
	}
	return &gh.Team{}, nil, nil
}
func (m *testTeamService) EditTeamBySlug(ctx context.Context, org, slug string, team gh.NewTeam, removeParent bool) (*gh.Team, *gh.Response, error) {
	if m.editTeamBySlugFunc != nil {
		return m.editTeamBySlugFunc(ctx, org, slug, team, removeParent)
	}
	return &gh.Team{}, nil, nil
}
func (m *testTeamService) DeleteTeamBySlug(ctx context.Context, org, slug string) (*gh.Response, error) {
	if m.deleteTeamBySlugFunc != nil {
		return m.deleteTeamBySlugFunc(ctx, org, slug)
	}
	return &gh.Response{}, nil
}
func (m *testTeamService) GetTeamBySlug(ctx context.Context, org, slug string) (*gh.Team, *gh.Response, error) {
	if m.getTeamBySlugFunc != nil {
		return m.getTeamBySlugFunc(ctx, org, slug)
	}
	return &gh.Team{}, nil, nil
}
func (m *testTeamService) AddTeamMembershipBySlug(ctx context.Context, org, slug, user string, opts *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
	if m.addTeamMembershipBySlugFunc != nil {
		return m.addTeamMembershipBySlugFunc(ctx, org, slug, user, opts)
	}
	return &gh.Membership{}, nil, nil
}
func (m *testTeamService) RemoveTeamMembershipBySlug(ctx context.Context, org, slug, user string) (*gh.Response, error) {
	if m.removeTeamMembershipFunc != nil {
		return m.removeTeamMembershipFunc(ctx, org, slug, user)
	}
	return &gh.Response{}, nil
}
func (m *testTeamService) ListTeamReposBySlug(ctx context.Context, org, slug string, opts *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
	if m.listTeamReposBySlugFunc != nil {
		return m.listTeamReposBySlugFunc(ctx, org, slug, opts)
	}
	return nil, nil, nil
}
func (m *testTeamService) AddTeamRepoBySlug(ctx context.Context, org, slug, owner, repo string, opts *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
	if m.addTeamRepoBySlugFunc != nil {
		return m.addTeamRepoBySlugFunc(ctx, org, slug, owner, repo, opts)
	}
	return &gh.Response{}, nil
}
func (m *testTeamService) RemoveTeamRepoBySlug(ctx context.Context, org, slug, owner, repo string) (*gh.Response, error) {
	if m.removeTeamRepoBySlugFunc != nil {
		return m.removeTeamRepoBySlugFunc(ctx, org, slug, owner, repo)
	}
	return &gh.Response{}, nil
}
func (m *testTeamService) ListTeamMembersBySlug(ctx context.Context, org, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
	if m.listTeamMembersBySlugFunc != nil {
		return m.listTeamMembersBySlugFunc(ctx, org, slug, opts)
	}
	return nil, nil, nil
}
func (m *testTeamService) ListTeams(ctx context.Context, org string, opts *gh.ListOptions) ([]*gh.Team, *gh.Response, error) {
	if m.listTeamsFunc != nil {
		return m.listTeamsFunc(ctx, org, opts)
	}
	return nil, nil, nil
}

type testUserService struct {
	getFunc     func(context.Context, string) (*gh.User, *gh.Response, error)
	getByIDFunc func(context.Context, int64) (*gh.User, *gh.Response, error)
}

func (m *testUserService) Get(ctx context.Context, username string) (*gh.User, *gh.Response, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, username)
	}
	return &gh.User{}, nil, nil
}
func (m *testUserService) GetByID(ctx context.Context, id int64) (*gh.User, *gh.Response, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return &gh.User{}, nil, nil
}

func updateAtomicMax(target *atomic.Int64, candidate int64) {
	for {
		current := target.Load()
		if candidate <= current {
			return
		}
		if target.CompareAndSwap(current, candidate) {
			return
		}
	}
}

const concurrencyTestTimeout = 2 * time.Second

func waitForSignals(t *testing.T, ch <-chan struct{}, count int, description string) {
	t.Helper()

	timer := time.NewTimer(concurrencyTestTimeout)
	defer timer.Stop()

	for range count {
		select {
		case <-ch:
		case <-timer.C:
			t.Fatalf("timed out waiting for %s", description)
		}
	}
}

func waitForSignal(t *testing.T, ch <-chan struct{}, description string) {
	t.Helper()

	timer := time.NewTimer(concurrencyTestTimeout)
	defer timer.Stop()

	select {
	case <-ch:
	case <-timer.C:
		t.Fatalf("timed out waiting for %s", description)
	}
}

func waitForError(t *testing.T, ch <-chan error, description string) error {
	t.Helper()

	timer := time.NewTimer(concurrencyTestTimeout)
	defer timer.Stop()

	select {
	case err := <-ch:
		return err
	case <-timer.C:
		t.Fatalf("timed out waiting for %s", description)
		return nil
	}
}

func assertValidationErrorHasIssue(t *testing.T, err error, wantPath string, wantCode config.ValidationIssueCode) {
	t.Helper()

	var validationErr *config.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected *config.ValidationError, got %T (%v)", err, err)
	}

	for _, issue := range validationErr.Report.Errors {
		if issue.Path == wantPath && issue.Code == wantCode {
			return
		}
	}

	t.Fatalf("expected validation issue path=%q code=%q, got %#v", wantPath, wantCode, validationErr.Report.Errors)
}
