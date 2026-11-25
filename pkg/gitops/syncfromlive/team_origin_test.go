package syncfromlive

import (
	"errors"
	"testing"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBootstrapAndAdoptFailClosedForUnknownMembershipOrigin(t *testing.T) {
	actual := &state.OrganizationState{
		Organization: "acme",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
		Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{{
			TeamSlug: "platform", Username: "alice", Role: "member", Origin: githubpkg.TeamMembershipOriginUnknown,
		}},
	}
	if _, err := BuildBootstrapConfig(BootstrapOptions{Actual: actual}); !errors.Is(err, githubpkg.ErrValidationFailed) {
		t.Fatalf("BuildBootstrapConfig error = %v, want validation failure for unknown origin", err)
	}
	if _, err := BuildAdoptConfig(AdoptOptions{Desired: config.OrganizationConfig{Organization: "acme"}, Actual: actual}); !errors.Is(err, githubpkg.ErrValidationFailed) {
		t.Fatalf("BuildAdoptConfig error = %v, want validation failure for unknown origin", err)
	}
}

func TestBootstrapAndAdoptRejectInvalidTeamMemberRole(t *testing.T) {
	actual := &state.OrganizationState{
		Organization: "acme",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
		Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{{
			TeamSlug: "platform", Username: "alice", Role: "owner", Origin: githubpkg.TeamMembershipOriginDirect,
		}},
	}
	if _, err := BuildBootstrapConfig(BootstrapOptions{Actual: actual}); !errors.Is(err, githubpkg.ErrValidationFailed) {
		t.Fatalf("BuildBootstrapConfig error = %v, want %v", err, githubpkg.ErrValidationFailed)
	}
	if _, err := BuildAdoptConfig(AdoptOptions{
		Desired: config.OrganizationConfig{Organization: "acme"},
		Actual:  actual,
	}); !errors.Is(err, githubpkg.ErrValidationFailed) {
		t.Fatalf("BuildAdoptConfig error = %v, want %v", err, githubpkg.ErrValidationFailed)
	}
}

func TestBootstrapDoesNotMaterializeInheritedMembershipAsDirect(t *testing.T) {
	configValue, err := BuildBootstrapConfig(BootstrapOptions{Actual: &state.OrganizationState{
		Organization: "acme",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
		Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{{
			TeamSlug: "platform", Username: "alice", Role: "member", Origin: githubpkg.TeamMembershipOriginInherited,
		}},
	}})
	if err != nil {
		t.Fatalf("BuildBootstrapConfig returned error: %v", err)
	}
	if len(configValue.Teams) != 1 || len(configValue.Teams[0].Members) != 0 {
		t.Fatalf("bootstrap team memberships = %#v, want no materialized direct memberships", configValue.Teams)
	}
}
