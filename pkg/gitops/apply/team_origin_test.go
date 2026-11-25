package apply

import (
	"context"
	"errors"
	"testing"

	gh "github.com/google/go-github/v88/github"
	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestCheckAndExecuteRequireExecutableForUnresolvedDesiredMembership(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "org-a",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Repositories: []config.RepositorySpec{{Owner: "org-a", Name: "application", Visibility: "private"}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Members: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
		}},
	}
	actual := &state.OrganizationState{
		Organization: "org-a",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
		Teams:        []state.Team{{ID: 1, Slug: "PLATFORM", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{{
			TeamSlug: " PLATFORM ", Username: " Alice ", Role: "member", Origin: githubpkg.TeamMembershipOriginUnknown,
		}},
	}

	for _, test := range []struct {
		name string
		run  func(Options) error
	}{
		{name: "check", run: func(options Options) error { _, err := Check(context.Background(), options); return err }},
		{name: "execute", run: func(options Options) error { _, err := Execute(context.Background(), options); return err }},
	} {
		t.Run(test.name, func(t *testing.T) {
			writes := 0
			options := testApplyOptions(desired, actual, planWith(gitopsplan.Action{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   repositoryResourceID("org-a", "application"),
				Executable:   true,
			}), withRepoService(&testRepoService{
				createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
					writes++
					return &gh.Repository{}, nil, nil
				},
			}))
			options.RequireExecutableDesiredActions = true
			if err := test.run(options); !errors.Is(err, ErrUnfulfilledDesiredAction) {
				t.Fatalf("error = %v, want ErrUnfulfilledDesiredAction for actionless unresolved membership", err)
			}
			if writes != 0 {
				t.Fatalf("repository writes = %d, want 0 after unresolved desired membership", writes)
			}
		})
	}
}

func TestReviewedCheckAndExecuteRequireExecutableForUnresolvedDesiredMembership(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "org-a",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Repositories: []config.RepositorySpec{{Owner: "org-a", Name: "application", Visibility: "private"}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Members: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
		}},
	}
	identity, err := config.DesiredStateIdentity(desired)
	if err != nil {
		t.Fatalf("compute desired identity: %v", err)
	}
	actual := &state.OrganizationState{
		Organization: "org-a",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
		Teams:        []state.Team{{ID: 1, Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{{
			TeamSlug: "platform", Username: "alice", Role: "member", Origin: githubpkg.TeamMembershipOriginUnknown,
		}},
	}
	writes := 0
	options := reviewedTestOptions(desired, actual, identity)
	options.RepositoryService = &testRepoService{
		createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
			writes++
			return &gh.Repository{}, nil, nil
		},
	}
	options.RequireExecutableDesiredActions = true

	if _, err := CheckReviewed(context.Background(), options); !errors.Is(err, ErrUnfulfilledDesiredAction) {
		t.Fatalf("CheckReviewed error = %v, want ErrUnfulfilledDesiredAction", err)
	}
	if _, err := ExecuteReviewed(context.Background(), options); !errors.Is(err, ErrUnfulfilledDesiredAction) {
		t.Fatalf("ExecuteReviewed error = %v, want ErrUnfulfilledDesiredAction", err)
	}
	if writes != 0 {
		t.Fatalf("reviewed repository writes = %d, want 0 after unresolved desired membership", writes)
	}
}

func TestCallerSuppliedTeamMembershipActionsForUnknownOriginFailBeforeWrites(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "org-a",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Repositories: []config.RepositorySpec{{Owner: "org-a", Name: "application", Visibility: "private"}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Members: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
		}},
	}
	actual := &state.OrganizationState{
		Organization: "org-a",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
		Teams:        []state.Team{{ID: 1, Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{{
			TeamSlug: "PLATFORM", Username: "ALICE", Role: "member", Origin: githubpkg.TeamMembershipOriginUnknown,
		}},
	}
	for _, operation := range []gitopsplan.ActionOperation{
		gitopsplan.ActionOperationCreate,
		gitopsplan.ActionOperationUpdate,
		gitopsplan.ActionOperationRemove,
	} {
		t.Run(string(operation), func(t *testing.T) {
			plan := planWith(
				gitopsplan.Action{
					ResourceType: gitopsplan.ActionResourceTypeTeamMember,
					Operation:    operation,
					ResourceID:   "Platform/Alice",
					Executable:   true,
				},
				gitopsplan.Action{
					ResourceType: gitopsplan.ActionResourceTypeRepository,
					Operation:    gitopsplan.ActionOperationCreate,
					ResourceID:   repositoryResourceID("org-a", "application"),
					Executable:   true,
				},
			)
			if len(plan.Actions) < 2 || plan.Actions[0].ResourceType != gitopsplan.ActionResourceTypeRepository || plan.Actions[1].ResourceType != gitopsplan.ActionResourceTypeTeamMember {
				t.Fatalf("plan order = %#v, want repository mutation before offending membership action", plan.Actions)
			}
			teamWrites := 0
			repositoryWrites := 0
			options := testApplyOptions(desired, actual, plan, withRepoService(&testRepoService{
				createFunc: func(context.Context, string, *gh.Repository) (*gh.Repository, *gh.Response, error) {
					repositoryWrites++
					return &gh.Repository{}, nil, nil
				},
			}), withTeamService(&testTeamService{
				addTeamMembershipBySlugFunc: func(context.Context, string, string, string, *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
					teamWrites++
					return &gh.Membership{}, nil, nil
				},
				removeTeamMembershipFunc: func(context.Context, string, string, string) (*gh.Response, error) {
					teamWrites++
					return nil, nil
				},
			}))

			if _, err := Check(context.Background(), options); !errors.Is(err, ErrUnresolvedTeamMembership) {
				t.Fatalf("Check error = %v, want ErrUnresolvedTeamMembership", err)
			}
			if _, err := Execute(context.Background(), options); !errors.Is(err, ErrUnresolvedTeamMembership) {
				t.Fatalf("Execute error = %v, want ErrUnresolvedTeamMembership", err)
			}
			if repositoryWrites != 0 || teamWrites != 0 {
				t.Fatalf("writes before unresolved membership rejection = repository:%d team-membership:%d, want 0", repositoryWrites, teamWrites)
			}
		})
	}
}

func TestCheckAndExecuteOutputUnresolvedTeamMemberships(t *testing.T) {
	actual := &state.OrganizationState{
		Organization: "org-a",
		TeamMembers: []state.TeamMember{{
			TeamSlug: "platform", Username: "alice", Role: "member", Origin: githubpkg.TeamMembershipOriginUnknown,
		}},
	}
	options := testApplyOptions(config.OrganizationConfig{Organization: "org-a"}, actual, planWith())
	checked, err := Check(context.Background(), options)
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if len(checked.UnresolvedTeamMemberships) != 1 || !checked.PlanSummary.HasUnresolved || checked.PlanSummary.HasChanges {
		t.Fatalf("Check unresolved memberships = %#v, want one", checked.UnresolvedTeamMemberships)
	}
	executed, err := Execute(context.Background(), options)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if len(executed.UnresolvedTeamMemberships) != 1 || !executed.PlanSummary.HasUnresolved || executed.PlanSummary.HasChanges {
		t.Fatalf("Execute unresolved memberships = %#v, want one", executed.UnresolvedTeamMemberships)
	}
}
