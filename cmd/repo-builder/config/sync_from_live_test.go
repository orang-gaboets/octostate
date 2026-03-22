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

	internalauth "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/syncfromlive"
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
		"--mode", "adopt",
		"--org", "orang-gaboets",
		"--config-dir", "./config",
		"--token", "secret-token",
	})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `mode "adopt" is not supported`) {
		t.Fatalf("expected unsupported mode error, got %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
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
	if !strings.Contains(err.Error(), "repositories[0].visibility: repository visibility is required") {
		t.Fatalf("expected validation details in error, got %v", err)
	}
	if !strings.Contains(err.Error(), "and 1 more error(s)") {
		t.Fatalf("expected remaining error count in error, got %v", err)
	}
	if out.Len() != 0 || errBuf.Len() != 0 {
		t.Fatalf("expected no output, got stdout=%q stderr=%q", out.String(), errBuf.String())
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

func TestWriteBootstrapConfigFileRenameFailureRemovesTempFile(t *testing.T) {
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

	_, err := writeBootstrapConfigFile(configDir, []byte("organization: orang-gaboets\n"))
	if err == nil || !strings.Contains(err.Error(), "replace bootstrap config") {
		t.Fatalf("expected replace error, got %v", err)
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
	oldBuild := buildSyncFromLiveBootstrap
	oldValidate := validateSyncFromLiveConfig
	oldEncode := encodeSyncFromLiveConfig
	oldStat := statSyncFromLivePath
	oldMkdirAll := mkdirAllSyncFromLiveConfigDir
	oldCreateTemp := createTempSyncFromLiveFile
	oldRename := renameSyncFromLivePath
	oldRemove := removeSyncFromLivePath

	t.Cleanup(func() {
		newSyncFromLiveClient = oldNewClient
		collectSyncFromLiveState = oldCollect
		buildSyncFromLiveBootstrap = oldBuild
		validateSyncFromLiveConfig = oldValidate
		encodeSyncFromLiveConfig = oldEncode
		statSyncFromLivePath = oldStat
		mkdirAllSyncFromLiveConfigDir = oldMkdirAll
		createTempSyncFromLiveFile = oldCreateTemp
		renameSyncFromLivePath = oldRename
		removeSyncFromLivePath = oldRemove
	})
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
