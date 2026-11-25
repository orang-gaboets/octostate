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
			options := testApplyOptions(desired, actual, planWith())
			options.RequireExecutableDesiredActions = true
			if err := test.run(options); !errors.Is(err, ErrUnfulfilledDesiredAction) {
				t.Fatalf("error = %v, want ErrUnfulfilledDesiredAction for actionless unresolved membership", err)
			}
		})
	}
}

func TestReviewedCheckAndExecuteRequireExecutableForUnresolvedDesiredMembership(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "org-a",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
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
	options := reviewedTestOptions(desired, actual, identity)
	options.RequireExecutableDesiredActions = true

	if _, err := CheckReviewed(context.Background(), options); !errors.Is(err, ErrUnfulfilledDesiredAction) {
		t.Fatalf("CheckReviewed error = %v, want ErrUnfulfilledDesiredAction", err)
	}
	if _, err := ExecuteReviewed(context.Background(), options); !errors.Is(err, ErrUnfulfilledDesiredAction) {
		t.Fatalf("ExecuteReviewed error = %v, want ErrUnfulfilledDesiredAction", err)
	}
}

func TestCallerSuppliedTeamMembershipActionForUnknownOriginFailsBeforeWrites(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "org-a",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
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
	plan := planWith(gitopsplan.Action{
		ResourceType: gitopsplan.ActionResourceTypeTeamMember,
		Operation:    gitopsplan.ActionOperationCreate,
		ResourceID:   "Platform/Alice",
		Executable:   true,
	})
	writes := 0
	options := testApplyOptions(desired, actual, plan, withTeamService(&testTeamService{
		addTeamMembershipBySlugFunc: func(context.Context, string, string, string, *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
			writes++
			return &gh.Membership{}, nil, nil
		},
	}))

	if _, err := Check(context.Background(), options); !errors.Is(err, ErrUnresolvedTeamMembership) {
		t.Fatalf("Check error = %v, want ErrUnresolvedTeamMembership", err)
	}
	if _, err := Execute(context.Background(), options); !errors.Is(err, ErrUnresolvedTeamMembership) {
		t.Fatalf("Execute error = %v, want ErrUnresolvedTeamMembership", err)
	}
	if writes != 0 {
		t.Fatalf("team membership writes = %d, want 0", writes)
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
