package plan

import (
	"context"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// memberDependencyConfig declares alice as a top-level member and as a member
// of platform, which is the shape the schema requires.
func memberDependencyConfig() config.OrganizationConfig {
	return config.OrganizationConfig{
		Organization: "acme",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Members: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
		}},
	}
}

func actionByID(actions []Action, resourceType ActionResourceType, id string) (Action, bool) {
	for _, a := range actions {
		if a.ResourceType == resourceType && a.ResourceID == id {
			return a, true
		}
	}
	return Action{}, false
}

func indexOf(actions []Action, resourceType ActionResourceType, id string) int {
	for i, a := range actions {
		if a.ResourceType == resourceType && a.ResourceID == id {
			return i
		}
	}
	return -1
}

// The organization-member create in the same plan satisfies the prerequisite,
// so the dependent team membership must not be reported as unfulfillable.
func TestBuildTeamMembershipExecutableWhenMemberCreatedInSamePlan(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
		Desired: memberDependencyConfig(),
		Actual: &state.OrganizationState{
			Organization: "acme",
			Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	member, ok := actionByID(report.Actions, ActionResourceTypeTeamMember, "platform/alice")
	if !ok {
		t.Fatalf("no team membership action emitted: %#v", report.Actions)
	}
	if !member.Executable {
		t.Fatalf("team membership must be executable after the same-plan member create: %q", member.Message)
	}
}

// The prerequisite has to be ordered before the action that depends on it.
func TestBuildOrdersOrganizationMemberBeforeDependentTeamMembership(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
		Desired: memberDependencyConfig(),
		Actual: &state.OrganizationState{
			Organization: "acme",
			Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	prerequisite := indexOf(report.Actions, ActionResourceTypeOrganizationMember, "alice")
	dependent := indexOf(report.Actions, ActionResourceTypeTeamMember, "platform/alice")
	if prerequisite < 0 || dependent < 0 {
		t.Fatalf("expected both actions, got %#v", report.Actions)
	}
	if prerequisite > dependent {
		t.Fatalf("organization member (index %d) must precede team membership (index %d)", prerequisite, dependent)
	}
}

// An existing live member keeps the previous behavior.
func TestBuildTeamMembershipExecutableWhenMemberAlreadyLive(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{
		Desired: memberDependencyConfig(),
		Actual: &state.OrganizationState{
			Organization: "acme",
			Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
			Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	member, ok := actionByID(report.Actions, ActionResourceTypeTeamMember, "platform/alice")
	if !ok {
		t.Fatalf("no team membership action emitted: %#v", report.Actions)
	}
	if !member.Executable {
		t.Fatalf("team membership for a live member must be executable: %q", member.Message)
	}
}

// A role update on an existing membership is unaffected by the dependency rule.
func TestBuildTeamMembershipRoleUpdateUnchanged(t *testing.T) {
	t.Parallel()

	desired := memberDependencyConfig()
	desired.Teams[0].Members[0].Role = "maintainer"

	report, err := Build(context.Background(), Options{
		Desired: desired,
		Actual: &state.OrganizationState{
			Organization: "acme",
			Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
			Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
			TeamMembers:  []state.TeamMember{{TeamSlug: "platform", Username: "alice", Role: "member"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	member, ok := actionByID(report.Actions, ActionResourceTypeTeamMember, "platform/alice")
	if !ok {
		t.Fatalf("no team membership action emitted: %#v", report.Actions)
	}
	if member.Operation != ActionOperationUpdate || !member.Executable {
		t.Fatalf("expected an executable role update, got %#v", member)
	}
}
