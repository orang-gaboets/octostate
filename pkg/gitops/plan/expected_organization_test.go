package plan

import (
	"context"
	"strings"
	"testing"

	"github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestBuildForOrganizationRequiresMatchingIndependentTarget(t *testing.T) {
	t.Parallel()
	opt := Options{
		Desired: config.OrganizationConfig{Organization: "org-b"},
		Actual:  &state.OrganizationState{Organization: "org-b"},
	}

	for _, expected := range []string{"org-a", ""} {
		report, err := BuildForOrganization(context.Background(), opt, expected)
		if err == nil || report != nil {
			t.Fatalf("expected target rejection for %q, got report %#v and error %v", expected, report, err)
		}
	}

	report, err := BuildForOrganization(context.Background(), opt, " ORG-B ")
	if err != nil || report == nil || report.Organization != "org-b" {
		t.Fatalf("expected matching target to plan, got report %#v and error %v", report, err)
	}

	opt.Desired.Repositories = []config.RepositorySpec{{
		Name: "service", Visibility: "private",
		Template: config.TemplateSpec{Owner: "external-org", Name: "template"},
	}}
	report, err = BuildForOrganization(context.Background(), opt, "org-b")
	if err != nil || report == nil || len(report.Actions) == 0 {
		t.Fatalf("expected external template reference to remain valid, got report %#v and error %v", report, err)
	}
	if strings.Contains(strings.ToLower(report.Actions[0].Message), "organization mismatch") {
		t.Fatalf("external template reference was treated as a target mismatch: %#v", report.Actions[0])
	}
}
