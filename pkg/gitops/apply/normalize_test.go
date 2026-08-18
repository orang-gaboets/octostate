package apply

import (
	"context"
	"reflect"
	"testing"

	gh "github.com/google/go-github/v88/github"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// paddedDesired carries surrounding whitespace on every scalar, as a
// programmatic caller might produce.
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
			Slug:         " platform ",
			Name:         " Platform ",
			Privacy:      " closed ",
			Members:      []config.TeamMemberSpec{{Username: " alice ", Role: " maintainer "}},
			Repositories: []config.TeamRepositorySpec{{Name: " service ", Permission: " push "}},
		}},
	}
}

func emptyPlan() *gitopsplan.Report {
	plan := &gitopsplan.Report{Organization: "orang-gaboets"}
	plan.Normalize()
	return plan
}

func TestCheckDoesNotMutateDesiredConfig(t *testing.T) {
	t.Parallel()

	desired := paddedDesired()
	want := paddedDesired()

	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	if _, err := Check(context.Background(), testApplyOptions(desired, actual, emptyPlan())); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !reflect.DeepEqual(desired, want) {
		t.Fatalf("Check mutated the caller's config:\n got %#v\nwant %#v", desired, want)
	}
}

func TestExecuteDoesNotMutateDesiredConfig(t *testing.T) {
	t.Parallel()

	desired := paddedDesired()
	want := paddedDesired()

	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	if _, err := Execute(context.Background(), testApplyOptions(desired, actual, emptyPlan())); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !reflect.DeepEqual(desired, want) {
		t.Fatalf("Execute mutated the caller's config:\n got %#v\nwant %#v", desired, want)
	}
}

func TestExecuteReportsNormalizedOrganization(t *testing.T) {
	t.Parallel()

	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	result, err := Execute(context.Background(), testApplyOptions(paddedDesired(), actual, emptyPlan()))
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result.Organization != "orang-gaboets" {
		t.Fatalf("result organization = %q, want %q", result.Organization, "orang-gaboets")
	}
}

// Non-mutation alone would not catch a padded value being sent to GitHub, so
// this asserts on what the executor actually transmits.
func TestExecuteSendsNormalizedValuesToGitHub(t *testing.T) {
	t.Parallel()

	desired := config.OrganizationConfig{
		Organization: "org-a",
		Teams: []config.TeamSpec{{
			Slug:    " platform ",
			Name:    " Platform ",
			Privacy: " closed ",
			Repositories: []config.TeamRepositorySpec{{
				Name:       " service ",
				Permission: " push ",
			}},
		}},
	}
	plan := &gitopsplan.Report{
		Organization: "org-a",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeTeamRepositoryPermission,
			Operation:    gitopsplan.ActionOperationUpdate,
			ResourceID:   teamRepoPermissionResourceID("platform", "org-a", "service"),
			Executable:   true,
			Changes:      []gitopsplan.FieldChange{{Field: "permission", From: "pull", To: "push"}},
		}},
	}
	plan.Normalize()

	actual := &state.OrganizationState{
		Organization: "org-a",
		Repositories: []state.Repository{{Owner: "org-a", Name: "service", Visibility: "private"}},
		Teams:        []state.Team{{ID: 1, Slug: "platform", Name: "Platform", Privacy: "closed"}},
		TeamRepositoryPermissions: []state.TeamRepositoryPermission{{
			TeamSlug: "platform", Owner: "org-a", Name: "service", Permission: "pull",
		}},
	}

	var gotSlug, gotOwner, gotRepo, gotPermission string
	teamSvc := &testTeamService{
		addTeamRepoBySlugFunc: func(_ context.Context, _, slug, owner, repo string, opts *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
			gotSlug, gotOwner, gotRepo = slug, owner, repo
			if opts != nil {
				gotPermission = opts.Permission
			}
			return nil, nil
		},
	}

	opts := testApplyOptions(desired, actual, plan)
	opts.TeamService = teamSvc
	if _, err := Execute(context.Background(), opts); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if gotSlug != "platform" || gotOwner != "org-a" || gotRepo != "service" || gotPermission != "push" {
		t.Fatalf("executor sent un-normalized values: slug=%q owner=%q repo=%q permission=%q",
			gotSlug, gotOwner, gotRepo, gotPermission)
	}
}
