package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
	gitopsdiff "github.com/orang-gaboets/octostate/pkg/gitops/diff"
	gitopssnapshot "github.com/orang-gaboets/octostate/pkg/gitops/snapshot"
)

func TestDiffCmdSuccessNoDrift(t *testing.T) {
	restoreAuditDiffHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	snapshot := &gitopssnapshot.ActualSnapshot{
		Organization:                    "orang-gaboets",
		PulledAt:                        time.Date(2026, 3, 14, 8, 30, 0, 0, time.UTC),
		ResolvedInviteUserIDsByUsername: map[string]int64{"octocat": 99},
	}
	want := &gitopsdiff.Report{
		Organization:     "orang-gaboets",
		SnapshotPulledAt: snapshot.PulledAt,
	}
	want.Normalize()

	loadAuditDiffConfig = func(path string) (gitopsconfig.OrganizationConfig, error) {
		if path != "./config" {
			t.Fatalf("unexpected config path %q", path)
		}
		return cfg, nil
	}
	validateAuditDiffConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		if !reflect.DeepEqual(got, cfg) {
			t.Fatalf("unexpected config: got %#v want %#v", got, cfg)
		}
		return gitopsconfig.ValidationReport{Valid: true}
	}
	readAuditSnapshot = func(path string) (*gitopssnapshot.ActualSnapshot, error) {
		if path != "./state" {
			t.Fatalf("unexpected state path %q", path)
		}
		return snapshot, nil
	}
	buildAuditDiffReport = func(opt gitopsdiff.Options) (*gitopsdiff.Report, error) {
		if !reflect.DeepEqual(opt.Desired, cfg) {
			t.Fatalf("unexpected desired config: got %#v want %#v", opt.Desired, cfg)
		}
		if opt.Snapshot != snapshot {
			t.Fatal("unexpected snapshot pointer")
		}
		if opt.ResolvedInviteUserIDsByUsername != nil {
			t.Fatalf("expected ResolvedInviteUserIDsByUsername to be nil, got %#v", opt.ResolvedInviteUserIDsByUsername)
		}
		return want, nil
	}

	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	got := decodeDiffReport(t, out.Bytes())
	got.Normalize()
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("unexpected diff report:\n got %#v\nwant %#v", got, *want)
	}
}

func TestDiffCmdSuccessWithDriftDoesNotFailByDefault(t *testing.T) {
	restoreAuditDiffHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	snapshot := &gitopssnapshot.ActualSnapshot{Organization: "orang-gaboets"}
	want := &gitopsdiff.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsdiff.Action{{
			ResourceType: gitopsdiff.ActionResourceTypeRepository,
			Operation:    gitopsdiff.ActionOperationCreate,
			ResourceID:   "orang-gaboets/octostate",
			Executable:   true,
			Message:      "create repository orang-gaboets/octostate",
		}},
	}
	want.Normalize()

	loadAuditDiffConfig = func(string) (gitopsconfig.OrganizationConfig, error) { return cfg, nil }
	validateAuditDiffConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	readAuditSnapshot = func(string) (*gitopssnapshot.ActualSnapshot, error) { return snapshot, nil }
	buildAuditDiffReport = func(gitopsdiff.Options) (*gitopsdiff.Report, error) { return want, nil }

	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	got := decodeDiffReport(t, out.Bytes())
	got.Normalize()
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("unexpected diff report:\n got %#v\nwant %#v", got, *want)
	}
}

func TestDiffCmdFailOnDriftReturnsTypedExitAndStillPrintsReport(t *testing.T) {
	restoreAuditDiffHooks(t)

	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}
	snapshot := &gitopssnapshot.ActualSnapshot{Organization: "orang-gaboets"}
	want := &gitopsdiff.Report{
		Organization: "orang-gaboets",
		Actions: []gitopsdiff.Action{{
			ResourceType: gitopsdiff.ActionResourceTypeTeam,
			Operation:    gitopsdiff.ActionOperationDelete,
			ResourceID:   "platform",
			Executable:   false,
			Message:      "delete extra team platform",
		}},
	}
	want.Normalize()

	loadAuditDiffConfig = func(string) (gitopsconfig.OrganizationConfig, error) { return cfg, nil }
	validateAuditDiffConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	readAuditSnapshot = func(string) (*gitopssnapshot.ActualSnapshot, error) { return snapshot, nil }
	buildAuditDiffReport = func(gitopsdiff.Options) (*gitopsdiff.Report, error) { return want, nil }

	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state", "--fail-on-drift"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected typed drift exit error")
	}
	if code, ok := exitcode.Code(err); !ok || code != auditDiffExitCodeDrift {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", auditDiffExitCodeDrift, code, ok, err)
	}
	if !strings.Contains(err.Error(), "drift detected") {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	got := decodeDiffReport(t, out.Bytes())
	got.Normalize()
	if !reflect.DeepEqual(got, *want) {
		t.Fatalf("unexpected diff report:\n got %#v\nwant %#v", got, *want)
	}
}

func TestDiffCmdLoadFailurePropagatesWithoutSnapshotRead(t *testing.T) {
	restoreAuditDiffHooks(t)

	wantErr := errors.New("load config failed")
	loadAuditDiffConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{}, wantErr
	}
	readAuditSnapshot = func(string) (*gitopssnapshot.ActualSnapshot, error) {
		t.Fatal("readAuditSnapshot should not be called after config load failure")
		return nil, nil
	}

	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestDiffCmdInvalidConfigReturnsErrorWithoutSnapshotRead(t *testing.T) {
	restoreAuditDiffHooks(t)

	loadAuditDiffConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	validateAuditDiffConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: false}
	}
	readAuditSnapshot = func(string) (*gitopssnapshot.ActualSnapshot, error) {
		t.Fatal("readAuditSnapshot should not be called for invalid config")
		return nil, nil
	}

	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if !strings.Contains(err.Error(), "configuration is invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := exitcode.Code(err); ok {
		t.Fatalf("expected plain error for invalid config, got typed exit error: %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestDiffCmdRejectsMismatchedRepositoryOwnerBeforeSnapshotRead(t *testing.T) {
	restoreAuditDiffHooks(t)

	cfg := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{{
			Owner:      "shared-platform",
			Name:       "octostate",
			Visibility: "private",
		}},
		Teams: []gitopsconfig.TeamSpec{},
	}
	loadAuditDiffConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return cfg, nil
	}
	validateAuditDiffConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		if !reflect.DeepEqual(got, cfg) {
			t.Fatalf("unexpected config: got %#v want %#v", got, cfg)
		}
		report := gitopsconfig.Validate(got)
		assertAuditValidationReportHasIssue(t, report, "repositories[0].owner", gitopsconfig.ValidationIssueCodeRepositoryOwnerScope)
		return report
	}
	readAuditSnapshot = func(string) (*gitopssnapshot.ActualSnapshot, error) {
		t.Fatal("readAuditSnapshot should not be called for invalid repository owner")
		return nil, nil
	}
	buildAuditDiffReport = func(gitopsdiff.Options) (*gitopsdiff.Report, error) {
		t.Fatal("buildAuditDiffReport should not be called for invalid repository owner")
		return nil, nil
	}

	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if !strings.Contains(err.Error(), "configuration is invalid") {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := exitcode.Code(err); ok {
		t.Fatalf("expected plain error for invalid config, got typed exit error: %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestDiffCmdSnapshotReadAndBuildFailuresPropagate(t *testing.T) {
	snapshotErr := errors.New("snapshot read failed")
	buildErr := errors.New("diff build failed")

	tests := []struct {
		name    string
		arrange func()
		wantErr error
	}{
		{
			name:    "snapshot read failure",
			wantErr: snapshotErr,
			arrange: func() {
				loadAuditDiffConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
				}
				validateAuditDiffConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
					return gitopsconfig.ValidationReport{Valid: true}
				}
				readAuditSnapshot = func(string) (*gitopssnapshot.ActualSnapshot, error) {
					return nil, snapshotErr
				}
				buildAuditDiffReport = func(gitopsdiff.Options) (*gitopsdiff.Report, error) {
					t.Fatal("buildAuditDiffReport should not be called after snapshot read failure")
					return nil, nil
				}
			},
		},
		{
			name:    "build failure",
			wantErr: buildErr,
			arrange: func() {
				loadAuditDiffConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
				}
				validateAuditDiffConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
					return gitopsconfig.ValidationReport{Valid: true}
				}
				readAuditSnapshot = func(string) (*gitopssnapshot.ActualSnapshot, error) {
					return &gitopssnapshot.ActualSnapshot{Organization: "orang-gaboets"}, nil
				}
				buildAuditDiffReport = func(gitopsdiff.Options) (*gitopsdiff.Report, error) {
					return nil, buildErr
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restoreAuditDiffHooks(t)
			tt.arrange()

			cmd := DiffCmd()
			var out bytes.Buffer
			var errBuf bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errBuf)
			cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state"})

			err := cmd.Execute()
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("unexpected error: got %v want %v", err, tt.wantErr)
			}
			if out.Len() != 0 || errBuf.Len() != 0 {
				t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
			}
		})
	}
}

func TestDiffCmdRejectsBlankOrganizationBeforeSnapshotRead(t *testing.T) {
	restoreAuditDiffHooks(t)

	loadAuditDiffConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "   "}, nil
	}
	validateAuditDiffConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	readAuditSnapshot = func(string) (*gitopssnapshot.ActualSnapshot, error) {
		t.Fatal("readAuditSnapshot should not be called for blank organization")
		return nil, nil
	}

	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config", "--state-dir", "./state"})

	err := cmd.Execute()
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("unexpected error: got %v want %v", err, github.ErrMissingRequiredField)
	}
	if !strings.Contains(err.Error(), "organization is required") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestDiffCmdRequiresConfigDir(t *testing.T) {
	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--state-dir", "./state"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing config-dir flag error")
	}
	if !strings.Contains(err.Error(), "required flag(s) \"config-dir\"") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestDiffCmdRequiresStateDir(t *testing.T) {
	cmd := DiffCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", "./config"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing state-dir flag error")
	}
	if !strings.Contains(err.Error(), "required flag(s) \"state-dir\"") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func restoreAuditDiffHooks(t *testing.T) {
	t.Helper()

	oldLoad := loadAuditDiffConfig
	oldValidate := validateAuditDiffConfig
	oldRead := readAuditSnapshot
	oldBuild := buildAuditDiffReport

	t.Cleanup(func() {
		loadAuditDiffConfig = oldLoad
		validateAuditDiffConfig = oldValidate
		readAuditSnapshot = oldRead
		buildAuditDiffReport = oldBuild
	})
}

func decodeDiffReport(t *testing.T, payload []byte) gitopsdiff.Report {
	t.Helper()

	var report gitopsdiff.Report
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("decode diff report: %v; payload=%q", err, string(payload))
	}
	return report
}

func assertAuditValidationReportHasIssue(t *testing.T, report gitopsconfig.ValidationReport, wantPath string, wantCode gitopsconfig.ValidationIssueCode) {
	t.Helper()

	for _, issue := range report.Errors {
		if issue.Path == wantPath && issue.Code == wantCode {
			return
		}
	}
	t.Fatalf("expected validation issue path=%q code=%q, got %#v", wantPath, wantCode, report.Errors)
}
