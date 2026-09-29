package audit

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopssnapshot "github.com/orang-gaboets/octostate/pkg/gitops/snapshot"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestAuditPullExpectedOrganizationRejectsBeforeLiveWork(t *testing.T) {
	for _, expected := range []string{"org-a", "org?foo", "", "  "} {
		t.Run("expected="+expected, func(t *testing.T) {
			restore := replaceAuditHooks(t)
			defer restore()
			loadAuditConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
				return gitopsconfig.OrganizationConfig{Organization: "org-b"}, nil
			}
			newAuditClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
				t.Fatal("target rejection must precede authentication")
				return nil, nil
			}
			collectAuditState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
				t.Fatal("target rejection must precede collection")
				return nil, nil
			}
			writeActualSnapshot = func(string, gitopssnapshot.ActualSnapshot) (string, error) {
				t.Fatal("target rejection must precede snapshot write")
				return "", nil
			}

			cmd := PullCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state", "--expected-org", expected})
			err := cmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "organization") || out.Len() != 0 {
				t.Fatalf("expected target rejection without output, got error %v and output %q", err, out.String())
			}
		})
	}
}

func TestAuditPullExpectedOrganizationMatches(t *testing.T) {
	restore := replaceAuditHooks(t)
	defer restore()
	loadAuditConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "org-b"}, nil
	}
	newAuditClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
		return auth.MockClient{}, nil
	}
	collectAuditState = func(_ context.Context, opt collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		if opt.OrgName != "org-b" {
			t.Fatalf("unexpected collection target %q", opt.OrgName)
		}
		return &state.OrganizationState{Organization: "org-b"}, nil
	}
	writeActualSnapshot = func(_ string, snapshot gitopssnapshot.ActualSnapshot) (string, error) {
		if snapshot.Organization != "org-b" {
			t.Fatalf("unexpected snapshot organization %q", snapshot.Organization)
		}
		return "state/actual/snapshot.json", nil
	}

	cmd := PullCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state", "--expected-org", " ORG-B "})
	if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), `"organization": "org-b"`) {
		t.Fatalf("expected bound audit pull success, got output %q and error %v", out.String(), err)
	}
}
