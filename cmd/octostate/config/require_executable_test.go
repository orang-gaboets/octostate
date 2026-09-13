package config

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	internalauth "github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	gitopsapply "github.com/orang-gaboets/octostate/pkg/gitops/apply"
	"github.com/orang-gaboets/octostate/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

// stubApplyPipeline replaces the collect/plan seams so the command can be
// exercised without GitHub, and records the options each apply path receives.
func stubApplyPipeline(t *testing.T, report *gitopsplan.Report) *gitopsapply.Options {
	t.Helper()

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	actual.Normalize()

	prevLoad, prevValidate := loadApplyConfig, validateApplyConfig
	prevClient, prevCollect := newApplyClient, collectApplyState
	prevPlan, prevCheck, prevExecute := buildApplyPlan, checkApply, executeApply
	t.Cleanup(func() {
		loadApplyConfig, validateApplyConfig = prevLoad, prevValidate
		newApplyClient, collectApplyState = prevClient, prevCollect
		buildApplyPlan, checkApply, executeApply = prevPlan, prevCheck, prevExecute
	})

	loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) { return cfg, nil }
	validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return actual, nil
	}
	buildApplyPlan = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) { return report, nil }

	var seen gitopsapply.Options
	checkApply = func(_ context.Context, opt gitopsapply.Options) (*gitopsapply.CheckResult, error) {
		seen = opt
		if err := failIfRequired(opt); err != nil {
			return nil, err
		}
		result := &gitopsapply.CheckResult{Organization: opt.Desired.Organization, PlanSummary: report.Summary}
		result.Normalize()
		return result, nil
	}
	executeApply = func(_ context.Context, opt gitopsapply.Options) (*gitopsapply.Result, error) {
		seen = opt
		if err := failIfRequired(opt); err != nil {
			return nil, err
		}
		result := &gitopsapply.Result{Organization: opt.Desired.Organization, PlanSummary: report.Summary}
		result.Normalize()
		return result, nil
	}
	return &seen
}

// failIfRequired mirrors the library contract so the command test asserts the
// caller-visible outcome rather than reimplementing the policy.
func failIfRequired(opt gitopsapply.Options) error {
	if !opt.RequireExecutableDesiredActions {
		return nil
	}
	if len(gitopsapply.UnfulfilledDesiredActions(opt.Plan.Actions)) == 0 {
		return nil
	}
	return gitopsapply.ErrUnfulfilledDesiredAction
}

func blockedPlan(t *testing.T) *gitopsplan.Report {
	t.Helper()

	report := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   "orang-gaboets/blocked",
			Executable:   false,
			Message:      "repository orang-gaboets/blocked cannot be created",
		}},
	}
	report.Normalize()
	return report
}

func runApplyCmd(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	cmd := ApplyConfigCmd()
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SilenceUsage = true
	cmd.SetArgs(append([]string{"--config-dir", "./config", "--token", "secret-token"}, args...))
	err := cmd.Execute()
	return out.String(), errBuf.String(), err
}

func TestApplyForwardsRequireExecutableToCheckAndLive(t *testing.T) {
	for _, mode := range []string{"--check", ""} {
		name := "live"
		if mode != "" {
			name = "check"
		}
		t.Run(name, func(t *testing.T) {
			report := &gitopsplan.Report{Organization: "orang-gaboets"}
			report.Normalize()
			seen := stubApplyPipeline(t, report)

			args := []string{"--require-executable"}
			if mode != "" {
				args = append(args, mode)
			}
			if _, _, err := runApplyCmd(t, args...); err != nil {
				t.Fatal(err)
			}
			if !seen.RequireExecutableDesiredActions {
				t.Fatal("the flag must reach the apply options")
			}
		})
	}
}

func TestApplyFailsWithoutSuccessOutputOnUnfulfilledDesiredAction(t *testing.T) {
	for _, mode := range []string{"--check", ""} {
		name := "live"
		if mode != "" {
			name = "check"
		}
		t.Run(name, func(t *testing.T) {
			stubApplyPipeline(t, blockedPlan(t))

			args := []string{"--require-executable"}
			if mode != "" {
				args = append(args, mode)
			}
			out, _, err := runApplyCmd(t, args...)
			if err == nil {
				t.Fatal("an unfulfilled desired action must produce a non-zero result")
			}
			if !errors.Is(err, gitopsapply.ErrUnfulfilledDesiredAction) {
				t.Fatalf("unexpected error: %v", err)
			}
			if strings.Contains(out, `"status": "success"`) {
				t.Fatalf("no success envelope may be printed on failure: %s", out)
			}
		})
	}
}

func TestApplySucceedsOnUnfulfilledDesiredActionWithoutTheFlag(t *testing.T) {
	stubApplyPipeline(t, blockedPlan(t))

	if _, _, err := runApplyCmd(t, "--check"); err != nil {
		t.Fatalf("default behavior must be unchanged: %v", err)
	}
}

func TestApplyRejectsRequireExecutableWithDryRun(t *testing.T) {
	stubApplyPipeline(t, blockedPlan(t))

	_, _, err := runApplyCmd(t, "--dry-run", "--require-executable")
	if err == nil {
		t.Fatal("--require-executable with --dry-run must be rejected rather than silently ignored")
	}
	if !strings.Contains(err.Error(), "--require-executable with --dry-run") {
		t.Fatalf("unexpected error: %v", err)
	}
}
