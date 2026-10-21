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

type applyReviewedRepoService struct {
	internalauth.MockRepoService
	createdRepositories []string
	deletedRepositories []string
}

func (s *applyReviewedRepoService) Create(_ context.Context, owner string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
	s.createdRepositories = append(s.createdRepositories, owner+"/"+repository.GetName())
	return repository, nil, nil
}

func (s *applyReviewedRepoService) Delete(_ context.Context, owner, repository string) (*gh.Response, error) {
	s.deletedRepositories = append(s.deletedRepositories, owner+"/"+repository)
	return &gh.Response{}, nil
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
	var outputData map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Data, &outputData); err != nil {
		t.Fatalf("decode raw apply data: %v", err)
	}
	var gotIdentity string
	if err := json.Unmarshal(outputData["desired_config_identity"], &gotIdentity); err != nil {
		t.Fatalf("decode desired config identity: %v; data=%s", err, string(envelope.Data))
	}
	wantIdentity, err := gitopsconfig.DesiredStateIdentity(cfg)
	if err != nil {
		t.Fatalf("compute expected desired config identity: %v", err)
	}
	if gotIdentity != wantIdentity {
		t.Fatalf("unexpected desired config identity: got %q want %q", gotIdentity, wantIdentity)
	}
}

func TestApplyConfigCmdExpectedIdentityMismatchStopsBeforeAuthAndCollection(t *testing.T) {
	restoreApplyHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) { return cfg, nil }
	validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("authentication must not run after an expected identity mismatch")
		return nil, nil
	}
	collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("live collection must not run after an expected identity mismatch")
		return nil, nil
	}

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--config-dir", "./config",
		"--expected-config-identity", "octostate-config-v1:sha256:" + strings.Repeat("0", 64),
	})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected a desired config identity mismatch")
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if !strings.Contains(err.Error(), "does not match expected identity") {
		t.Fatalf("expected identity mismatch diagnostic, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("mismatch must not write stdout, got %q", out.String())
	}
	if errBuf.Len() == 0 {
		t.Fatal("expected mismatch diagnostic on stderr")
	}
}

func TestApplyConfigCmdReviewedCheckReturnsDesiredIdentity(t *testing.T) {
	restoreApplyHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	identity, err := gitopsconfig.DesiredStateIdentity(cfg)
	if err != nil {
		t.Fatalf("compute desired config identity: %v", err)
	}
	loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) { return cfg, nil }
	validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{
			OrganizationsService: internalauth.MockOrganizationService{},
			ReposService:         internalauth.MockRepoService{},
			TeamsService:         internalauth.MockTeamsService{},
			UsersService:         internalauth.MockUserService{},
		}, nil
	}
	collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return actual, nil
	}

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--config-dir", "./config",
		"--check",
		"--expected-config-identity", identity,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reviewed check failed: %v; stderr=%s", err, errBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	envelope := decodeApplyEnvelope(t, out.Bytes())
	var data map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("decode apply check data: %v", err)
	}
	var gotIdentity string
	if err := json.Unmarshal(data["desired_config_identity"], &gotIdentity); err != nil {
		t.Fatalf("decode desired config identity: %v; data=%s", err, string(envelope.Data))
	}
	if gotIdentity != identity {
		t.Fatalf("unexpected desired config identity: got %q want %q", gotIdentity, identity)
	}
}

func TestApplyConfigCmdReviewedLiveApplyIncludesFreshPlanAndSkippedDrift(t *testing.T) {
	restoreApplyHooks(t)

	cfg := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Repositories: []gitopsconfig.RepositorySpec{{
			Name: "service", Visibility: "private",
		}},
	}
	identity, err := gitopsconfig.DesiredStateIdentity(cfg)
	if err != nil {
		t.Fatalf("compute desired config identity: %v", err)
	}
	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		Repositories: []state.Repository{{
			Owner: "orang-gaboets", Name: "orphan", Visibility: "private",
		}},
	}
	repoService := &applyReviewedRepoService{}
	collectionCalls := 0
	loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) { return cfg, nil }
	validateApplyConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{
			OrganizationsService: internalauth.MockOrganizationService{},
			ReposService:         repoService,
			TeamsService:         internalauth.MockTeamsService{},
			UsersService:         internalauth.MockUserService{},
		}, nil
	}
	collectApplyState = func(_ context.Context, options collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		collectionCalls++
		if options.OrgName != "orang-gaboets" {
			t.Fatalf("unexpected collection organization %q", options.OrgName)
		}
		return actual, nil
	}

	cmd := ApplyConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--config-dir", "./config",
		"--expected-config-identity", identity,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("reviewed live apply failed: %v; stderr=%s", err, errBuf.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if collectionCalls != 1 {
		t.Fatalf("expected one fresh live-state collection, got %d", collectionCalls)
	}

	envelope := decodeApplyEnvelope(t, out.Bytes())
	if envelope.Status != string(cmdoutput.OperationResultStatusSuccess) {
		t.Fatalf("unexpected status: got %q want %q", envelope.Status, cmdoutput.OperationResultStatusSuccess)
	}
	var outputData map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Data, &outputData); err != nil {
		t.Fatalf("decode apply data: %v", err)
	}
	var gotIdentity string
	if err := json.Unmarshal(outputData["desired_config_identity"], &gotIdentity); err != nil {
		t.Fatalf("decode desired config identity: %v", err)
	}
	if gotIdentity != identity {
		t.Fatalf("unexpected desired config identity: got %q want %q", gotIdentity, identity)
	}
	var result gitopsapply.Result
	decodeApplyData(t, envelope.Data, &result)
	result.Normalize()
	if len(result.Executed) != 1 || result.Executed[0].ResourceID != "orang-gaboets/service" || result.Executed[0].Operation != gitopsplan.ActionOperationCreate {
		t.Fatalf("expected the freshly planned service create, got %#v", result.Executed)
	}
	if len(result.SkippedDrift) != 1 || result.SkippedDrift[0].ResourceID != "orang-gaboets/orphan" || result.SkippedDrift[0].Operation != gitopsplan.ActionOperationDelete || result.SkippedDrift[0].Executable {
		t.Fatalf("expected orphan repository delete to remain skipped drift, got %#v", result.SkippedDrift)
	}
	if !reflect.DeepEqual(repoService.createdRepositories, []string{"orang-gaboets/service"}) {
		t.Fatalf("unexpected repository creates: %#v", repoService.createdRepositories)
	}
	if len(repoService.deletedRepositories) != 0 {
		t.Fatalf("unsupported destructive drift must not be deleted: %#v", repoService.deletedRepositories)
	}
}

func TestApplyConfigCmdDryRunSkipsExecution(t *testing.T) {
	restoreApplyHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	expectedIdentity, identityErr := gitopsconfig.DesiredStateIdentity(cfg)
	if identityErr != nil {
		t.Fatalf("compute desired config identity: %v", identityErr)
	}
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
	cmd.SetArgs([]string{
		"--config-dir", "./config", "--token", "secret-token", "--dry-run",
		"--expected-config-identity", expectedIdentity,
	})

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
	wantIdentity, err := gitopsconfig.DesiredStateIdentity(cfg)
	if err != nil {
		t.Fatalf("compute expected desired config identity: %v", err)
	}
	want.DesiredConfigIdentity = wantIdentity
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

func TestApplyConfigCmdRejectsMismatchedRepositoryOwnerBeforeGitHub(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "apply", args: []string{"--config-dir", "./config", "--token", "secret-token"}},
		{name: "check", args: []string{"--config-dir", "./config", "--token", "secret-token", "--check"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreApplyHooks(t)

			cfg := mismatchedRepositoryOwnerConfig()
			loadApplyConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
				return cfg, nil
			}
			validateApplyConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
				if !reflect.DeepEqual(got, cfg) {
					t.Fatalf("unexpected config: got %#v want %#v", got, cfg)
				}
				report := gitopsconfig.Validate(got)
				assertValidationReportHasIssue(t, report, "repositories[0].owner", gitopsconfig.ValidationIssueCodeRepositoryOwnerScope)
				return report
			}
			newApplyClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
				t.Fatal("newApplyClient should not be called for invalid repository owner")
				return nil, nil
			}
			collectApplyState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
				t.Fatal("collectApplyState should not be called for invalid repository owner")
				return nil, nil
			}
			buildApplyPlan = func(context.Context, gitopsplan.Options) (*gitopsplan.Report, error) {
				t.Fatal("buildApplyPlan should not be called for invalid repository owner")
				return nil, nil
			}
			checkApply = func(context.Context, gitopsapply.Options) (*gitopsapply.CheckResult, error) {
				t.Fatal("checkApply should not be called for invalid repository owner")
				return nil, nil
			}
			executeApply = func(context.Context, gitopsapply.Options) (*gitopsapply.Result, error) {
				t.Fatal("executeApply should not be called for invalid repository owner")
				return nil, nil
			}

			cmd := ApplyConfigCmd()
			var out bytes.Buffer
			var errBuf bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errBuf)
			cmd.SetArgs(tt.args)

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
		})
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

	_, _, _, _, err := applyConfig(context.Background(), "secret-token", 0, 0, "", "./config", applyRunModeApply, false, nil, nil)
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

	result, preview, checkResult, desiredConfigIdentity, err := applyConfig(context.Background(), "secret-token", 0, 0, "", "./config", applyRunModeCheck, false, nil, nil)
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
	if desiredConfigIdentity == "" {
		t.Fatal("expected desired config identity")
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
