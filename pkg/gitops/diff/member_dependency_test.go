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
