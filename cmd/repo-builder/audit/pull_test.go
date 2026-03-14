package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	cmdoutput "github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/output"
	"github.com/orang-gaboets/repo-builder/pkg/github"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/collector"
	gitopsconfig "github.com/orang-gaboets/repo-builder/pkg/gitops/config"
	gitopssnapshot "github.com/orang-gaboets/repo-builder/pkg/gitops/snapshot"
	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

type userServiceStub struct {
	getFunc     func(context.Context, string) (*gh.User, *gh.Response, error)
	getByIDFunc func(context.Context, int64) (*gh.User, *gh.Response, error)
}

func (s userServiceStub) Get(ctx context.Context, username string) (*gh.User, *gh.Response, error) {
	if s.getFunc != nil {
		return s.getFunc(ctx, username)
	}
	return &gh.User{}, &gh.Response{}, nil
}

func (s userServiceStub) GetByID(ctx context.Context, id int64) (*gh.User, *gh.Response, error) {
	if s.getByIDFunc != nil {
		return s.getByIDFunc(ctx, id)
	}
	return &gh.User{}, &gh.Response{}, nil
}

func TestPullCmdSuccess(t *testing.T) {
	fixedTime := time.Date(2026, 3, 10, 9, 30, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	expectedPath := "/tmp/state/actual/snapshot.json"

	restore := replaceAuditHooks(t)
	nowAuditSnapshotTime = func() time.Time { return fixedTime }
	newAuditClient = func(_ context.Context, token string, appID, installationID int64, appKeyPath string) (auth.Client, error) {
		if token != "token-value" {
			t.Fatalf("unexpected token: got %q want %q", token, "token-value")
		}
		if appID != 0 || installationID != 0 || appKeyPath != "" {
			t.Fatalf("unexpected app auth values: appID=%d installationID=%d appKeyPath=%q", appID, installationID, appKeyPath)
		}
		return auth.MockClient{
			OrganizationsService: auth.MockOrganizationService{},
			ReposService:         auth.MockRepoService{},
			TeamsService:         auth.MockTeamsService{},
		}, nil
	}
	loadAuditConfig = func(configDir string) (gitopsconfig.OrganizationConfig, error) {
		if configDir != "/control/config" {
			t.Fatalf("unexpected config dir: got %q want %q", configDir, "/control/config")
		}
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	collectAuditState = func(_ context.Context, opt collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		if opt.OrgName != "orang-gaboets" {
			t.Fatalf("unexpected org name: got %q want %q", opt.OrgName, "orang-gaboets")
		}
		return &state.OrganizationState{
			Organization: "orang-gaboets",
			Repositories: []state.Repository{{Owner: "orang-gaboets", Name: "repo-builder"}},
		}, nil
	}
	writeActualSnapshot = func(stateDir string, snapshot gitopssnapshot.ActualSnapshot) (string, error) {
		if stateDir != "/tmp/state" {
			t.Fatalf("unexpected state dir: got %q want %q", stateDir, "/tmp/state")
		}
		if snapshot.Organization != "orang-gaboets" {
			t.Fatalf("unexpected snapshot organization: got %q", snapshot.Organization)
		}
		if !snapshot.PulledAt.Equal(fixedTime.UTC()) {
			t.Fatalf("unexpected pulled time: got %v want %v", snapshot.PulledAt, fixedTime.UTC())
		}
		if len(snapshot.Repositories) != 1 || snapshot.Repositories[0].Name != "repo-builder" {
			t.Fatalf("unexpected snapshot repositories: %#v", snapshot.Repositories)
		}
		return expectedPath, nil
	}
	defer restore()

	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--token", "token-value",
		"--config-dir", "/control/config",
		"--state-dir", "/tmp/state",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result cmdoutput.OperationResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("decode JSON output: %v; payload=%q", err, out.String())
	}
	if result.Status != cmdoutput.OperationResultStatusSuccess {
		t.Fatalf("unexpected status: got %q want %q", result.Status, cmdoutput.OperationResultStatusSuccess)
	}
	if result.Message != "wrote actual-state snapshot" {
		t.Fatalf("unexpected message: got %q", result.Message)
	}

	payload, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("unexpected data payload type: %#v", result.Data)
	}
	if payload["path"] != expectedPath {
		t.Fatalf("unexpected path: got %#v want %q", payload["path"], expectedPath)
	}
	if payload["organization"] != "orang-gaboets" {
		t.Fatalf("unexpected organization: got %#v", payload["organization"])
	}
	if payload["pulled_at"] != fixedTime.UTC().Format(time.RFC3339) {
		t.Fatalf("unexpected pulled_at: got %#v want %q", payload["pulled_at"], fixedTime.UTC().Format(time.RFC3339))
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestPullCmdPersistsResolvedInviteUserIDsForPendingInvitations(t *testing.T) {
	fixedTime := time.Date(2026, 3, 10, 9, 30, 0, 0, time.UTC)

	restore := replaceAuditHooks(t)
	nowAuditSnapshotTime = func() time.Time { return fixedTime }
	newAuditClient = func(_ context.Context, _ string, _, _ int64, _ string) (auth.Client, error) {
		return auth.MockClient{
			OrganizationsService: auth.MockOrganizationService{},
			ReposService:         auth.MockRepoService{},
			TeamsService:         auth.MockTeamsService{},
			UsersService: userServiceStub{
				getFunc: func(_ context.Context, username string) (*gh.User, *gh.Response, error) {
					if username != "octocat" {
						t.Fatalf("unexpected username lookup: %q", username)
					}
					return &gh.User{Login: github.Ptr("octocat"), ID: github.Ptr(int64(99))}, &gh.Response{}, nil
				},
			},
		}, nil
	}
	loadAuditConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{
			Organization: "orang-gaboets",
		}, nil
	}
	collectAuditState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return &state.OrganizationState{
			Organization: "orang-gaboets",
			PendingInvitations: []state.PendingInvitation{
				{Username: "octocat"},
			},
		}, nil
	}
	writeActualSnapshot = func(_ string, snapshot gitopssnapshot.ActualSnapshot) (string, error) {
		if !reflect.DeepEqual(snapshot.ResolvedInviteUserIDsByUsername, map[string]int64{"octocat": 99}) {
			t.Fatalf("unexpected resolved invite user IDs: %#v", snapshot.ResolvedInviteUserIDsByUsername)
		}
		return "/tmp/state/actual/snapshot.json", nil
	}
	defer restore()

	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{
		"--token", "token-value",
		"--config-dir", "/control/config",
		"--state-dir", "/tmp/state",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
}

func TestResolveInviteUserIDsByUsernameRejectsNilUser(t *testing.T) {
	t.Parallel()

	resolved, err := resolveInviteUserIDsByUsername(context.Background(), userServiceStub{
		getFunc: func(context.Context, string) (*gh.User, *gh.Response, error) {
			return nil, &gh.Response{}, nil
		},
	}, []state.PendingInvitation{
		{Username: "octocat"},
	})
	if err == nil {
		t.Fatal("expected error for nil user")
	}
	if !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("unexpected error: got %v want %v", err, github.ErrInvalidFieldValue)
	}
	if !strings.Contains(err.Error(), "missing user") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if resolved != nil {
		t.Fatalf("expected nil resolved map on error, got %#v", resolved)
	}
}

func TestPullCmdPropagatesAuthError(t *testing.T) {
	wantErr := errors.New("auth failed")

	restore := replaceAuditHooks(t)
	loadAuditConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	newAuditClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
		return nil, wantErr
	}
	defer restore()

	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--token", "token-value", "--config-dir", "/control/config", "--state-dir", "/tmp/state"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output on auth error, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output on auth error, got %q", errBuf.String())
	}
}

func TestPullCmdPropagatesCollectorError(t *testing.T) {
	wantErr := errors.New("collector failed")

	restore := replaceAuditHooks(t)
	newAuditClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
		return auth.MockClient{
			OrganizationsService: auth.MockOrganizationService{},
			ReposService:         auth.MockRepoService{},
			TeamsService:         auth.MockTeamsService{},
		}, nil
	}
	loadAuditConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	collectAuditState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return nil, wantErr
	}
	defer restore()

	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--token", "token-value", "--config-dir", "/control/config", "--state-dir", "/tmp/state"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output on collector error, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output on collector error, got %q", errBuf.String())
	}
}

func TestPullCmdPropagatesLoadError(t *testing.T) {
	wantErr := errors.New("load config failed")

	restore := replaceAuditHooks(t)
	newAuditClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
		t.Fatal("expected auth client construction to be skipped on config load error")
		return nil, nil
	}
	loadAuditConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{}, wantErr
	}
	defer restore()

	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--token", "token-value", "--config-dir", "/control/config", "--state-dir", "/tmp/state"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output on load error, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output on load error, got %q", errBuf.String())
	}
}

func TestPullCmdRejectsBlankOrganizationBeforeAuth(t *testing.T) {
	restore := replaceAuditHooks(t)
	loadAuditConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "   "}, nil
	}
	newAuditClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
		t.Fatal("expected auth client construction to be skipped on blank organization")
		return nil, nil
	}
	defer restore()

	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--token", "token-value", "--config-dir", "/control/config", "--state-dir", "/tmp/state"})

	err := cmd.Execute()
	if !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("unexpected error: got %v want %v", err, github.ErrMissingRequiredField)
	}
	if !strings.Contains(err.Error(), "organization is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output on blank organization error, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output on blank organization error, got %q", errBuf.String())
	}
}

func TestPullCmdWrapsSnapshotWriteError(t *testing.T) {
	wantErr := errors.New("snapshot write failed")

	restore := replaceAuditHooks(t)
	newAuditClient = func(context.Context, string, int64, int64, string) (auth.Client, error) {
		return auth.MockClient{
			OrganizationsService: auth.MockOrganizationService{},
			ReposService:         auth.MockRepoService{},
			TeamsService:         auth.MockTeamsService{},
		}, nil
	}
	loadAuditConfig = func(string) (gitopsconfig.OrganizationConfig, error) {
		return gitopsconfig.OrganizationConfig{Organization: "orang-gaboets"}, nil
	}
	collectAuditState = func(context.Context, collector.CollectOrganizationOptions) (*state.OrganizationState, error) {
		return &state.OrganizationState{Organization: "orang-gaboets"}, nil
	}
	writeActualSnapshot = func(string, gitopssnapshot.ActualSnapshot) (string, error) {
		return "", wantErr
	}
	defer restore()

	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--token", "token-value", "--config-dir", "/control/config", "--state-dir", "/tmp/state"})

	err := cmd.Execute()
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "write actual-state snapshot") {
		t.Fatalf("expected wrapped snapshot write error, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output on snapshot write error, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output on snapshot write error, got %q", errBuf.String())
	}
}

func replaceAuditHooks(t *testing.T) func() {
	t.Helper()

	originalLoadAuditConfig := loadAuditConfig
	originalCollectAuditState := collectAuditState
	originalNewAuditClient := newAuditClient
	originalNewAuditSnapshot := newAuditSnapshot
	originalWriteActualSnapshot := writeActualSnapshot
	originalNowAuditSnapshotTime := nowAuditSnapshotTime
	originalResolveAuditInviteUserIDs := resolveAuditInviteUserIDs

	return func() {
		loadAuditConfig = originalLoadAuditConfig
		collectAuditState = originalCollectAuditState
		newAuditClient = originalNewAuditClient
		newAuditSnapshot = originalNewAuditSnapshot
		writeActualSnapshot = originalWriteActualSnapshot
		nowAuditSnapshotTime = originalNowAuditSnapshotTime
		resolveAuditInviteUserIDs = originalResolveAuditInviteUserIDs
	}
}

func TestPullCmdRequiresFlags(t *testing.T) {
	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--token", "token-value"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing flag error")
	}
	if !strings.Contains(err.Error(), "required flag(s) \"config-dir\"") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output on missing flag error, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output on missing flag error, got %q", errBuf.String())
	}
}

func TestPullCmdRequiresStateDirFlag(t *testing.T) {
	cmd := PullCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--token", "token-value", "--config-dir", "/control/config"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected missing state-dir flag error")
	}
	if !strings.Contains(err.Error(), "required flag(s) \"state-dir\"") {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no stdout output on missing state-dir flag, got %q", out.String())
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output on missing state-dir flag, got %q", errBuf.String())
	}
}
