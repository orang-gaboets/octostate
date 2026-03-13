package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	internalauth "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/repo-builder/pkg/gitops/plan"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
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
			ResourceID:   "orang-gaboets/repo-builder",
			Executable:   true,
			Message:      "create repository orang-gaboets/repo-builder",
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

	got := decodePlanReport(t, out.Bytes())
	got.Normalize()
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("unexpected plan report:\n got %#v\nwant %#v", got, *want)
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

	got := decodePlanReport(t, out.Bytes())
	got.Normalize()
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("unexpected no-op plan report:\n got %#v\nwant %#v", got, *want)
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

func TestPlanConfigCmdInvalidConfigReturnsErrorWithoutStderr(t *testing.T) {
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
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
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

func decodePlanReport(t *testing.T, payload []byte) gitopsplan.Report {
	t.Helper()

	var report gitopsplan.Report
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("decode JSON report: %v; payload=%q", err, string(payload))
	}
	return report
}
