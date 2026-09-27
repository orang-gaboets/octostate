package diff

import (
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/snapshot"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// Offline diff reports the same dependency relationship as live planning: a
// desired top-level member satisfies the prerequisite for a desired team
// membership in the same desired state.
func TestBuildTeamMembershipExecutableWhenMemberDeclaredInDesiredState(t *testing.T) {
	t.Parallel()

	report, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "acme",
			Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
			Teams: []config.TeamSpec{{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
				Members: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
			}},
		},
		Snapshot: &snapshot.ActualSnapshot{
			Organization: "acme",
			Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		},
	})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	for _, action := range report.Actions {
		if action.ResourceType == ActionResourceTypeTeamMember && action.ResourceID == "platform/alice" {
			if !action.Executable {
				t.Fatalf("team membership must be executable alongside the desired member: %q", action.Message)
			}
			return
		}
	}
	t.Fatalf("no team membership action emitted: %#v", report.Actions)
}

func TestBuildTreatsInheritedOnlyParentMembershipAsAbsent(t *testing.T) {
	t.Parallel()

	for _, includeParentMembership := range []bool{false, true} {
		name := "omitted from desired"
		parentMembers := []config.TeamMemberSpec{}
		if includeParentMembership {
			name = "desired as direct"
			parentMembers = []config.TeamMemberSpec{{Username: "bob", Role: "member"}}
		}
		t.Run(name, func(t *testing.T) {
			desired := config.OrganizationConfig{
				Organization: "acme",
				Members:      []config.OrganizationMemberSpec{{Username: "bob", Role: "member"}},
				Teams: []config.TeamSpec{
					{Slug: "parent", Name: "Parent", Privacy: "closed", Members: parentMembers},
					{Slug: "child", Name: "Child", Privacy: "closed", ParentSlug: "parent", Members: []config.TeamMemberSpec{{Username: "bob", Role: "member"}}},
				},
			}
			// The live parent response contained only inherited membership for bob,
			// so the snapshot contains only the child's direct row.
			snapshotState := &snapshot.ActualSnapshot{
				Organization: "acme",
				Members:      []state.OrganizationMember{{Username: "bob", Role: "member"}},
				Teams: []state.Team{
					{Slug: "parent", Name: "Parent", Privacy: "closed"},
					{Slug: "child", Name: "Child", Privacy: "closed", ParentSlug: "parent"},
				},
				TeamMembers: []state.TeamMember{{TeamSlug: "child", Username: "bob", Role: "member"}},
			}

			report, err := Build(Options{Desired: desired, Snapshot: snapshotState})
			if err != nil {
				t.Fatalf("Build returned error: %v", err)
			}
			var parentAction *Action
			for i := range report.Actions {
				if report.Actions[i].ResourceType == ActionResourceTypeTeamMember && report.Actions[i].ResourceID == "parent/bob" {
					parentAction = &report.Actions[i]
					break
				}
			}
			if !includeParentMembership {
				if parentAction != nil {
					t.Fatalf("inherited-only parent member produced action: %#v", *parentAction)
				}
				return
			}
			if parentAction == nil {
				t.Fatalf("desired direct parent membership produced no action: %#v", report.Actions)
			}
			if parentAction.Operation != ActionOperationCreate || !parentAction.Executable {
				t.Fatalf("direct parent membership action = %#v, want executable create", *parentAction)
			}
		})
	}
}

// A team member neither live nor declared never reaches planning: validation
// rejects it first. That is why the unavailable-prerequisite branch in
// planning is defensive rather than a reachable state through Build.
func TestBuildRejectsTeamMemberMissingFromTopLevelMembers(t *testing.T) {
	t.Parallel()

	_, err := Build(Options{
		Desired: config.OrganizationConfig{
			Organization: "acme",
			Teams: []config.TeamSpec{{
				Slug:    "platform",
				Name:    "Platform",
				Privacy: "closed",
				Members: []config.TeamMemberSpec{{Username: "ghost", Role: "member"}},
			}},
		},
		Snapshot: &snapshot.ActualSnapshot{
			Organization: "acme",
			Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		},
	})
	if err == nil {
		t.Fatal("an undeclared team member must fail validation before planning")
	}
}
