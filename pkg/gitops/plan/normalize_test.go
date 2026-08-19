package plan

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// paddedDesired declares the same organization as liveState below, but every
// scalar carries surrounding whitespace as a programmatic caller might produce.
func paddedDesired() config.OrganizationConfig {
	return config.OrganizationConfig{
		Organization: " orang-gaboets ",
		Members: []config.OrganizationMemberSpec{
			{Username: " alice ", Role: " admin "},
		},
		Invites: []config.InviteSpec{
			{Username: config.OptionalString{Present: true, Value: " octocat "}, Role: " direct_member ", TeamSlugs: []string{" platform "}},
			{Email: config.OptionalString{Present: true, Value: " dev@example.com "}, Role: " direct_member "},
		},
		Repositories: []config.RepositorySpec{{
			Name:       " service ",
			Visibility: " private ",
			Topics:     []string{" go ", " gitops "},
		}},
		Teams: []config.TeamSpec{{
			Slug:        " platform ",
			Name:        " Platform ",
			Description: " Platform team ",
			Privacy:     " closed ",
			Members: []config.TeamMemberSpec{
				{Username: " alice ", Role: " maintainer "},
			},
			Repositories: []config.TeamRepositorySpec{
				{Name: " service ", Permission: " push "},
			},
		}},
	}
}

func liveState() *state.OrganizationState {
	return &state.OrganizationState{
		Organization: "orang-gaboets",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "admin"}},
		PendingInvitations: []state.PendingInvitation{
			{ID: 1, Username: "octocat", Role: "direct_member", TeamSlugs: []string{"platform"}},
			{ID: 2, Email: "dev@example.com", Role: "direct_member", TeamSlugs: []string{}},
		},
		Repositories: []state.Repository{{
			Owner: "orang-gaboets", Name: "service", Visibility: "private",
			Topics: []string{"go", "gitops"},
		}},
		Teams: []state.Team{{
			Slug: "platform", Name: "Platform", Description: "Platform team", Privacy: "closed",
		}},
		TeamMembers: []state.TeamMember{{TeamSlug: "platform", Username: "alice", Role: "maintainer"}},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{{
			TeamSlug: "platform", Owner: "orang-gaboets", Name: "service", Permission: "push",
		}},
	}
}

func TestBuildTreatsWhitespacePaddedDesiredStateAsNoDrift(t *testing.T) {
	t.Parallel()

	report, err := Build(context.Background(), Options{Desired: paddedDesired(), Actual: liveState()})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if report.Summary.HasChanges {
		t.Fatalf("whitespace-only differences must not be drift, got %#v", report.Actions)
	}
}

func TestBuildDoesNotMutateDesiredConfig(t *testing.T) {
	t.Parallel()

	desired := paddedDesired()
	snapshot := paddedDesired()

	if _, err := Build(context.Background(), Options{Desired: desired, Actual: liveState()}); err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !reflect.DeepEqual(desired, snapshot) {
		t.Fatalf("Build mutated the caller's config:\n got %#v\nwant %#v", desired, snapshot)
	}
}

func TestBuildProducesSameReportForLoadedAndConstructedConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "organization.yaml")
	yaml := []byte(`organization: " orang-gaboets "
members:
  - username: " alice "
    role: " admin "
invites:
  - username: " octocat "
    role: " direct_member "
    team_slugs: [" platform "]
  - email: " dev@example.com "
    role: " direct_member "
repositories:
  - name: " service "
    visibility: " private "
    topics: [" go ", " gitops "]
teams:
  - slug: " platform "
    name: " Platform "
    description: " Platform team "
    privacy: " closed "
    members:
      - username: " alice "
        role: " maintainer "
    repositories:
      - name: " service "
        permission: " push "
`)
	if err := os.WriteFile(path, yaml, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile returned error: %v", err)
	}

	fromFile, err := Build(context.Background(), Options{Desired: loaded, Actual: liveState()})
	if err != nil {
		t.Fatalf("Build from loaded config returned error: %v", err)
	}
	fromStruct, err := Build(context.Background(), Options{Desired: paddedDesired(), Actual: liveState()})
	if err != nil {
		t.Fatalf("Build from constructed config returned error: %v", err)
	}
	if !reflect.DeepEqual(fromFile, fromStruct) {
		t.Fatalf("loaded and constructed configs must reconcile identically:\n file   %#v\n struct %#v", fromFile, fromStruct)
	}
}
