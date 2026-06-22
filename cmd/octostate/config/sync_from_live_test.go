package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	internalauth "github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
	cmdoutput "github.com/orang-gaboets/octostate/cmd/octostate/internal/output"
	"github.com/orang-gaboets/octostate/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
	"github.com/orang-gaboets/octostate/pkg/gitops/state"
	"github.com/orang-gaboets/octostate/pkg/gitops/syncfromlive"
)

func TestSyncFromLiveConfigCmdPrintsBootstrapYAML(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets", Invites: []gitopsconfig.InviteSpec{}, Repositories: []gitopsconfig.RepositorySpec{}, Teams: []gitopsconfig.TeamSpec{}}

	newSyncFromLiveClient = func(_ context.Context, token string, appID, installationID int64, appKeyPath string) (internalauth.Client, error) {
		if token != "secret-token" || appID != 0 || installationID != 0 || appKeyPath != "" {
			t.Fatalf("unexpected auth args token=%q appID=%d installationID=%d appKeyPath=%q", token, appID, installationID, appKeyPath)
		}
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(_ context.Context, opt collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		if opt.OrgName != "orang-gaboets" {
			t.Fatalf("unexpected organization %q", opt.OrgName)
		}
		return actual, nil
	}
	buildSyncFromLiveBootstrap = func(opt syncfromlive.BootstrapOptions) (gitopsconfig.OrganizationConfig, error) {
		if opt.Actual != actual {
			t.Fatalf("unexpected actual state pointer")
		}
		return cfg, nil
	}
	validateSyncFromLiveConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		if got.Organization != cfg.Organization {
			t.Fatalf("unexpected config %#v", got)
		}
		return gitopsconfig.ValidationReport{Valid: true}
	}
	encodeSyncFromLiveConfig = func(got gitopsconfig.OrganizationConfig) ([]byte, error) {
		if got.Organization != cfg.Organization {
			t.Fatalf("unexpected config %#v", got)
		}
		return []byte("organization: orang-gaboets\ninvites: []\nrepositories: []\nteams: []\n"), nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "bootstrap",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if got := out.String(); got != "organization: orang-gaboets\ninvites: []\nrepositories: []\nteams: []\n" {
		t.Fatalf("unexpected YAML output: %q", got)
	}
}

func TestSyncFromLiveConfigCmdWriteSuccess(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := filepath.Join(t.TempDir(), "config")
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	cfg := gitopsconfig.OrganizationConfig{Organization: "orang-gaboets", Invites: []gitopsconfig.InviteSpec{}, Repositories: []gitopsconfig.RepositorySpec{}, Teams: []gitopsconfig.TeamSpec{}}

	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return actual, nil
	}
	buildSyncFromLiveBootstrap = func(syncfromlive.BootstrapOptions) (gitopsconfig.OrganizationConfig, error) {
		return cfg, nil
	}
	validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	encodeSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) ([]byte, error) {
		return []byte("organization: orang-gaboets\ninvites: []\nrepositories: []\nteams: []\n"), nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "bootstrap",
		"--org", "orang-gaboets",
		"--config-dir", configDir,
		"--token", "secret-token",
		"--write",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	envelope := decodeSyncFromLiveEnvelope(t, out.Bytes())
	if envelope.Status != string(cmdoutput.OperationResultStatusSuccess) {
		t.Fatalf("unexpected status: got %q want %q", envelope.Status, cmdoutput.OperationResultStatusSuccess)
	}
	var result syncFromLiveWriteResult
	decodeSyncFromLiveData(t, envelope.Data, &result)
	if result.Organization != "orang-gaboets" || result.Mode != syncFromLiveModeBootstrap {
		t.Fatalf("unexpected write result %#v", result)
	}

	writtenPath := filepath.Join(configDir, syncFromLiveOrganizationFile)
	if result.Path != writtenPath {
		t.Fatalf("unexpected written path %#v", result.Path)
	}
	content, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if string(content) != "organization: orang-gaboets\ninvites: []\nrepositories: []\nteams: []\n" {
		t.Fatalf("unexpected written config:\n%s", string(content))
	}
	if mode := os.FileMode(syncFromLiveConfigFileMode); fileMode(t, writtenPath) != mode {
		t.Fatalf("unexpected written mode: got %o want %o", fileMode(t, writtenPath), mode)
	}
}

func TestSyncFromLiveConfigCmdUnsupportedModeReturnsBeforeAuth(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newSyncFromLiveClient should not be called for unsupported mode")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "reconcile",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `mode "reconcile" is not supported`) {
		t.Fatalf("expected unsupported mode error, got %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestSyncFromLiveConfigCmdPrintsMaterializedYAML(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	existing := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{{Owner: "orang-gaboets", Name: "octostate"}},
		Teams:        []gitopsconfig.TeamSpec{},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	materialized := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{{
			Owner: "orang-gaboets",
			Name:  "octostate",
		}},
		Teams: []gitopsconfig.TeamSpec{},
	}
	materialized.Repositories[0].SetManagedHomepage("https://example.com/octostate")

	loadSyncFromLiveConfig = func(configDir string) (gitopsconfig.OrganizationConfig, error) {
		if configDir != "./config" {
			t.Fatalf("unexpected configDir %q", configDir)
		}
		return existing, nil
	}
	newSyncFromLiveClient = func(_ context.Context, token string, appID, installationID int64, appKeyPath string) (internalauth.Client, error) {
		if token != "secret-token" || appID != 0 || installationID != 0 || appKeyPath != "" {
			t.Fatalf("unexpected auth args token=%q appID=%d installationID=%d appKeyPath=%q", token, appID, installationID, appKeyPath)
		}
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("collectSyncFromLiveState should not be called for materialize mode")
		return nil, nil
	}
	collectSyncFromLiveMaterializeState = func(_ context.Context, opt collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		if opt.OrgName != "orang-gaboets" {
			t.Fatalf("unexpected organization %q", opt.OrgName)
		}
		return actual, nil
	}
	buildSyncFromLiveMaterialize = func(opt syncfromlive.MaterializeOptions) (gitopsconfig.OrganizationConfig, error) {
		if opt.Actual != actual {
			t.Fatal("unexpected actual state pointer")
		}
		if opt.Desired.Organization != existing.Organization || len(opt.Desired.Repositories) != 1 {
			t.Fatalf("unexpected desired config %#v", opt.Desired)
		}
		return materialized, nil
	}
	validateCalls := 0
	validateSyncFromLiveConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		validateCalls++
		switch validateCalls {
		case 1:
			if got.Organization != existing.Organization || len(got.Repositories) != 1 {
				t.Fatalf("unexpected existing config %#v", got)
			}
		case 2:
			homepage, managed := got.Repositories[0].ManagedHomepage()
			if got.Organization != materialized.Organization || !managed || homepage != "https://example.com/octostate" {
				t.Fatalf("unexpected materialized config %#v", got)
			}
		default:
			t.Fatalf("unexpected validate call %d with %#v", validateCalls, got)
		}
		return gitopsconfig.ValidationReport{Valid: true}
	}
	encodeSyncFromLiveConfig = func(got gitopsconfig.OrganizationConfig) ([]byte, error) {
		homepage, managed := got.Repositories[0].ManagedHomepage()
		if got.Organization != materialized.Organization || !managed || homepage != "https://example.com/octostate" {
			t.Fatalf("unexpected config %#v", got)
		}
		return []byte("organization: orang-gaboets\nmembers:\n  - username: alice\n    role: member\ninvites: []\nrepositories:\n  - owner: orang-gaboets\n    name: octostate\n    homepage: https://example.com/octostate\nteams: []\n"), nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "materialize",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if got := out.String(); got != "organization: orang-gaboets\nmembers:\n  - username: alice\n    role: member\ninvites: []\nrepositories:\n  - owner: orang-gaboets\n    name: octostate\n    homepage: https://example.com/octostate\nteams: []\n" {
		t.Fatalf("unexpected YAML output: %q", got)
	}
}

func TestSyncFromLiveConfigCmdMaterializeWriteSuccess(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	existingPath := filepath.Join(configDir, syncFromLiveOrganizationFile)
	if err := os.WriteFile(existingPath, []byte("organization: orang-gaboets\nmembers: []\ninvites: []\nrepositories:\n  - owner: orang-gaboets\n    name: octostate\nteams: []\n"), 0o644); err != nil {
		t.Fatalf("seed existing config: %v", err)
	}

	existing := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{{Owner: "orang-gaboets", Name: "octostate"}},
		Teams:        []gitopsconfig.TeamSpec{},
	}
	materialized := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{{Owner: "orang-gaboets", Name: "octostate"}},
		Teams:        []gitopsconfig.TeamSpec{},
	}
	materialized.Repositories[0].SetManagedHomepage("https://example.com/octostate")

	loadSyncFromLiveConfig = func(gotConfigDir string) (gitopsconfig.OrganizationConfig, error) {
		if gotConfigDir != configDir {
			t.Fatalf("unexpected configDir %q", gotConfigDir)
		}
		return existing, nil
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("collectSyncFromLiveState should not be called for materialize mode")
		return nil, nil
	}
	collectSyncFromLiveMaterializeState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return &state.OrganizationState{Organization: "orang-gaboets"}, nil
	}
	buildSyncFromLiveMaterialize = func(syncfromlive.MaterializeOptions) (gitopsconfig.OrganizationConfig, error) {
		return materialized, nil
	}
	validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	encodeSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) ([]byte, error) {
		return []byte("organization: orang-gaboets\nmembers: []\ninvites: []\nrepositories:\n  - owner: orang-gaboets\n    name: octostate\n    homepage: https://example.com/octostate\nteams: []\n"), nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "materialize",
		"--org", "orang-gaboets",
		"--config-dir", configDir,
		"--token", "secret-token",
		"--write",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	envelope := decodeSyncFromLiveEnvelope(t, out.Bytes())
	if envelope.Status != string(cmdoutput.OperationResultStatusSuccess) {
		t.Fatalf("unexpected status: got %q want %q", envelope.Status, cmdoutput.OperationResultStatusSuccess)
	}
	var result syncFromLiveWriteResult
	decodeSyncFromLiveData(t, envelope.Data, &result)
	if result.Organization != "orang-gaboets" || result.Mode != syncFromLiveModeMaterialize {
		t.Fatalf("unexpected write result %#v", result)
	}

	writtenPath := filepath.Join(configDir, syncFromLiveOrganizationFile)
	if result.Path != writtenPath {
		t.Fatalf("unexpected written path %#v", result.Path)
	}
	content, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if string(content) != "organization: orang-gaboets\nmembers: []\ninvites: []\nrepositories:\n  - owner: orang-gaboets\n    name: octostate\n    homepage: https://example.com/octostate\nteams: []\n" {
		t.Fatalf("unexpected written config:\n%s", string(content))
	}
	if mode := os.FileMode(syncFromLiveConfigFileMode); fileMode(t, writtenPath) != mode {
		t.Fatalf("unexpected written mode: got %o want %o", fileMode(t, writtenPath), mode)
	}
}

func TestSyncFromLiveConfigCmdPrintsAdoptedYAML(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	existing := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{},
		Teams:        []gitopsconfig.TeamSpec{},
	}
	actual := &state.OrganizationState{Organization: "orang-gaboets"}
	adopted := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{},
		Teams:        []gitopsconfig.TeamSpec{},
	}

	loadSyncFromLiveConfig = func(configDir string) (gitopsconfig.OrganizationConfig, error) {
		if configDir != "./config" {
			t.Fatalf("unexpected configDir %q", configDir)
		}
		return existing, nil
	}
	newSyncFromLiveClient = func(_ context.Context, token string, appID, installationID int64, appKeyPath string) (internalauth.Client, error) {
		if token != "secret-token" || appID != 0 || installationID != 0 || appKeyPath != "" {
			t.Fatalf("unexpected auth args token=%q appID=%d installationID=%d appKeyPath=%q", token, appID, installationID, appKeyPath)
		}
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(_ context.Context, opt collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		if opt.OrgName != "orang-gaboets" {
			t.Fatalf("unexpected organization %q", opt.OrgName)
		}
		return actual, nil
	}
	buildSyncFromLiveAdopt = func(opt syncfromlive.AdoptOptions) (gitopsconfig.OrganizationConfig, error) {
		if opt.Actual != actual {
			t.Fatal("unexpected actual state pointer")
		}
		if opt.Desired.Organization != existing.Organization {
			t.Fatalf("unexpected desired config %#v", opt.Desired)
		}
		return adopted, nil
	}
	validateCalls := 0
	validateSyncFromLiveConfig = func(got gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		validateCalls++
		switch validateCalls {
		case 1:
			if got.Organization != existing.Organization {
				t.Fatalf("unexpected existing config %#v", got)
			}
		case 2:
			if got.Organization != adopted.Organization || len(got.Members) != 1 {
				t.Fatalf("unexpected adopted config %#v", got)
			}
		default:
			t.Fatalf("unexpected validate call %d with %#v", validateCalls, got)
		}
		return gitopsconfig.ValidationReport{Valid: true}
	}
	encodeSyncFromLiveConfig = func(got gitopsconfig.OrganizationConfig) ([]byte, error) {
		if got.Organization != adopted.Organization || len(got.Members) != 1 {
			t.Fatalf("unexpected config %#v", got)
		}
		return []byte("organization: orang-gaboets\nmembers:\n  - username: alice\n    role: member\ninvites: []\nrepositories: []\nteams: []\n"), nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "adopt",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if got := out.String(); got != "organization: orang-gaboets\nmembers:\n  - username: alice\n    role: member\ninvites: []\nrepositories: []\nteams: []\n" {
		t.Fatalf("unexpected YAML output: %q", got)
	}
}

func TestSyncFromLiveConfigCmdAdoptWriteSuccess(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("create config dir: %v", err)
	}
	existingPath := filepath.Join(configDir, syncFromLiveOrganizationFile)
	if err := os.WriteFile(existingPath, []byte("organization: orang-gaboets\nmembers: []\ninvites: []\nrepositories: []\nteams: []\n"), 0o644); err != nil {
		t.Fatalf("seed existing config: %v", err)
	}

	existing := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{},
		Teams:        []gitopsconfig.TeamSpec{},
	}
	adopted := gitopsconfig.OrganizationConfig{
		Organization: "orang-gaboets",
		Members:      []gitopsconfig.OrganizationMemberSpec{{Username: "alice", Role: "member"}},
		Invites:      []gitopsconfig.InviteSpec{},
		Repositories: []gitopsconfig.RepositorySpec{},
		Teams:        []gitopsconfig.TeamSpec{},
	}

	loadSyncFromLiveConfig = func(gotConfigDir string) (gitopsconfig.OrganizationConfig, error) {
		if gotConfigDir != configDir {
			t.Fatalf("unexpected configDir %q", gotConfigDir)
		}
		return existing, nil
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return &state.OrganizationState{Organization: "orang-gaboets"}, nil
	}
	buildSyncFromLiveAdopt = func(syncfromlive.AdoptOptions) (gitopsconfig.OrganizationConfig, error) {
		return adopted, nil
	}
	validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	encodeSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) ([]byte, error) {
		return []byte("organization: orang-gaboets\nmembers:\n  - username: alice\n    role: member\ninvites: []\nrepositories: []\nteams: []\n"), nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "adopt",
		"--org", "orang-gaboets",
		"--config-dir", configDir,
		"--token", "secret-token",
		"--write",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}

	envelope := decodeSyncFromLiveEnvelope(t, out.Bytes())
	if envelope.Status != string(cmdoutput.OperationResultStatusSuccess) {
		t.Fatalf("unexpected status: got %q want %q", envelope.Status, cmdoutput.OperationResultStatusSuccess)
	}
	var result syncFromLiveWriteResult
	decodeSyncFromLiveData(t, envelope.Data, &result)
	if result.Organization != "orang-gaboets" || result.Mode != syncFromLiveModeAdopt {
		t.Fatalf("unexpected write result %#v", result)
	}

	writtenPath := filepath.Join(configDir, syncFromLiveOrganizationFile)
	if result.Path != writtenPath {
		t.Fatalf("unexpected written path %#v", result.Path)
	}
	content, err := os.ReadFile(writtenPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	if string(content) != "organization: orang-gaboets\nmembers:\n  - username: alice\n    role: member\ninvites: []\nrepositories: []\nteams: []\n" {
		t.Fatalf("unexpected written config:\n%s", string(content))
	}
	if mode := os.FileMode(syncFromLiveConfigFileMode); fileMode(t, writtenPath) != mode {
		t.Fatalf("unexpected written mode: got %o want %o", fileMode(t, writtenPath), mode)
	}
}

func TestSyncFromLiveConfigCmdAdoptExistingConfigInvalidFailsBeforeAuth(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	loadSyncFromLiveConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "orang-gaboets",
			Members:      []gitopsconfig.OrganizationMemberSpec{{Username: "alice", Role: ""}},
		}, nil
	}
	validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{
			Valid: false,
			Errors: []gitopsconfig.ValidationIssue{{
				Path:    "members[0].role",
				Code:    gitopsconfig.ValidationIssueCodeMissingRequiredField,
				Message: "organization member role is required",
			}},
		}
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newSyncFromLiveClient should not be called when existing adopt config is invalid")
		return nil, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("collectSyncFromLiveState should not be called when existing adopt config is invalid")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "adopt",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "existing config is invalid") {
		t.Fatalf("expected existing config invalid error, got %v", err)
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: existing config is invalid") {
		t.Fatalf("expected validation error on stderr, got %q", got)
	}
	if !strings.Contains(errBuf.String(), "members[0].role: organization member role is required") {
		t.Fatalf("expected detailed validation stderr output, got %q", errBuf.String())
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}

func TestSyncFromLiveConfigCmdAdoptOrganizationMismatchFailsBeforeAuth(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	loadSyncFromLiveConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "other-org",
			Members:      []gitopsconfig.OrganizationMemberSpec{},
			Invites:      []gitopsconfig.InviteSpec{},
			Repositories: []gitopsconfig.RepositorySpec{},
			Teams:        []gitopsconfig.TeamSpec{},
		}, nil
	}
	validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newSyncFromLiveClient should not be called when adopt config organization mismatches --org")
		return nil, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("collectSyncFromLiveState should not be called when adopt config organization mismatches --org")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "adopt",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `organization "other-org" in organization.yaml does not match --org "orang-gaboets"`) {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
	if _, ok := exitcode.Code(err); ok {
		t.Fatalf("expected plain error, got typed exit error %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestSyncFromLiveConfigCmdAdoptMissingConfigFailsBeforeAuth(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	loadSyncFromLiveConfig = func(configDir string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{}, &gitopsconfig.LoadError{
			Kind: gitopsconfig.LoadErrorMissingFile,
			Path: filepath.Join(configDir, "organization.yaml"),
			Err:  os.ErrNotExist,
		}
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newSyncFromLiveClient should not be called when adopt config file is missing")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "adopt",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("expected load failure error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: failed to load config") {
		t.Fatalf("expected load failure on stderr, got %q", got)
	}
}

func TestSyncFromLiveConfigCmdAdoptGeneratedInvalidConfigFails(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	loadSyncFromLiveConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "orang-gaboets",
			Members:      []gitopsconfig.OrganizationMemberSpec{},
			Invites:      []gitopsconfig.InviteSpec{},
			Repositories: []gitopsconfig.RepositorySpec{},
			Teams:        []gitopsconfig.TeamSpec{},
		}, nil
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return &state.OrganizationState{Organization: "orang-gaboets"}, nil
	}
	buildSyncFromLiveAdopt = func(syncfromlive.AdoptOptions) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "orang-gaboets",
			Members: []gitopsconfig.OrganizationMemberSpec{
				{Username: "alice", Role: ""},
			},
		}, nil
	}
	validateCalls := 0
	validateSyncFromLiveConfig = func(_ gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		validateCalls++
		if validateCalls == 1 {
			return gitopsconfig.ValidationReport{Valid: true}
		}
		return gitopsconfig.ValidationReport{
			Valid: false,
			Errors: []gitopsconfig.ValidationIssue{{
				Path:    "members[0].role",
				Code:    gitopsconfig.ValidationIssueCodeMissingRequiredField,
				Message: "organization member role is required",
			}},
		}
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "adopt",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "generated adopt config is invalid") {
		t.Fatalf("expected generated adopt config invalid error, got %v", err)
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: generated adopt config is invalid") {
		t.Fatalf("expected validation error on stderr, got %q", got)
	}
	if !strings.Contains(errBuf.String(), "members[0].role: organization member role is required") {
		t.Fatalf("expected detailed validation stderr output, got %q", errBuf.String())
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}

func TestSyncFromLiveConfigCmdMaterializeExistingConfigInvalidFailsBeforeAuth(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	loadSyncFromLiveConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []gitopsconfig.RepositorySpec{{Owner: "", Name: "octostate"}},
		}, nil
	}
	validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{
			Valid: false,
			Errors: []gitopsconfig.ValidationIssue{{
				Path:    "repositories[0].owner",
				Code:    gitopsconfig.ValidationIssueCodeMissingRequiredField,
				Message: "repository owner is required",
			}},
		}
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newSyncFromLiveClient should not be called when existing materialize config is invalid")
		return nil, nil
	}
	collectSyncFromLiveMaterializeState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("collectSyncFromLiveMaterializeState should not be called when existing materialize config is invalid")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "materialize",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "existing config is invalid") {
		t.Fatalf("expected existing config invalid error, got %v", err)
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: existing config is invalid") {
		t.Fatalf("expected validation error on stderr, got %q", got)
	}
	if !strings.Contains(errBuf.String(), "repositories[0].owner: repository owner is required") {
		t.Fatalf("expected detailed validation stderr output, got %q", errBuf.String())
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}

func TestSyncFromLiveConfigCmdMaterializeOrganizationMismatchFailsBeforeAuth(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	loadSyncFromLiveConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "other-org",
			Members:      []gitopsconfig.OrganizationMemberSpec{},
			Invites:      []gitopsconfig.InviteSpec{},
			Repositories: []gitopsconfig.RepositorySpec{},
			Teams:        []gitopsconfig.TeamSpec{},
		}, nil
	}
	validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{Valid: true}
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newSyncFromLiveClient should not be called when materialize config organization mismatches --org")
		return nil, nil
	}
	collectSyncFromLiveMaterializeState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("collectSyncFromLiveMaterializeState should not be called when materialize config organization mismatches --org")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "materialize",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `organization "other-org" in organization.yaml does not match --org "orang-gaboets"`) {
		t.Fatalf("expected organization mismatch error, got %v", err)
	}
	if _, ok := exitcode.Code(err); ok {
		t.Fatalf("expected plain error, got typed exit error %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestSyncFromLiveConfigCmdMaterializeMissingConfigFailsBeforeAuth(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	loadSyncFromLiveConfig = func(configDir string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{}, &gitopsconfig.LoadError{
			Kind: gitopsconfig.LoadErrorMissingFile,
			Path: filepath.Join(configDir, "organization.yaml"),
			Err:  os.ErrNotExist,
		}
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newSyncFromLiveClient should not be called when materialize config file is missing")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "materialize",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if !strings.Contains(err.Error(), "failed to load config") {
		t.Fatalf("expected load failure error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: failed to load config") {
		t.Fatalf("expected load failure on stderr, got %q", got)
	}
}

func TestSyncFromLiveConfigCmdMaterializeGeneratedInvalidConfigFails(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	loadSyncFromLiveConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "orang-gaboets",
			Members:      []gitopsconfig.OrganizationMemberSpec{},
			Invites:      []gitopsconfig.InviteSpec{},
			Repositories: []gitopsconfig.RepositorySpec{{Owner: "orang-gaboets", Name: "octostate"}},
			Teams:        []gitopsconfig.TeamSpec{},
		}, nil
	}
	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("collectSyncFromLiveState should not be called for materialize mode")
		return nil, nil
	}
	collectSyncFromLiveMaterializeState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return &state.OrganizationState{Organization: "orang-gaboets"}, nil
	}
	buildSyncFromLiveMaterialize = func(syncfromlive.MaterializeOptions) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "orang-gaboets",
			Repositories: []gitopsconfig.RepositorySpec{{Owner: "", Name: "octostate"}},
		}, nil
	}
	validateCalls := 0
	validateSyncFromLiveConfig = func(_ gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		validateCalls++
		if validateCalls == 1 {
			return gitopsconfig.ValidationReport{Valid: true}
		}
		return gitopsconfig.ValidationReport{
			Valid: false,
			Errors: []gitopsconfig.ValidationIssue{{
				Path:    "repositories[0].owner",
				Code:    gitopsconfig.ValidationIssueCodeMissingRequiredField,
				Message: "repository owner is required",
			}},
		}
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "materialize",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "generated materialize config is invalid") {
		t.Fatalf("expected generated materialize config invalid error, got %v", err)
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: generated materialize config is invalid") {
		t.Fatalf("expected validation error on stderr, got %q", got)
	}
	if !strings.Contains(errBuf.String(), "repositories[0].owner: repository owner is required") {
		t.Fatalf("expected detailed validation stderr output, got %q", errBuf.String())
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
}

func TestSyncFromLiveConfigCmdGeneratedInvalidConfigFails(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		return internalauth.MockClient{}, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return &state.OrganizationState{Organization: "orang-gaboets"}, nil
	}
	buildSyncFromLiveBootstrap = func(syncfromlive.BootstrapOptions) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
		return gitopsconfig.ValidationReport{
			Valid: false,
			Errors: []gitopsconfig.ValidationIssue{
				{
					Path:    "repositories[0].visibility",
					Code:    gitopsconfig.ValidationIssueCodeMissingRequiredField,
					Message: "repository visibility is required",
				},
				{
					Path:    "repositories[0].name",
					Code:    gitopsconfig.ValidationIssueCodeMissingRequiredField,
					Message: "repository name is required",
				},
			},
		}
	}
	encodeSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) ([]byte, error) {
		t.Fatal("encodeSyncFromLiveConfig should not be called for invalid generated config")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "bootstrap",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "generated bootstrap config is invalid") {
		t.Fatalf("expected generated invalid config error, got %v", err)
	}
	if code, ok := exitcode.Code(err); !ok || code != validateExitCodeInvalidConfig {
		t.Fatalf("expected typed exit code %d, got code=%d ok=%v err=%v", validateExitCodeInvalidConfig, code, ok, err)
	}
	if !strings.Contains(err.Error(), "repositories[0].visibility: repository visibility is required") {
		t.Fatalf("expected validation details in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "and 1 more error(s)") {
		t.Fatalf("expected remaining error count in error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output, got %q", out.String())
	}
	if got := errBuf.String(); !strings.Contains(got, "Error: generated bootstrap config is invalid") {
		t.Fatalf("expected validation error on stderr, got %q", got)
	}
	if !strings.Contains(errBuf.String(), "repositories[0].visibility: repository visibility is required") {
		t.Fatalf("expected detailed validation stderr output, got %q", errBuf.String())
	}
}

func TestSyncFromLiveConfigCmdWriteFailsWhenTargetExists(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := t.TempDir()
	path := filepath.Join(configDir, syncFromLiveOrganizationFile)
	if err := os.WriteFile(path, []byte("existing\n"), 0o600); err != nil {
		t.Fatalf("write existing bootstrap config: %v", err)
	}

	newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
		t.Fatal("newSyncFromLiveClient should not be called when bootstrap target already exists")
		return nil, nil
	}
	collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		t.Fatal("collectSyncFromLiveState should not be called when bootstrap target already exists")
		return nil, nil
	}

	cmd := SyncFromLiveConfigCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--mode", "bootstrap",
		"--org", "orang-gaboets",
		"--config-dir", configDir,
		"--token", "secret-token",
		"--write",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected target exists error, got %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
	}
}

func TestWriteBootstrapConfigFileLinkFailureRemovesTempFile(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := t.TempDir()
	var tempPath string
	var removedPath string

	createTempSyncFromLiveFile = func(dir, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		tempPath = file.Name()
		return file, nil
	}
	linkSyncFromLivePath = func(_, _ string) error {
		return errors.New("link failed")
	}
	removeSyncFromLivePath = func(path string) error {
		removedPath = path
		return os.Remove(path)
	}

	_, err := writeBootstrapConfigFile(configDir, []byte("organization: orang-gaboets\n"))
	if err == nil || !strings.Contains(err.Error(), "link bootstrap config") {
		t.Fatalf("expected link error, got %v", err)
	}
	if tempPath == "" {
		t.Fatal("expected temp file path to be captured")
	}
	if removedPath != tempPath {
		t.Fatalf("expected temp file removal, removed=%q temp=%q", removedPath, tempPath)
	}
	if _, statErr := os.Stat(tempPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected temp file to be removed, got stat error %v", statErr)
	}
}

func TestWriteBootstrapConfigFileLinkExistingTargetRemovesTempFile(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := t.TempDir()
	var tempPath string
	var removedPath string

	createTempSyncFromLiveFile = func(dir, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		tempPath = file.Name()
		return file, nil
	}
	linkSyncFromLivePath = func(_, _ string) error {
		return os.ErrExist
	}
	removeSyncFromLivePath = func(path string) error {
		removedPath = path
		return os.Remove(path)
	}

	_, err := writeBootstrapConfigFile(configDir, []byte("organization: orang-gaboets\n"))
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected target exists error, got %v", err)
	}
	if tempPath == "" {
		t.Fatal("expected temp file path to be captured")
	}
	if removedPath != tempPath {
		t.Fatalf("expected temp file removal, removed=%q temp=%q", removedPath, tempPath)
	}
	if _, statErr := os.Stat(tempPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected temp file to be removed, got stat error %v", statErr)
	}
}

func TestWriteBootstrapConfigFileRemoveAfterSuccessfulLinkIsBestEffort(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := t.TempDir()
	removeErr := errors.New("remove failed")

	linkSyncFromLivePath = func(oldname, newname string) error {
		return os.Link(oldname, newname)
	}
	removeSyncFromLivePath = func(path string) error {
		if strings.HasPrefix(filepath.Base(path), "organization-") {
			return removeErr
		}
		return os.Remove(path)
	}

	targetPath, err := writeBootstrapConfigFile(configDir, []byte("organization: orang-gaboets\n"))
	if err != nil {
		t.Fatalf("expected successful write despite temp cleanup failure, got %v", err)
	}
	if targetPath != filepath.Join(configDir, syncFromLiveOrganizationFile) {
		t.Fatalf("unexpected target path %q", targetPath)
	}
	content, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read target config: %v", err)
	}
	if string(content) != "organization: orang-gaboets\n" {
		t.Fatalf("unexpected target config content %q", string(content))
	}
}

func TestWriteBootstrapConfigFileChmodFailureRemovesTempFile(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := t.TempDir()
	var tempPath string
	var removedPath string

	createTempSyncFromLiveFile = func(dir, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		tempPath = file.Name()
		return file, nil
	}
	chmodSyncFromLivePath = func(_ string, mode os.FileMode) error {
		if mode != syncFromLiveConfigFileMode {
			t.Fatalf("unexpected chmod mode %o", mode)
		}
		return errors.New("chmod failed")
	}
	removeSyncFromLivePath = func(path string) error {
		removedPath = path
		return os.Remove(path)
	}

	_, err := writeBootstrapConfigFile(configDir, []byte("organization: orang-gaboets\n"))
	if err == nil || !strings.Contains(err.Error(), "set bootstrap config file mode") {
		t.Fatalf("expected chmod error, got %v", err)
	}
	if tempPath == "" {
		t.Fatal("expected temp file path to be captured")
	}
	if removedPath != tempPath {
		t.Fatalf("expected temp file removal, removed=%q temp=%q", removedPath, tempPath)
	}
	if _, statErr := os.Stat(tempPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected temp file to be removed, got stat error %v", statErr)
	}
}

func TestReplaceSyncFromLiveConfigFileRenameFailureRemovesTempFile(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := t.TempDir()
	var tempPath string
	var removedPath string

	createTempSyncFromLiveFile = func(dir, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		tempPath = file.Name()
		return file, nil
	}
	renameSyncFromLivePath = func(_, _ string) error {
		return errors.New("rename failed")
	}
	removeSyncFromLivePath = func(path string) error {
		removedPath = path
		return os.Remove(path)
	}

	_, err := replaceSyncFromLiveConfigFile(configDir, []byte("organization: orang-gaboets\n"))
	if err == nil || !strings.Contains(err.Error(), "replace sync-from-live config") {
		t.Fatalf("expected rename error, got %v", err)
	}
	if tempPath == "" {
		t.Fatal("expected temp file path to be captured")
	}
	if removedPath != tempPath {
		t.Fatalf("expected temp file removal, removed=%q temp=%q", removedPath, tempPath)
	}
	if _, statErr := os.Stat(tempPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected temp file to be removed, got stat error %v", statErr)
	}
}

func TestReplaceSyncFromLiveConfigFileChmodFailureRemovesTempFile(t *testing.T) {
	restoreSyncFromLiveHooks(t)

	configDir := t.TempDir()
	var tempPath string
	var removedPath string

	createTempSyncFromLiveFile = func(dir, pattern string) (*os.File, error) {
		file, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		tempPath = file.Name()
		return file, nil
	}
	chmodSyncFromLivePath = func(_ string, mode os.FileMode) error {
		if mode != syncFromLiveConfigFileMode {
			t.Fatalf("unexpected chmod mode %o", mode)
		}
		return errors.New("chmod failed")
	}
	removeSyncFromLivePath = func(path string) error {
		removedPath = path
		return os.Remove(path)
	}

	_, err := replaceSyncFromLiveConfigFile(configDir, []byte("organization: orang-gaboets\n"))
	if err == nil || !strings.Contains(err.Error(), "set sync-from-live config file mode") {
		t.Fatalf("expected chmod error, got %v", err)
	}
	if tempPath == "" {
		t.Fatal("expected temp file path to be captured")
	}
	if removedPath != tempPath {
		t.Fatalf("expected temp file removal, removed=%q temp=%q", removedPath, tempPath)
	}
	if _, statErr := os.Stat(tempPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("expected temp file to be removed, got stat error %v", statErr)
	}
}

func TestSyncFromLiveConfigCmdAuthCollectBuildEncodeFailuresPropagate(t *testing.T) {
	authErr := errors.New("auth failed")
	collectErr := errors.New("collect failed")
	buildErr := errors.New("build failed")
	encodeErr := errors.New("encode failed")

	tests := []struct {
		name    string
		arrange func()
		wantErr error
	}{
		{
			name:    "auth failure",
			wantErr: authErr,
			arrange: func() {
				newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
					return nil, authErr
				}
			},
		},
		{
			name:    "collector failure",
			wantErr: collectErr,
			arrange: func() {
				newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
					return internalauth.MockClient{}, nil
				}
				collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
					return nil, collectErr
				}
			},
		},
		{
			name:    "builder failure",
			wantErr: buildErr,
			arrange: func() {
				newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
					return internalauth.MockClient{}, nil
				}
				collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
					return &state.OrganizationState{Organization: "orang-gaboets"}, nil
				}
				buildSyncFromLiveBootstrap = func(syncfromlive.BootstrapOptions) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{}, buildErr
				}
			},
		},
		{
			name:    "encode failure",
			wantErr: encodeErr,
			arrange: func() {
				newSyncFromLiveClient = func(context.Context, string, int64, int64, string) (internalauth.Client, error) {
					return internalauth.MockClient{}, nil
				}
				collectSyncFromLiveState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
					return &state.OrganizationState{Organization: "orang-gaboets"}, nil
				}
				buildSyncFromLiveBootstrap = func(syncfromlive.BootstrapOptions) (gitopsconfig.OrganizationConfig, error) {
					return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
				}
				validateSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) gitopsconfig.ValidationReport {
					return gitopsconfig.ValidationReport{Valid: true}
				}
				encodeSyncFromLiveConfig = func(gitopsconfig.OrganizationConfig) ([]byte, error) {
					return nil, encodeErr
				}
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			restoreSyncFromLiveHooks(t)
			tt.arrange()

			cmd := SyncFromLiveConfigCmd()
			var out bytes.Buffer
			var errBuf bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errBuf)
			cmd.SetArgs([]string{
				"--mode", "bootstrap",
				"--org", "orang-gaboets",
				"--config-dir", "./config",
				"--token", "secret-token",
			})

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
			case "builder failure":
				if !strings.Contains(err.Error(), "failed to build generated config: build failed") {
					t.Fatalf("unexpected error message: %v", err)
				}
			case "encode failure":
				if !strings.Contains(err.Error(), "failed to encode generated config: encode failed") {
					t.Fatalf("unexpected error message: %v", err)
				}
			}
			if out.Len() != 0 || errBuf.Len() != 0 {
				t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
			}
		})
	}
}

func restoreSyncFromLiveHooks(t *testing.T) {
	t.Helper()

	oldNewClient := newSyncFromLiveClient
	oldCollect := collectSyncFromLiveState
	oldCollectMaterialize := collectSyncFromLiveMaterializeState
	oldLoad := loadSyncFromLiveConfig
	oldBuild := buildSyncFromLiveBootstrap
	oldBuildAdopt := buildSyncFromLiveAdopt
	oldBuildMaterialize := buildSyncFromLiveMaterialize
	oldValidate := validateSyncFromLiveConfig
	oldEncode := encodeSyncFromLiveConfig
	oldStat := statSyncFromLivePath
	oldMkdirAll := mkdirAllSyncFromLiveConfigDir
	oldCreateTemp := createTempSyncFromLiveFile
	oldChmod := chmodSyncFromLivePath
	oldLink := linkSyncFromLivePath
	oldRename := renameSyncFromLivePath
	oldRemove := removeSyncFromLivePath

	t.Cleanup(func() {
		newSyncFromLiveClient = oldNewClient
		collectSyncFromLiveState = oldCollect
		collectSyncFromLiveMaterializeState = oldCollectMaterialize
		loadSyncFromLiveConfig = oldLoad
		buildSyncFromLiveBootstrap = oldBuild
		buildSyncFromLiveAdopt = oldBuildAdopt
		buildSyncFromLiveMaterialize = oldBuildMaterialize
		validateSyncFromLiveConfig = oldValidate
		encodeSyncFromLiveConfig = oldEncode
		statSyncFromLivePath = oldStat
		mkdirAllSyncFromLiveConfigDir = oldMkdirAll
		createTempSyncFromLiveFile = oldCreateTemp
		chmodSyncFromLivePath = oldChmod
		linkSyncFromLivePath = oldLink
		renameSyncFromLivePath = oldRename
		removeSyncFromLivePath = oldRemove
	})
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

type syncFromLiveEnvelope struct {
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func decodeSyncFromLiveEnvelope(t *testing.T, payload []byte) syncFromLiveEnvelope {
	t.Helper()

	var envelope syncFromLiveEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		t.Fatalf("decode sync-from-live envelope: %v; payload=%q", err, string(payload))
	}
	return envelope
}

func decodeSyncFromLiveData(t *testing.T, payload json.RawMessage, out any) {
	t.Helper()

	if err := json.Unmarshal(payload, out); err != nil {
		t.Fatalf("decode sync-from-live data: %v; payload=%q", err, string(payload))
	}
}
