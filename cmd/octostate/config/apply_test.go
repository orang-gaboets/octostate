package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	internalauth "github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsapply "github.com/orang-gaboets/octostate/pkg/gitops/apply"
	"github.com/orang-gaboets/octostate/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsplan "github.com/orang-gaboets/octostate/pkg/gitops/plan"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

type applyTemplateRepoService struct {
	internalauth.MockRepoService
}

func (s applyTemplateRepoService) Get(_ context.Context, _, repo string) (*gh.Repository, *gh.Response, error) {
	switch repo {
	case "zzz-template":
		return &gh.Repository{
			Name:       github.Ptr("zzz-template"),
			IsTemplate: github.Ptr(false),
		}, nil, nil
	case "aaa-app":
		return nil, nil, github.ErrNotFound
	default:
		return nil, nil, github.ErrNotFound
	}
}

func TestApplyConfigCmdSuccess(t *testing.T) {
	restoreApplyHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	report := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   "orang-gaboets/octostate",
			Executable:   true,
		}},
	}
	report.Normalize()
	want := &gitopsapply.Result{
		Organization: "orang-gaboets",
		PlanSummary:  report.Summary,
		Executed:     []gitopsplan.Action{report.Actions[0]},
		SkippedDrift: []gitopsplan.Action{},
	}
	want.Normalize()

	loadApplyConfig = func(path string) (gitopsconfig.OrganizationConfig, error) {
		if path != "./config" {
			t.Fatalf("unexpected config path %q", path)
		}
		return cfg, nil
	}
	validateApplyConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		if !reflect.DeepEqual(got, cfg) {
			t.Fatalf("unexpected config: got %#v want %#v", got, cfg)
		}
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newApplyClient = func(_ context.Context, token string, appID, installationID int64, appKeyPath string) (internalauth.Client, error) {
		if token != "secret-token" || appID != 0 || installationID != 0 || appKeyPath != "" {
			t.Fatalf("unexpected auth args token=%q appID=%d installationID=%d appKeyPath=%q", token, appID, installationID, appKeyPath)
		}
		return internalauth.MockClient{}, nil
	}
	collectApplyState = func(_ context.Context, opt collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		if opt.OrgName != cfg.Organization {
			t.Fatalf("unexpected organization %q", opt.OrgName)
		}
		return actual, nil
	}
	buildApplyPlan = func(_ context.Context, opt gitopsplan.Options) (*gitopsplan.Report, error) {
		if !reflect.DeepEqual(opt.Desired, cfg) {
			t.Fatalf("unexpected desired config: got %#v want %#v", opt.Desired, cfg)
		}
		if opt.Actual != actual {
			t.Fatalf("unexpected actual state pointer")
		}
		return report, nil
	}
	executeApply = func(_ context.Context, opt gitopsapply.Options) (*gitopsapply.Result, error) {
		if !reflect.DeepEqual(opt.Desired, cfg) {
			t.Fatalf("unexpected desired config: got %#v want %#v", opt.Desired, cfg)
		}
		if opt.Actual != actual {
			t.Fatalf("unexpected actual state pointer")
		}
		if opt.Plan != report {
			t.Fatalf("unexpected plan pointer")
		}
		return want, nil
	}

	cmd := ApplyConfigCmd()
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

	envelope := decodeApplyEnvelope(t, out.Bytes())
	if envelope.Status != string(cmdoutput.OperationResultStatusSuccess) {
		t.Fatalf("unexpected status: got %q want %q", envelope.Status, cmdoutput.OperationResultStatusSuccess)
	}
	var got gitopsapply.Result
	decodeApplyData(t, envelope.Data, &got)
	got.Normalize()
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("unexpected apply result:\n got %#v\nwant %#v", got, *want)
	}
}

func TestApplyConfigCmdDryRunSkipsExecution(t *testing.T) {
	restoreApplyHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	report := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{ResourceType: gitopsplan.ActionResourceTypeRepository, Operation: gitopsplan.ActionOperationCreate, ResourceID: "orang-gaboets/octostate", Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationDelete, ResourceID: "platform", Executable: false},
		},
	}
	report.Normalize()

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
	executeApply = func(context.Context, gitopsapply.Options) (*gitopsapply.Result, error) {
		t.Fatal("executeApply should not be called during dry-run")
		return nil, nil
	}

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token", "--dry-run"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	envelope := decodeApplyEnvelope(t, out.Bytes())
	if envelope.Status != string(cmdoutput.OperationResultStatusDryRun) {
		t.Fatalf("unexpected status: got %q want %q", envelope.Status, cmdoutput.OperationResultStatusDryRun)
	}
	var got planPreview
	decodeApplyData(t, envelope.Data, &got)
	normalizePlanPreviewForCompare(&got)
	want := previewFromPlan(report)
	normalizePlanPreviewForCompare(want)
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("unexpected dry-run preview:\n got %#v\nwant %#v", got, *want)
	}
}

func TestApplyConfigCmdCheckSkipsExecution(t *testing.T) {
	restoreApplyHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	report := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{
			{ResourceType: gitopsplan.ActionResourceTypeRepository, Operation: gitopsplan.ActionOperationCreate, ResourceID: "orang-gaboets/octostate", Executable: true},
			{ResourceType: gitopsplan.ActionResourceTypeTeam, Operation: gitopsplan.ActionOperationDelete, ResourceID: "legacy-team", Executable: false},
		},
	}
	report.Normalize()
	check := &gitopsapply.CheckResult{
		Organization:   "orang-gaboets",
		PlanSummary:    report.Summary,
		CheckedActions: []gitopsplan.Action{report.Actions[0]},
		SkippedActions: []gitopsplan.Action{report.Actions[1]},
	}
	check.Normalize()

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
	checkApply = func(context.Context, gitopsapply.Options) (*gitopsapply.CheckResult, error) {
		return check, nil
	}
	executeApply = func(context.Context, gitopsapply.Options) (*gitopsapply.Result, error) {
		t.Fatal("executeApply should not be called during check")
		return nil, nil
	}

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token", "--check"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	envelope := decodeApplyEnvelope(t, out.Bytes())
	if envelope.Status != string(cmdoutput.OperationResultStatusCheck) {
		t.Fatalf("unexpected status: got %q want %q", envelope.Status, cmdoutput.OperationResultStatusCheck)
	}
	var got gitopsapply.CheckResult
	decodeApplyData(t, envelope.Data, &got)
	got.Normalize()
	if !reflect.DeepEqual(got, *check) {
		t.Fatalf("unexpected check result:\n got %#v\nwant %#v", got, *check)
	}
}

func TestApplyConfigCmdCheckFailurePropagatesWithoutSuccessOutput(t *testing.T) {
	restoreApplyHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	report := &gitopsplan.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsplan.Action{{
			ResourceType: gitopsplan.ActionResourceTypeRepository,
			Operation:    gitopsplan.ActionOperationCreate,
			ResourceID:   "orang-gaboets/octostate",
			Executable:   true,
		}},
	}
	report.Normalize()
	checkErr := errors.New("apply preflight failed for 2 action(s): create repository orang-gaboets/octostate: target repository orang-gaboets/octostate already exists: invalid field value")

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
	checkApply = func(context.Context, gitopsapply.Options) (*gitopsapply.CheckResult, error) {
		return nil, checkErr
	}

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token", "--check"})

	err := cmd.Execute()
	if !errors.Is(err, checkErr) {
		t.Fatalf("unexpected error: got %v want %v", err, checkErr)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no command output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestApplyConfigCmdRejectsCheckAndDryRunTogether(t *testing.T) {
	restoreApplyHooks(t)

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token", "--check", "--dry-run"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected conflicting flag error")
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if got := errBuf.String(); !strings.Contains(got, "cannot use --check and --dry-run together") {
		t.Fatalf("expected conflicting flag error on stderr, got %q", got)
	}
}

func TestApplyConfigCmdLoadFailurePropagatesWithoutAuth(t *testing.T) {
	restoreApplyHooks(t)

	wantErr := &gitopsconfig.LoadError{Kind: gitopsconfig.LoadErrorMissingFile, Path: "/tmp/config/organization.yaml", Err: errors.New("not found")}
	loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{}, wantErr
	}
	newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newApplyClient should not be called after config load failure")
		return nil, nil
	}

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	err := cmd.Execute()
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("expected load failure message, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: failed to load config") {
		t.Fatalf("expected load failure on stderr, got %q", got)
	}
}

func TestApplyConfigCmdInvalidConfigPrintsStderrAndReturnsTypedExit(t *testing.T) {
	restoreApplyHooks(t)

	loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: false}
	}
	newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newApplyClient should not be called for invalid config")
		return nil, nil
	}

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid config error")
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

func TestApplyConfigCmdAuthCollectorBuildAndExecuteFailuresPropagate(t *testing.T) {
	authErr := errors.New("auth failed")
	collectErr := errors.New("collect failed")
	buildErr := errors.New("build failed")
	execErr := errors.New("apply failed")

	tests := []struct {
		name    string
		arrange func()
		wantErr error
	}{
		{
			name:    "auth failure",
			wantErr: authErr,
			arrange: func() {
				loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
				}
				validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
					return gitopsconfig.ValidationReport{Valid: true}
				}
				newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) { return nil, authErr }
			},
		},
		{
			name:    "collector failure",
			wantErr: collectErr,
			arrange: func() {
				loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
				}
				validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
					return gitopsconfig.ValidationReport{Valid: true}
				}
				newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
					return internalauth.MockClient{}, nil
				}
				collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
					return nil, collectErr
				}
				buildApplyPlan = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) {
					t.Fatal("buildApplyPlan should not be called after collector failure")
					return nil, nil
				}
			},
		},
		{
			name:    "planner failure",
			wantErr: buildErr,
			arrange: func() {
				loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
				}
				validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
					return gitopsconfig.ValidationReport{Valid: true}
				}
				newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
					return internalauth.MockClient{}, nil
				}
				collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
					return &state.OrganizationState{Organization: "orang-gaboets"}, nil
				}
				buildApplyPlan = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) { return nil, buildErr }
			},
		},
		{
			name:    "executor failure",
			wantErr: execErr,
			arrange: func() {
				loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
				}
				validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
					return gitopsconfig.ValidationReport{Valid: true}
				}
				newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
					return internalauth.MockClient{}, nil
				}
				collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
					return &state.OrganizationState{Organization: "orang-gaboets"}, nil
				}
				buildApplyPlan = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) {
					return &gitopsplan.Report{Organization: "orang-gaboets"}, nil
				}
				executeApply = func(context.Context, gitopsapply.Options) (*gitopsapply.Result, error) { return nil, execErr }
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreApplyHooks(t)
			tt.arrange()

			cmd := ApplyConfigCmd()
			var out bytes.Buffer
			var errBuf bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errBuf)
			cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token"})

			err := cmd.Execute()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error: got %v want %v", err, tt.wantErr)
			}
			switch tt.name {
			case "auth failure":
				if !strings.Contains(err.Error(), "failed to create GitHub client: auth failed") {
					t.Fatalf("unexpected error message: %v", err)
				}
			case "collector failure":
				if !strings.Contains(err.Error(), "failed to collect live GitHub state: collect failed") {
					t.Fatalf("unexpected error message: %v", err)
				}
			case "planner failure":
				if !strings.Contains(err.Error(), "failed to build reconciliation plan: build failed") {
					t.Fatalf("unexpected error message: %v", err)
				}
			case "executor failure":
				if !strings.Contains(err.Error(), "failed to execute apply plan: apply failed") {
					t.Fatalf("unexpected error message: %v", err)
				}
			}
			if out.Len() != 0 || errBuf.Len() != 0 {
				t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
			}
		})
	}
}

func TestApplyConfigCmdRequiresConfigDir(t *testing.T) {
	restoreApplyHooks(t)

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected required flag error")
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestApplyConfigRejectsBlankOrganizationBeforeAuth(t *testing.T) {
	restoreApplyHooks(t)

	loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "   "}, nil
	}
	validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newApplyClient should not be called for blank organization")
		return nil, nil
	}

	_, _, _, err := applyConfig(context.Background(), "secret-token", 0, 0, "", "./config", applyRunModeApply)
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("unexpected error: %v", err)
	}
}

type applyTemplateOrderingFixture struct {
	desired     gitopsconfig.OrganizationConfig
	actual      *state.OrganizationState
	wantChecked []gitopsplan.Action
}

func setupApplyTemplateOrderingFixture(t *testing.T) applyTemplateOrderingFixture {
	t.Helper()

	templateRepo := gitopsconfig.RepositorySpec{
		Owner:      "orang-gaboets",
		Name:       "zzz-template",
		Visibility: "private",
	}
	templateRepo.SetManagedIsTemplate(true)

	desired := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []gitopsconfig.RepositorySpec{
			{
				Owner:      "orang-gaboets",
				Name:       "aaa-app",
				Visibility: "private",
				Template: gitopsconfig.TemplateSpec{
					Owner: "orang-gaboets",
					Name:  "zzz-template",
				},
			},
			templateRepo,
		},
	}
	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{{
			Owner:      "orang-gaboets",
			Name:       "zzz-template",
			Visibility: "private",
			IsTemplate: false,
		}},
	}

	loadApplyConfig = func(path string) (gitopsconfig.OrganizationConfig, error) {
		if path != "./config" {
			t.Fatalf("unexpected config path %q", path)
		}
		return desired, nil
	}
	validateApplyConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		if !reflect.DeepEqual(got, desired) {
			t.Fatalf("unexpected config: got %#v want %#v", got, desired)
		}
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newApplyClient = func(_ context.Context, token string, appID, installationID int64, appKeyPath string) (internalauth.Client, error) {
		if token != "secret-token" || appID != 0 || installationID != 0 || appKeyPath != "" {
			t.Fatalf("unexpected auth args token=%q appID=%d installationID=%d appKeyPath=%q", token, appID, installationID, appKeyPath)
		}
		return internalauth.MockClient{
			OrganizationsService: internalauth.MockOrganizationService{},
			ReposService:         applyTemplateRepoService{MockRepoService: internalauth.MockRepoService{}},
			TeamsService:         internalauth.MockTeamsService{},
			UsersService:         internalauth.MockUserService{},
		}, nil
	}
	collectApplyState = func(_ context.Context, opt collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		if opt.OrgName != desired.Organization {
			t.Fatalf("unexpected organization %q", opt.OrgName)
		}
		return actual, nil
	}

	return applyTemplateOrderingFixture{
		desired: desired,
		actual:  actual,
		wantChecked: []gitopsplan.Action{
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationUpdate,
				ResourceID:   "orang-gaboets/zzz-template",
				Executable:   true,
				Message:      "update repository orang-gaboets/zzz-template",
				Changes: []gitopsplan.FieldChange{{
					Field: "is_template",
					From:  false,
					To:    true,
				}},
			},
			{
				ResourceType: gitopsplan.ActionResourceTypeRepository,
				Operation:    gitopsplan.ActionOperationCreate,
				ResourceID:   "orang-gaboets/aaa-app",
				Executable:   true,
				Message:      "create repository orang-gaboets/aaa-app",
				Changes:      []gitopsplan.FieldChange{},
			},
		},
	}
}

func restoreApplyHooks(t *testing.T) {
	t.Helper()

	oldLoad := loadApplyConfig
	oldValidate := validateApplyConfig
	oldNewClient := newApplyClient
	oldCollect := collectApplyState
	oldBuild := buildApplyPlan
	oldCheck := checkApply
	oldExecute := executeApply

	t.Cleanup(func() {
		loadApplyConfig = oldLoad
		validateApplyConfig = oldValidate
		newApplyClient = oldNewClient
		collectApplyState = oldCollect
		buildApplyPlan = oldBuild
		checkApply = oldCheck
		executeApply = oldExecute
	})
}

type applyEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func decodeApplyEnvelope(t *testing.T, payload []byte) applyEnvelope {
	t.Helper()

	var envelope applyEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode apply envelope: %v; payload=%q", err, string(payload))
	}
	return envelope
}

func decodeApplyData(t *testing.T, payload json.RawMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(payload, out); err != nil {
		t.Fatalf("decode apply data: %v; payload=%q", err, string(payload))
	}
}

func TestApplyConfigCheckUsesRealPlannerAndPreflight(t *testing.T) {
	restoreApplyHooks(t)

	fixture := setupApplyTemplateOrderingFixture(t)

	result, preview, checkResult, err := applyConfig(context.Background(), "secret-token", 0, 0, "", "./config", applyRunModeCheck)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview != nil {
		t.Fatalf("expected no dry-run preview, got %#v", preview)
	}
	if result != nil {
		t.Fatalf("expected no apply result, got %#v", result)
	}
	if checkResult == nil {
		t.Fatal("expected check result")
		return
	}
	gotCheckResult := *checkResult
	if gotCheckResult.Organization != fixture.desired.Organization {
		t.Fatalf("unexpected organization: got %q want %q", gotCheckResult.Organization, fixture.desired.Organization)
	}
	if gotCheckResult.PlanSummary.ExecutableActions != len(fixture.wantChecked) {
		t.Fatalf("unexpected executable action count: got %d want %d", gotCheckResult.PlanSummary.ExecutableActions, len(fixture.wantChecked))
	}
	if !reflect.DeepEqual(gotCheckResult.CheckedActions, fixture.wantChecked) {
		t.Fatalf("unexpected checked actions:\n got %#v\nwant %#v", gotCheckResult.CheckedActions, fixture.wantChecked)
	}
	if len(gotCheckResult.SkippedActions) != 0 {
		t.Fatalf("expected no skipped actions, got %#v", gotCheckResult.SkippedActions)
	}
}

func TestApplyConfigCmdCheckUsesRealPlannerAndPreflight(t *testing.T) {
	restoreApplyHooks(t)

	fixture := setupApplyTemplateOrderingFixture(t)

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--token", "secret-token", "--check"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	envelope := decodeApplyEnvelope(t, out.Bytes())
	if envelope.Status != string(cmdoutput.OperationResultStatusCheck) {
		t.Fatalf("unexpected status: got %q want %q", envelope.Status, cmdoutput.OperationResultStatusCheck)
	}
	var got gitopsapply.CheckResult
	decodeApplyData(t, envelope.Data, &got)
	got.Normalize()
	if got.Organization != fixture.desired.Organization {
		t.Fatalf("unexpected organization: got %q want %q", got.Organization, fixture.desired.Organization)
	}
	if got.PlanSummary.ExecutableActions != len(fixture.wantChecked) {
		t.Fatalf("unexpected executable action count: got %d want %d", got.PlanSummary.ExecutableActions, len(fixture.wantChecked))
	}
	if !reflect.DeepEqual(got.CheckedActions, fixture.wantChecked) {
		t.Fatalf("unexpected checked actions:\n got %#v\nwant %#v", got.CheckedActions, fixture.wantChecked)
	}
	if len(got.SkippedActions) != 0 {
		t.Fatalf("expected no skipped actions, got %#v", got.SkippedActions)
	}
}
