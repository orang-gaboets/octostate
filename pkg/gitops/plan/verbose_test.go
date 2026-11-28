package plan

import (
	"bytes"
	"context"
	"strings"
	"testing"

	githubpkg "github.com/orang-gaboets/octostate/pkg/github"
	ghlogging "github.com/orang-gaboets/octostate/pkg/github/logging"
	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBuildVerboseLogsPlanCounts(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), true, &logs)
	report, err := Build(ctx, Options{Desired: paddedDesired(), Actual: liveState()})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	want := "plan built for orang-gaboets: actions=0 executable=0 unresolved_memberships=0"
	if len(report.Actions) != 0 || !strings.Contains(logs.String(), want) {
		t.Fatalf("verbose output missing %q (actions=%d):\n%s", want, len(report.Actions), logs.String())
	}
}

func TestBuildVerboseLogsDistinctPlanCounts(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "acme",
		Members:      []config.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Teams: []config.TeamSpec{
			{Slug: "platform", Name: "Platform", Privacy: "closed", Members: []config.TeamMemberSpec{{Username: "alice", Role: "member"}}},
			{Slug: "ops", Name: "Ops", Privacy: "closed"},
			{Slug: "web", Name: "Web", Privacy: "closed"},
		},
	}
	actual := &state.OrganizationState{
		Organization: "acme",
		Members:      []state.OrganizationMember{{Username: "alice", Role: "member"}},
		Teams:        []state.Team{{Slug: "platform", Name: "Platform", Privacy: "closed"}, {Slug: "legacy", Name: "Legacy", Privacy: "closed"}},
		TeamMembers:  []state.TeamMember{{TeamSlug: "platform", Username: "alice", Role: "member", Origin: githubpkg.TeamMembershipOriginUnknown}},
	}
	var logs bytes.Buffer
	ctx := ghlogging.WithVerbose(context.Background(), true, &logs)
	report, err := Build(ctx, Options{Desired: desired, Actual: actual})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if report.Summary.Actions != 3 || report.Summary.ExecutableActions != 2 || report.Summary.UnresolvedMemberships != 1 {
		t.Fatalf("fixture summary = %#v, want actions=3 executable=2 unresolved=1", report.Summary)
	}
	want := "plan built for acme: actions=3 executable=2 unresolved_memberships=1"
	if strings.Count(logs.String(), want) != 1 {
		t.Fatalf("verbose output missing %q exactly once:\n%s", want, logs.String())
	}
}
