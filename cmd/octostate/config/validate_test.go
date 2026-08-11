package config_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configcmd "github.com/orang-gaboets/octostate/cmd/octostate/config"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

func TestValidateConfigCmdValid(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites: []
repositories: []
teams: []
`)

	cmd := configcmd.ValidateConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", configDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report := decodeReport(t, out.Bytes())
	if !report.Valid {
		t.Fatalf("expected valid report, got %#v", report)
	}
	if report.Summary.Errors != 0 {
		t.Fatalf("expected zero errors, got %#v", report.Summary)
	}
}

func TestValidateConfigCmdAllowsNullRepositoryTemplateState(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites: []
repositories:
  - name: repo-template
    visibility: private
    is_template: null
teams: []
`)

	cmd := configcmd.ValidateConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", configDir})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report := decodeReport(t, out.Bytes())
	if !report.Valid || report.Summary.Errors != 0 {
		t.Fatalf("expected valid report, got %#v", report)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestValidateConfigCmdInvalidSemanticConfigReturnsExitCode2(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites: []
repositories:
  - name: octostate
    visibility: internal
teams: []
`)

	cmd := configcmd.ValidateConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", configDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}

	code, ok := exitcode.Code(err)
	if !ok || code != 2 {
		t.Fatalf("expected typed exit code 2, got ok=%v code=%d err=%v", ok, code, err)
	}

	report := decodeReport(t, out.Bytes())
	if report.Valid {
		t.Fatalf("expected invalid report, got %#v", report)
	}
	if report.Summary.Errors == 0 || len(report.Errors) == 0 {
		t.Fatalf("expected validation errors in report, got %#v", report)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestValidateConfigCmdRejectsMismatchedRepositoryOwner(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeOrganizationYAML(t, configDir, `
organization: orang-gaboets
members: []
invites: []
repositories:
  - owner: shared-platform
    name: octostate
    visibility: private
teams: []
`)

	cmd := configcmd.ValidateConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", configDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}
	if code, ok := exitcode.Code(err); !ok || code != 2 {
		t.Fatalf("expected typed exit code 2, got ok=%v code=%d err=%v", ok, code, err)
	}

	report := decodeReport(t, out.Bytes())
	if report.Valid {
		t.Fatalf("expected invalid report, got %#v", report)
	}
	if len(report.Errors) != 1 {
		t.Fatalf("expected one validation error, got %#v", report.Errors)
	}
	if got, want := report.Errors[0].Path, "repositories[0].owner"; got != want {
		t.Fatalf("unexpected validation path: got %q want %q", got, want)
	}
	if got, want := report.Errors[0].Code, gitopsconfig.ValidationIssueCodeRepositoryOwnerScope; got != want {
		t.Fatalf("unexpected validation code: got %q want %q", got, want)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestValidateConfigCmdMissingOrganizationFileReturnsExitCode1(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()

	cmd := configcmd.ValidateConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", configDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected load error")
	}

	code, ok := exitcode.Code(err)
	if !ok || code != 1 {
		t.Fatalf("expected typed exit code 1, got ok=%v code=%d err=%v", ok, code, err)
	}

	report := decodeReport(t, out.Bytes())
	if report.Valid {
		t.Fatalf("expected invalid report, got %#v", report)
	}
	if report.Summary.Errors != 1 || len(report.Errors) != 1 {
		t.Fatalf("expected one load error issue, got %#v", report)
	}
	if report.Errors[0].Code != gitopsconfig.ValidationIssueCode(gitopsconfig.LoadErrorMissingFile) {
		t.Fatalf("expected missing_file issue code, got %q", report.Errors[0].Code)
	}
	if !strings.HasSuffix(report.Errors[0].Path, "organization.yaml") {
		t.Fatalf("expected path to include organization.yaml, got %q", report.Errors[0].Path)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestValidateConfigCmdZeroInviteUserIDReturnsExitCode2(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites:
  - user_id: 0
repositories: []
teams: []
`)

	cmd := configcmd.ValidateConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", configDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}

	code, ok := exitcode.Code(err)
	if !ok || code != 2 {
		t.Fatalf("expected typed exit code 2, got ok=%v code=%d err=%v", ok, code, err)
	}

	report := decodeReport(t, out.Bytes())
	if report.Valid {
		t.Fatalf("expected invalid report, got %#v", report)
	}
	if report.Summary.Errors == 0 || len(report.Errors) == 0 {
		t.Fatalf("expected validation errors in report, got %#v", report)
	}
}

func TestValidateConfigCmdInvalidInviteEmailReturnsExitCode2(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites:
  - email: invalid-email
repositories: []
teams: []
`)

	cmd := configcmd.ValidateConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", configDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}

	code, ok := exitcode.Code(err)
	if !ok || code != 2 {
		t.Fatalf("expected typed exit code 2, got ok=%v code=%d err=%v", ok, code, err)
	}

	report := decodeReport(t, out.Bytes())
	if report.Valid {
		t.Fatalf("expected invalid report, got %#v", report)
	}
	if report.Summary.Errors == 0 || len(report.Errors) == 0 {
		t.Fatalf("expected validation errors in report, got %#v", report)
	}
}

func TestValidateConfigCmdNullInviteUserIDReturnsExitCode2(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	writeOrganizationYAML(t, configDir, `
organization: orang-gaboets
invites:
  - user_id: null
repositories: []
teams: []
`)

	cmd := configcmd.ValidateConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--config-dir", configDir})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected validation error")
	}

	code, ok := exitcode.Code(err)
	if !ok || code != 2 {
		t.Fatalf("expected typed exit code 2, got ok=%v code=%d err=%v", ok, code, err)
	}

	report := decodeReport(t, out.Bytes())
	if report.Valid {
		t.Fatalf("expected invalid report, got %#v", report)
	}
	if report.Summary.Errors == 0 || len(report.Errors) == 0 {
		t.Fatalf("expected validation errors in report, got %#v", report)
	}
}

func writeOrganizationYAML(t *testing.T, configDir, contents string) {
	t.Helper()

	path := filepath.Join(configDir, "organization.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(contents)+"\n"), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func decodeReport(t *testing.T, payload []byte) gitopsconfig.ValidationReport {
	t.Helper()

	var report gitopsconfig.ValidationReport
	if err := json.Unmarshal(payload, &report); err != nil {
		t.Fatalf("decode JSON report: %v; payload=%q", err, string(payload))
	}
	return report
}
