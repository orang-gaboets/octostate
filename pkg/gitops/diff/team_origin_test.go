package diff

import (
	"errors"
	"testing"
	"time"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/snapshot"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBuildDistinguishesUnknownAndInheritedTeamMemberships(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "acme",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Teams: []config.TeamSpec{{
			Slug:    "platform",
			Name:    "Platform",
			Privacy: "closed",
			Members: []config.TeamMemberSpec{{Username: "alice", Role: "member"}},
		}},
	}

	for _, test := range []struct {
		name              string
		origin            githubpkg.TeamMembershipOrigin
		wantActions       int
		wantUnresolved    int
		wantHasChanges    bool
		wantHasUnresolved bool
		wantOperation     ActionOperation
	}{
		{name: "unknown is unresolved and non-actionable", origin: githubpkg.TeamMembershipOriginUnknown, wantUnresolved: 1, wantHasUnresolved: true},
		{name: "inherited preserves direct create semantics", origin: githubpkg.TeamMembershipOriginInherited, wantActions: 1, wantHasChanges: true, wantOperation: ActionOperationCreate},
	} {
		t.Run(test.name, func(t *testing.T) {
			snap := snapshot.NewActualSnapshot(time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC), &state.OrganizationState{
				Organization: "acme",
				Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
				Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
				TeamMembers: []state.TeamMember{{
					TeamSlug: " PLATFORM ", Username: " Alice ", Role: "member", Origin: test.origin,
				}},
			})
			report, err := Build(Options{Desired: desired, Snapshot: &snap})
			if err != nil {
				t.Fatalf("Build returned error: %v", err)
			}
			if len(report.Actions) != test.wantActions {
				t.Fatalf("actions = %#v, want %d actions", report.Actions, test.wantActions)
			}
			if report.Summary.HasChanges != test.wantHasChanges {
				t.Fatalf("has_changes = %t, want %t", report.Summary.HasChanges, test.wantHasChanges)
			}
			if len(report.UnresolvedTeamMemberships) != test.wantUnresolved || report.Summary.UnresolvedMemberships != test.wantUnresolved {
				t.Fatalf("unresolved memberships = %#v, summary %#v; want %d", report.UnresolvedTeamMemberships, report.Summary, test.wantUnresolved)
			}
			if report.Summary.HasUnresolved != test.wantHasUnresolved {
				t.Fatalf("has_unresolved = %t, want %t", report.Summary.HasUnresolved, test.wantHasUnresolved)
			}
			if test.wantActions > 0 && report.Actions[0].Operation != test.wantOperation {
				t.Fatalf("action = %#v, want %s", report.Actions[0], test.wantOperation)
			}
		})
	}
}

func TestBuildDoesNotPlanRemovalForInheritedOnlyMembership(t *testing.T) {
	desired := config.OrganizationConfig{
		Organization: "acme",
		Teams:        []config.TeamSpec{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
	}
	snap := snapshot.NewActualSnapshot(time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "acme",
		Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{{
			TeamSlug: "platform", Username: "alice", Role: "member", Origin: githubpkg.TeamMembershipOriginInherited,
		}},
	})
	report, err := Build(Options{Desired: desired, Snapshot: &snap})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if len(report.Actions) != 0 {
		t.Fatalf("actions = %#v, want no action for inherited-only membership", report.Actions)
	}
}

func TestBuildRejectsInvalidSnapshotTeamMemberRole(t *testing.T) {
	snap := snapshot.NewActualSnapshot(time.Date(2026, 11, 25, 0, 0, 0, 0, time.UTC), &state.OrganizationState{
		Organization: "acme",
		TeamMembers: []state.TeamMember{{
			TeamSlug: "platform", Username: "alice", Role: "owner", Origin: githubpkg.TeamMembershipOriginDirect,
		}},
	})
	_, err := Build(Options{Desired: config.OrganizationConfig{Organization: "acme"}, Snapshot: &snap})
	if !errors.Is(err, githubpkg.ErrValidationFailed) {
		t.Fatalf("Build error = %v, want %v", err, githubpkg.ErrValidationFailed)
	}
}
