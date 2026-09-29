package config

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
	"github.com/orang-gaboets/octostate/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestPlanExpectedOrganizationRejectsBeforeAuthentication(t *testing.T) {
	for _, expected := range []string{"org-a", "org?foo", "", "  "} {
		t.Run("expected="+expected, func(t *testing.T) {
			restorePlanHooks(t)
			loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
				return gitopsconfig.OrganizationConfig{Organization: "org-b"}, nil
			}
			newPlanClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
				t.Fatal("target rejection must precede authentication")
				return nil, nil
			}
			collectPlanState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
				t.Fatal("target rejection must precede live collection")
				return nil, nil
			}

			cmd := PlanConfigCmd()
			assertTargetCommandFailure(t, cmd, []string{"--config-dir", "./config", "--expected-org", expected})
		})
	}
}

func TestPlanExpectedOrganizationMatchesWithPATAndApp(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		pat  bool
	}{
		{name: "PAT", args: []string{"--token", "token"}, pat: true},
		{name: "App", args: []string{"--app-id", "7", "--installation-id", "9", "--app-key-path", "key.pem"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			restorePlanHooks(t)
			loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
				return gitopsconfig.OrganizationConfig{Organization: "org-b"}, nil
			}
			newPlanClient = func(_ context.Context, token string, appID, installationID int64, appKeyPath string) (auth.Client, error) {
				if tc.pat && (token != "token" || appID != 0 || installationID != 0 || appKeyPath != "") {
					t.Fatalf("unexpected PAT authentication inputs")
				}
				if !tc.pat && (token != "" || appID != 7 || installationID != 9 || appKeyPath != "key.pem") {
					t.Fatalf("unexpected App authentication inputs")
				}
				return auth.MockClient{}, nil
			}
			collectPlanState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
				return &state.OrganizationState{Organization: "org-b"}, nil
			}

			cmd := PlanConfigCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(append([]string{"--config-dir", "./config", "--expected-org", " ORG-B "}, tc.args...))
			if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), `"organization": "org-b"`) {
				t.Fatalf("expected bound plan success, got output %q and error %v", out.String(), err)
			}
		})
	}
}

func TestApplyExpectedOrganizationRejectsBeforeAuthentication(t *testing.T) {
	for _, mode := range []struct {
		name string
		flag string
	}{
		{name: "live"}, {name: "check", flag: "--check"}, {name: "dry-run", flag: "--dry-run"},
	} {
		for _, expected := range []string{"org-a", "org?foo", "", "  "} {
			t.Run(mode.name+"/expected="+expected, func(t *testing.T) {
				restoreApplyHooks(t)
				loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{Organization: "org-b"}, nil
				}
				newApplyClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
					t.Fatal("target rejection must precede authentication")
					return nil, nil
				}
				collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
					t.Fatal("target rejection must precede live collection")
					return nil, nil
				}

				args := []string{"--config-dir", "./config", "--expected-org", expected}
				if mode.flag != "" {
					args = append(args, mode.flag)
				}
				assertTargetCommandFailure(t, ApplyConfigCmd(), args)
			})
		}
	}
}

func TestApplyExpectedOrganizationMatchesAcrossModes(t *testing.T) {
	for _, mode := range []struct {
		name string
		flag string
	}{
		{name: "live"}, {name: "check", flag: "--check"}, {name: "dry-run", flag: "--dry-run"},
	} {
		t.Run(mode.name, func(t *testing.T) {
			restoreApplyHooks(t)
			loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
				return gitopsconfig.OrganizationConfig{Organization: "org-b"}, nil
			}
			newApplyClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
				return auth.MockClient{
					OrganizationsService: auth.MockOrganizationService{},
					ReposService:         auth.MockRepoService{},
					TeamsService:         auth.MockTeamsService{},
					UsersService:         auth.MockUserService{},
				}, nil
			}
			collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
				return &state.OrganizationState{Organization: "org-b"}, nil
			}

			args := []string{"--config-dir", "./config", "--expected-org", " ORG-B "}
			if mode.flag != "" {
				args = append(args, mode.flag)
			}
			cmd := ApplyConfigCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), `"organization": "org-b"`) {
				t.Fatalf("expected bound apply success, got output %q and error %v", out.String(), err)
			}
		})
	}
}

func assertTargetCommandFailure(t *testing.T, cmd *cobra.Command, args []string) {
	t.Helper()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "organization") {
		t.Fatalf("expected target error, got %v", err)
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected invalid-input exit code, got code %d (present %t), error %v", code, ok, err)
	}
	if out.Len() != 0 || !strings.Contains(stderr.String(), "organization") {
		t.Fatalf("unexpected command output: stdout %q stderr %q", out.String(), stderr.String())
	}
}
