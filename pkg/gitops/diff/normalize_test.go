package diff

import (
	"reflect"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/snapshot"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// paddedDesired declares the same organization as paddedSnapshot below, but
// every scalar carries surrounding whitespace as a programmatic caller might
// produce.
func paddedDesired() config.OrganizationConfig {
	return config.OrganizationConfig{
		Organization: " orang-gaboets ",
		Members: []config.OrganizationMemberSpec{
			{Username: " alice ", Role: " admin "},
		},
		Repositories: []config.RepositorySpec{{
			Name:       " service ",
			Visibility: " private ",
			Topics:     []string{" go "},
		}},
		Teams: []config.TeamSpec{{
			Slug:    " platform ",
			Name:    " Platform ",
			Privacy: " closed ",
			Members: []config.TeamMemberSpec{{Username: " alice ", Role: " maintainer "}},
			Repositories: []config.TeamRepositorySpec{
				{Name: " service ", Permission: " push "},
			},
		}},
	}
}

func paddedSnapshot() *snapshot.ActualSnapshot {
	return &snapshot.ActualSnapshot{
		Organization: "orang-gaboets",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "admin"}},
		Repositories: []state.Repository{{
			Owner: "orang-gaboets", Name: "service", Visibility: "private", Topics: []string{"go"},
		}},
		Teams:       []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamMembers: []state.TeamMember{{TeamSlug: "platform", Username: "alice", Role: "maintainer"}},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{{
			TeamSlug: "platform", Owner: "orang-gaboets", Name: "service", Permission: "push",
		}},
	}
}

func TestBuildTreatsWhitespacePaddedDesiredStateAsNoDrift(t *testing.T) {
	t.Parallel()

	report, err := Build(Options{Desired: paddedDesired(), Snapshot: paddedSnapshot()})
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
	want := paddedDesired()

	if _, err := Build(Options{Desired: desired, Snapshot: paddedSnapshot()}); err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !reflect.DeepEqual(desired, want) {
		t.Fatalf("Build mutated the caller's config:\n got %#v\nwant %#v", desired, want)
	}
}
