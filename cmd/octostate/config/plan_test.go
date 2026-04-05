package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	internalauth "github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
	"github.com/orang-gaboets/octostate/pkg/github"
	"github.com/orang-gaboets/octostate/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestPlanConfigCmdSuccess(t *testing.T) {
	restorePlanHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	want := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   "orang-gaboets/octostate",
			Executable:   true,
			Message:      "create repository orang-gaboets/octostate",
		}},
	}
	want.Normalize()

	loadPlanConfig = func(path string) (gitopsconfig.OrganizationConfig, error) {
		if path != "./config" {
			t.Fatalf("unexpected config path %q", path)
		}
		return cfg, nil
	}
	validatePlanConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		if !reflect.DeepEqual(got, cfg) {
			t.Fatalf("unexpected config: got %#v want %#v", got, cfg)
		}
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newPlanClient = func(_ context.Context, token string, appID, installationID int64, appKeyPath string) (internalauth.Client, error) {
		if token != "secret-token" || appID != 0 || installationID != 0 || appKeyPath != "" {
			t.Fatalf("unexpected auth args token=%q appID=%d installationID=%d appKeyPath=%q", token, appID, installationID, appKeyPath)
		}
		return internalauth.MockClient{}, nil
	}
	collectPlanState = func(_ context.Context, opt collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		if opt.OrgName != "orang-gaboets" {
			t.Fatalf("unexpected organization %q", opt.OrgName)
		}
		return actual, nil
	}
	buildPlanReport = func(_ context.Context, opt gitopsplan.Options) (*gitopsplan.Report, error) {
		if !reflect.DeepEqual(opt.Desired, cfg) {
			t.Fatalf("unexpected desired config: got %#v want %#v", opt.Desired, cfg)
		}
		if opt.Actual != actual {
			t.Fatalf("unexpected actual state pointer")
		}
		return want, nil
	}

	cmd := PlanConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	got := decodePlanPreview(t, out.Bytes())
	got.Normalize()
	wantPreview := previewFromPlan(want)
	wantPreview.Normalize()
	if !reflect.DeepEqual(got, *wantPreview) {
		t.Fatalf("unexpected plan preview:\n got %#v\nwant %#v", got, *wantPreview)
	}
}

func TestPlanConfigCmdSuccessNoOpReport(t *testing.T) {
	restorePlanHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	want := &gitopsplan.Report{
		Organization: "orang-gaboets",
	}
	want.Normalize()

	loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return cfg, nil
	}
	validatePlanConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newPlanClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectPlanState = func(_ context.Context, _ collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return actual, nil
	}
	buildPlanReport = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) {
		return want, nil
	}

	cmd := PlanConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	got := decodePlanPreview(t, out.Bytes())
	got.Normalize()
	wantPreview := previewFromPlan(want)
	wantPreview.Normalize()
	if !reflect.DeepEqual(got, *wantPreview) {
		t.Fatalf("unexpected no-op plan preview:\n got %#v\nwant %#v", got, *wantPreview)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw plan preview: %v", err)
	}
	if _, ok := raw["summary"]; ok {
		t.Fatalf("did not expect summary key in payload: %s", out.String())
	}
	if _, ok := raw["actions"]; ok {
		t.Fatalf("did not expect actions key in payload: %s", out.String())
	}
	if _, ok := raw["plan_summary"]; !ok {
		t.Fatalf("expected plan_summary key in payload: %s", out.String())
	}
	if _, ok := raw["executable_actions"]; !ok {
		t.Fatalf("expected executable_actions key in payload: %s", out.String())
	}
	if _, ok := raw["skipped_actions"]; !ok {
		t.Fatalf("expected skipped_actions key in payload: %s", out.String())
	}
}

func TestPlanConfigCmdSplitsExecutableAndSkippedActions(t *testing.T) {
	restorePlanHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	report := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   "orang-gaboets/octostate",
				Executable:   true,
				Message:      "create repository orang-gaboets/octostate",
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeTeam,
				Operation:    gitopsplan.ActionOperationDelete,
				ResourceID:   "legacy-team",
				Executable:   false,
				Message:      "team legacy-team exists in live state but is not declared in desired config",
			},
		},
	}
	report.Normalize()

	loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return cfg, nil
	}
	validatePlanConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newPlanClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectPlanState = func(_ context.Context, _ collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return actual, nil
	}
	buildPlanReport = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) {
		return report, nil
	}

	cmd := PlanConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	got := decodePlanPreview(t, out.Bytes())
	normalizePlanPreviewForCompare(&got)
	want := previewFromPlan(report)
	normalizePlanPreviewForCompare(want)
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("unexpected split plan preview:\n got %#v\nwant %#v", got, *want)
	}
	if got.PlanSummary != report.Summary {
		t.Fatalf("unexpected plan summary: got %#v want %#v", got.PlanSummary, report.Summary)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw plan preview: %v", err)
	}
	if _, ok := raw["actions"]; ok {
		t.Fatalf("did not expect actions key in payload: %s", out.String())
	}
	if _, ok := raw["summary"]; ok {
		t.Fatalf("did not expect summary key in payload: %s", out.String())
	}
	if _, ok := raw["plan_summary"]; !ok {
		t.Fatalf("expected plan_summary key in payload: %s", out.String())
	}
}

func TestPreviewFromPlanNilReportReturnsEmptyPreview(t *testing.T) {
	got := previewFromPlan(nil)
	if got == nil {
		t.Fatal("expected non-nil preview")
		return
	}
	got.Normalize()
	if got.Organization != "" {
		t.Fatalf("expected empty organization, got %q", got.Organization)
	}
	if len(got.ExecutableActions) != 0 {
		t.Fatalf("expected no executable actions, got %#v", got.ExecutableActions)
	}
	if len(got.SkippedActions) != 0 {
		t.Fatalf("expected no skipped actions, got %#v", got.SkippedActions)
	}
}

func TestPlanConfigCmdLoadFailurePropagatesWithoutAuth(t *testing.T) {
	restorePlanHooks(t)

	tests := []struct {
		name    string
		loadErr error
	}{
		{
			name: "missing organization file",
			loadErr: &gitopsconfig.LoadError{
				Kind: gitopsconfig.LoadErrorMissingFile,
				Path: "/tmp/config/organization.yaml",
				Err:  errors.New("not found"),
			},
		},
		{
			name: "malformed organization file",
			loadErr: &gitopsconfig.LoadError{
				Kind: gitopsconfig.LoadErrorDecodeFile,
				Path: "/tmp/config/organization.yaml",
				Err:  errors.New("yaml: line 1: did not find expected key"),
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			restorePlanHooks(t)

			loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
				return gitopsconfig.OrganizationConfig{}, tt.loadErr
			}
			newPlanClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
				t.Fatal("newPlanClient should not be called after config load failure")
				return nil, nil
			}

			cmd := PlanConfigCmd()
			var out bytes.Buffer
			var errBuf bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errBuf)
			cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

			err := cmd.Execute()
			if !errors.Is(err, tt.loadErr) {
				t.Fatalf("unexpected error: got %v want %v", err, tt.loadErr)
			}
			if out.Len() != 0 {
				t.Fatalf("expected no stdout output, got %q", out.String())
			}
			if errBuf.Len() != 0 {
				t.Fatalf("expected no stderr output, got %q", errBuf.String())
			}
		})
	}
}

func TestPlanConfigCmdInvalidConfigPrintsStderrAndReturnsTypedExit(t *testing.T) {
	restorePlanHooks(t)

	loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	validatePlanConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: false}
	}
	newPlanClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newPlanClient should not be called for invalid config")
		return nil, nil
	}

	cmd := PlanConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if !strings.Contains(err.Error(), "configuration is invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: configuration is invalid; run `octostate config validate`") {
		t.Fatalf("expected invalid config error on stderr, got %q", got)
	}
}

func TestPlanConfigCmdAuthFailurePropagates(t *testing.T) {
	restorePlanHooks(t)

	wantErr := errors.New("auth failed")
	loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	validatePlanConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newPlanClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return nil, wantErr
	}

	cmd := PlanConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestPlanConfigCmdCollectorFailurePropagates(t *testing.T) {
	restorePlanHooks(t)

	wantErr := errors.New("collect failed")
	loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	validatePlanConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newPlanClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectPlanState = func(_ context.Context, _ collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return nil, wantErr
	}
	buildPlanReport = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) {
		t.Fatal("buildPlanReport should not be called after collector failure")
		return nil, nil
	}

	cmd := PlanConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestPlanConfigCmdBuildFailurePropagates(t *testing.T) {
	restorePlanHooks(t)

	wantErr := errors.New("build failed")
	loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	validatePlanConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newPlanClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectPlanState = func(_ context.Context, _ collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return &state.OrganizationState{Organization: "orang-gaboets"}, nil
	}
	buildPlanReport = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) {
		return nil, wantErr
	}

	cmd := PlanConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestPlanConfigCmdRequiresConfigDir(t *testing.T) {
	restorePlanHooks(t)

	cmd := PlanConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected required flag error")
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestPlanConfigRejectsBlankOrganizationBeforeAuth(t *testing.T) {
	restorePlanHooks(t)

	loadPlanConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "   "}, nil
	}
	validatePlanConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newPlanClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newPlanClient should not be called for blank organization")
		return nil, nil
	}

	_, err := planConfig(context.Background(), "secret-token", 0, 0, "", "./config")
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func restorePlanHooks(t *testing.T) {
	t.Helper()

	oldLoad := loadPlanConfig
	oldValidate := validatePlanConfig
	oldNewClient := newPlanClient
	oldCollect := collectPlanState
	oldBuild := buildPlanReport

	t.Cleanup(func() {
		loadPlanConfig = oldLoad
		validatePlanConfig = oldValidate
		newPlanClient = oldNewClient
		collectPlanState = oldCollect
		buildPlanReport = oldBuild
	})
}

func decodePlanPreview(t *testing.T, payload []byte) planPreview {
	t.Helper()

	var preview planPreview
	if err := json.Unmarshal(payload, &preview); err != nil {
		t.Fatalf("decode JSON preview: %v; payload=%q", err, string(payload))
	}
	normalizePlanPreviewForCompare(&preview)
	return preview
}

func normalizePlanPreviewForCompare(preview *planPreview) {
	if preview == nil {
		return
	}
	preview.Normalize()
	for i := range preview.ExecutableActions {
		preview.ExecutableActions[i].Normalize()
	}
	for i := range preview.SkippedActions {
		preview.SkippedActions[i].Normalize()
	}
}
