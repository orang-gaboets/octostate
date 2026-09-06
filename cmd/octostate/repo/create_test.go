package repo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	reposcmd "github.com/orang-gaboets/octostate/cmd/octostate/repo"
	"github.com/orang-gaboets/octostate/pkg/github"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type configOperationData struct {
	Owner              string   `json:"owner"`
	Name               string   `json:"name"`
	ConfigPath         string   `json:"config_path"`
	Changed            bool     `json:"changed"`
	ChangedFields      []string `json:"changed_fields"`
	TemplateOwner      string   `json:"template_owner"`
	TemplateRepo       string   `json:"template_repo"`
	Private            bool     `json:"private"`
	IncludeAllBranches bool     `json:"include_all_branches"`
	Topics             []string `json:"topics"`
}

type configOperationResult struct {
	Status  string              `json:"status"`
	Message string              `json:"message"`
	Data    configOperationData `json:"data"`
}

func decodeConfigOperationOutput(t *testing.T, output string) configOperationResult {
	t.Helper()
	var result configOperationResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode config operation output: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("expected success status, got %q", result.Status)
	}
	return result
}

type captureCreateRepoFromTemplateService struct {
	lastTemplateRepo string
	lastTemplateOrg  string
	lastRequest      *gh.TemplateRepoRequest
	lastCreateOwner  string
	lastCreateRepo   *gh.Repository
	lastTopics       []string
	createCalled     bool
	ordinaryCalled   bool
}

func TestCreateRepoCmdHelpDescribesOptionalTemplateCreation(t *testing.T) {
	cmd := reposcmd.CreateRepoCmd(nil)
	if cmd.Short != "Create a GitHub repository, optionally from a template" {
		t.Fatalf("unexpected short help: %q", cmd.Short)
	}
	if !strings.Contains(cmd.Long, "optionally from a template repository") {
		t.Fatalf("expected general repository help to describe optional templates: %q", cmd.Long)
	}
	if !strings.Contains(cmd.Example, "octostate repo create --org <org> --name <new-repo-name>") || !strings.Contains(cmd.Example, "--template-name <template-name>") {
		t.Fatalf("expected general repository examples for ordinary and template creation: %q", cmd.Example)
	}
}

func TestCreateRepoCmdSupportsOrdinaryCreation(t *testing.T) {
	service := &captureCreateRepoFromTemplateService{}
	cmd := reposcmd.CreateRepoCmd(service)
	cmd.SetArgs([]string{"--org", "org", "--name", "service", "--desc", "description", "--topics", "go,cli", "--private"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ordinary create returned error: %v", err)
	}
	if !service.ordinaryCalled || service.createCalled {
		t.Fatalf("ordinary create used the wrong service path: %#v", service)
	}
	if service.lastCreateOwner != "org" || service.lastCreateRepo.GetName() != "service" || service.lastCreateRepo.GetDescription() != "description" || !service.lastCreateRepo.GetPrivate() {
		t.Fatalf("unexpected ordinary create request: owner=%q repo=%#v", service.lastCreateOwner, service.lastCreateRepo)
	}
	if got, want := strings.Join(service.lastTopics, ","), "go,cli"; got != want {
		t.Fatalf("unexpected ordinary create topics: got %q want %q", got, want)
	}
}

func TestCreateRepoCmdSupportsInternalVisibility(t *testing.T) {
	service := &captureCreateRepoFromTemplateService{}
	cmd := reposcmd.CreateRepoCmd(service)
	cmd.SetArgs([]string{"--org", "org", "--name", "service", "--visibility", "internal"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("internal create returned error: %v", err)
	}
	if service.lastCreateRepo.GetVisibility() != "internal" || service.lastCreateRepo.Private != nil {
		t.Fatalf("unexpected internal create request: %#v", service.lastCreateRepo)
	}
}

func TestCreateRepoCmdRejectsPrivateAndVisibilityTogether(t *testing.T) {
	service := &captureCreateRepoFromTemplateService{}
	cmd := reposcmd.CreateRepoCmd(service)
	cmd.SetArgs([]string{"--org", "org", "--name", "service", "--private", "--visibility", "private"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("expected visibility conflict, got %v", err)
	}
	if service.ordinaryCalled || service.createCalled {
		t.Fatal("visibility conflict reached a create service")
	}
}

func TestCreateRepoCmdRejectsInternalTemplateDryRun(t *testing.T) {
	cmd := reposcmd.CreateNewRepoFromTemplateCmd(&captureCreateRepoFromTemplateService{})
	cmd.SetArgs([]string{"--org", "org", "--template-name", "base", "--name", "service", "--visibility", "internal", "--dry-run"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported for template-based") {
		t.Fatalf("expected unsupported template visibility error, got %v", err)
	}
}

func TestCreateRepoCmdRejectsInternalTemplateLiveBeforeAuth(t *testing.T) {
	cmd := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	cmd.SetArgs([]string{"--org", "org", "--template-name", "base", "--name", "service", "--visibility", "internal"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unsupported for template-based") {
		t.Fatalf("expected unsupported template visibility error, got %v", err)
	}
}

func TestCreateRepoCmdToConfigAllowsInternalTemplate(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte("organization: org\nrepositories: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := reposcmd.CreateRepoCmd(nil)
	cmd.SetArgs([]string{"--org", "org", "--template-name", "base", "--name", "service", "--visibility", "internal", "--to-config", configPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("internal template proposal returned error: %v", err)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Repositories) != 1 || cfg.Repositories[0].Visibility != "internal" || cfg.Repositories[0].Template.Name != "base" {
		t.Fatalf("unexpected internal template proposal: %#v", cfg.Repositories)
	}
}

func TestCreateRepoCmdDryRunSkipsOrdinaryCreation(t *testing.T) {
	service := &captureCreateRepoFromTemplateService{}
	cmd := reposcmd.CreateRepoCmd(service)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--org", "org", "--name", "service", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ordinary dry-run returned error: %v", err)
	}
	if service.ordinaryCalled || service.createCalled {
		t.Fatalf("ordinary dry-run called a create service: %#v", service)
	}
	if !strings.Contains(out.String(), `"status": "dry-run"`) {
		t.Fatalf("expected dry-run output, got %q", out.String())
	}
}

func TestCreateRepoCmdWithTemplateUsesTemplateCreation(t *testing.T) {
	service := &captureCreateRepoFromTemplateService{}
	cmd := reposcmd.CreateRepoCmd(service)
	cmd.SetArgs([]string{"--org", "org", "--template-org", "templates", "--template-name", "base", "--name", "service", "--include-all-branches"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("template create returned error: %v", err)
	}
	if !service.createCalled || service.ordinaryCalled {
		t.Fatalf("template create used the wrong service path: %#v", service)
	}
	if service.lastTemplateOrg != "templates" || service.lastTemplateRepo != "base" || service.lastRequest == nil || !service.lastRequest.GetIncludeAllBranches() {
		t.Fatalf("unexpected template create request: org=%q repo=%q request=%#v", service.lastTemplateOrg, service.lastTemplateRepo, service.lastRequest)
	}
}

func TestCreateRepoCmdRejectsExplicitBlankTemplateName(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
	}{
		{
			name: "live",
			args: []string{"--org", "org", "--name", "service", "--template-name", "   "},
		},
		{
			name: "dry-run",
			args: []string{"--org", "org", "--name", "service", "--template-name", "   ", "--dry-run"},
		},
		{
			name: "to-config",
			args: []string{"--org", "org", "--name", "service", "--template-name", "   "},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var service *captureCreateRepoFromTemplateService
			if test.name == "live" || test.name == "dry-run" {
				service = &captureCreateRepoFromTemplateService{}
			}
			cmd := reposcmd.CreateRepoCmd(service)
			args := append([]string(nil), test.args...)
			var before []byte
			var configPath string
			if test.name == "to-config" {
				configPath = filepath.Join(t.TempDir(), "organization.yaml")
				before = []byte("organization: org\nrepositories: []\n")
				if err := os.WriteFile(configPath, before, 0o600); err != nil {
					t.Fatal(err)
				}
				args = append(args, "--to-config", configPath)
			}
			cmd.SetArgs(args)

			err := cmd.Execute()
			if err == nil || err.Error() != "--template-name must not be empty" {
				t.Fatalf("expected blank template-name error, got %v", err)
			}
			if service != nil && (service.ordinaryCalled || service.createCalled) {
				t.Fatalf("blank template name reached a create service: %#v", service)
			}
			if configPath != "" {
				after, readErr := os.ReadFile(configPath)
				if readErr != nil {
					t.Fatal(readErr)
				}
				if !bytes.Equal(after, before) {
					t.Fatalf("config changed after blank template-name rejection:\n%s", after)
				}
			}
		})
	}
}

func TestCreateRepoCmdToConfigOmitsTemplate(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte("organization: org\nrepositories: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cmd := reposcmd.CreateRepoCmd(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--org", "org", "--name", "service", "--desc", "description", "--private", "--to-config", configPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ordinary proposal returned error: %v", err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	if result.Data.TemplateOwner != "" || result.Data.TemplateRepo != "" || result.Data.IncludeAllBranches {
		t.Fatalf("ordinary proposal unexpectedly contains template data: %#v", result.Data)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Repositories) != 1 {
		t.Fatalf("expected one repository, got %#v", cfg.Repositories)
	}
	repository := cfg.Repositories[0]
	if repository.Template != (gitopsconfig.TemplateSpec{}) {
		t.Fatalf("ordinary proposal unexpectedly wrote template settings: %#v", repository.Template)
	}
	if repository.Visibility != "private" || repository.Description != "description" {
		t.Fatalf("unexpected ordinary proposal repository: %#v", repository)
	}
}

func (m *captureCreateRepoFromTemplateService) Create(_ context.Context, owner string, repository *gh.Repository) (*gh.Repository, *gh.Response, error) {
	m.ordinaryCalled = true
	m.lastCreateOwner = owner
	m.lastCreateRepo = repository
	return &gh.Repository{}, nil, nil
}

func (m *captureCreateRepoFromTemplateService) CreateFromTemplate(_ context.Context, templateOwner, templateRepo string, req *gh.TemplateRepoRequest) (*gh.Repository, *gh.Response, error) {
	m.lastTemplateOrg = templateOwner
	m.lastTemplateRepo = templateRepo
	m.lastRequest = req
	m.createCalled = true
	return &gh.Repository{}, nil, nil
}

func (*captureCreateRepoFromTemplateService) Delete(_ context.Context, _, _ string) (*gh.Response, error) {
	return nil, nil
}

func (*captureCreateRepoFromTemplateService) Edit(_ context.Context, _, _ string, _ *gh.Repository) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

func (*captureCreateRepoFromTemplateService) Get(_ context.Context, _, _ string) (*gh.Repository, *gh.Response, error) {
	return &gh.Repository{}, nil, nil
}

func (*captureCreateRepoFromTemplateService) ListByOrg(_ context.Context, _ string, _ *gh.RepositoryListByOrgOptions) ([]*gh.Repository, *gh.Response, error) {
	return nil, nil, nil
}

func (m *captureCreateRepoFromTemplateService) ReplaceAllTopics(_ context.Context, _, _ string, topics []string) ([]string, *gh.Response, error) {
	m.lastTopics = append([]string(nil), topics...)
	return topics, nil, nil
}

func (*captureCreateRepoFromTemplateService) ListAllTopics(_ context.Context, _, _ string) ([]string, *gh.Response, error) {
	return nil, nil, nil
}

func TestCreateRepoFromTemplateNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestCreateRepoFromTemplateMissingTemplateNameFlag(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--name", "n"})
	err := c.Execute()
	if err == nil {
		t.Fatalf("expected error for missing template-name flag")
	}
	if !strings.Contains(err.Error(), "template-name") {
		t.Fatalf("expected error mentioning template-name, got %v", err)
	}
}

func TestCreateRepoFromTemplateRejectsExplicitBlankTemplateName(t *testing.T) {
	service := &captureCreateRepoFromTemplateService{}
	c := reposcmd.CreateNewRepoFromTemplateCmd(service)
	c.SetArgs([]string{"--org", "o", "--template-name", "   ", "--name", "n"})
	err := c.Execute()
	if err == nil || err.Error() != "--template-name must not be empty" {
		t.Fatalf("expected blank template-name error, got %v", err)
	}
	if service.createCalled || service.ordinaryCalled {
		t.Fatalf("blank template name reached a create service: %#v", service)
	}
}

func TestCreateRepoFromTemplateAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Created repository o/n from template o/temp") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestCreateRepoFromTemplateAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateRepoFromTemplatePartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestCreateRepoFromTemplateBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key", "--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestCreateRepoFromTemplateWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--template-name", "temp", "--name", "n", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestCreateRepoFromTemplateIncludeAllBranchesDefaultsFalse(t *testing.T) {
	svc := &captureCreateRepoFromTemplateService{}
	c := reposcmd.CreateNewRepoFromTemplateCmd(svc)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastRequest == nil || svc.lastRequest.IncludeAllBranches == nil {
		t.Fatalf("expected CreateFromTemplate request with IncludeAllBranches set")
	}
	if *svc.lastRequest.IncludeAllBranches {
		t.Fatalf("expected IncludeAllBranches default false, got true")
	}
}

func TestCreateRepoFromTemplateIncludeAllBranchesCanBeEnabled(t *testing.T) {
	svc := &captureCreateRepoFromTemplateService{}
	c := reposcmd.CreateNewRepoFromTemplateCmd(svc)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n", "--include-all-branches=true"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastRequest == nil || svc.lastRequest.IncludeAllBranches == nil {
		t.Fatalf("expected CreateFromTemplate request with IncludeAllBranches set")
	}
	if !*svc.lastRequest.IncludeAllBranches {
		t.Fatalf("expected IncludeAllBranches true when explicitly set")
	}
}

func TestCreateRepoFromTemplateDryRunSkipsCreateService(t *testing.T) {
	svc := &captureCreateRepoFromTemplateService{}
	c := reposcmd.CreateNewRepoFromTemplateCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n", "--topics", "a,b", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.createCalled {
		t.Fatalf("expected create service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would create repository o/n from template o/temp") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestCreateRepoFromTemplateToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte("organization: o\nrepositories: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	c.SetOut(&out)
	c.SetErr(&errBuf)
	c.SetArgs([]string{
		"--org", "o",
		"--template-name", "temp",
		"--name", "n",
		"--template-org", "template-org",
		"--desc", "description",
		"--topics", " a,b,a ",
		"--private=true",
		"--include-all-branches=true",
		"--to-config", configPath,
	})

	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	result := decodeConfigOperationOutput(t, out.String())
	data := result.Data
	if result.Message != "Proposed repository o/n in config" {
		t.Fatalf("unexpected config operation message: %q", result.Message)
	}
	if errBuf.Len() != 0 {
		t.Fatalf("expected no stderr output, got %q", errBuf.String())
	}
	if data.Owner != "o" || data.Name != "n" || data.ConfigPath != configPath || !data.Changed {
		t.Fatalf("unexpected config operation data: %#v", data)
	}
	if data.TemplateOwner != "template-org" || data.TemplateRepo != "temp" || !data.Private || !data.IncludeAllBranches {
		t.Fatalf("unexpected template output data: %#v", data)
	}
	if got, want := strings.Join(data.Topics, ","), "a,b,a"; got != want {
		t.Fatalf("unexpected output topics: got %q want %q", got, want)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Repositories) != 1 {
		t.Fatalf("expected one repository, got %#v", cfg.Repositories)
	}
	repository := cfg.Repositories[0]
	if repository.Owner != "o" || repository.Name != "n" || repository.Visibility != "private" {
		t.Fatalf("unexpected repository identity/settings: %#v", repository)
	}
	if repository.Template.Owner != "template-org" || repository.Template.Name != "temp" || !repository.Template.IncludeAllBranches {
		t.Fatalf("unexpected template settings: %#v", repository.Template)
	}
	if repository.Description != "description" || !repository.DescriptionOption().Present {
		t.Fatalf("unexpected description: %#v", repository)
	}
	if got, want := strings.Join(repository.Topics, ","), "a,b,a"; got != want {
		t.Fatalf("unexpected topics: got %q want %q", got, want)
	}
}

func TestCreateRepoFromTemplateExplicitEmptyToConfigDoesNotUseGitHub(t *testing.T) {
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "empty", path: ""},
		{name: "whitespace", path: " "},
	} {
		t.Run(test.name, func(t *testing.T) {
			c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
			c.SetArgs([]string{
				"--org", "o",
				"--template-name", "temp",
				"--name", "n",
				"--to-config", test.path,
			})
			err := c.Execute()
			if err == nil {
				t.Fatal("expected invalid config path error")
			}
			if errors.Is(err, github.ErrNoValidCredentials) {
				t.Fatalf("explicit config mode attempted GitHub authentication: %v", err)
			}
		})
	}
}

func TestCreateRepoFromTemplateToConfigDefaultsTemplateOwnerAndPublicVisibility(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(configPath, []byte("organization: o\nrepositories: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n", "--to-config", configPath})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}

	cfg, err := gitopsconfig.LoadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	repository := cfg.Repositories[0]
	if repository.Template.Owner != "o" {
		t.Fatalf("expected template owner o, got %q", repository.Template.Owner)
	}
	if repository.Visibility != "public" {
		t.Fatalf("expected public visibility, got %q", repository.Visibility)
	}
	if option := repository.DescriptionOption(); !option.Present || option.Null {
		t.Fatalf("expected explicitly managed empty description: %#v", option)
	}
	if repository.Description != "" {
		t.Fatalf("expected empty description, got %q", repository.Description)
	}
}

func TestCreateRepoFromTemplateToConfigRejectsEmptyTopicBeforeLoadingConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n", "--topics", "a,,b", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "topic cannot be empty") {
		t.Fatalf("expected empty-topic error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}

func TestCreateRepoFromTemplateToConfigRejectsDuplicate(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "organization.yaml")
	before := "organization: o\nrepositories:\n  - name: n\n    visibility: public\n"
	if err := os.WriteFile(configPath, []byte(before), 0o600); err != nil {
		t.Fatal(err)
	}

	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--org", "O", "--template-name", "temp", "--name", "N", "--to-config", configPath})
	err := c.Execute()
	if err == nil || !strings.Contains(err.Error(), "already exists in config") {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	got, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != before {
		t.Fatalf("config changed after duplicate rejection:\n%s", got)
	}
}

func TestCreateRepoFromTemplateRejectsDryRunWithToConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "missing.yaml")
	c := reposcmd.CreateNewRepoFromTemplateCmd(nil)
	c.SetArgs([]string{"--org", "o", "--template-name", "temp", "--name", "n", "--dry-run", "--to-config", configPath})
	err := c.Execute()
	if err == nil || err.Error() != "--to-config cannot be combined with --dry-run" {
		t.Fatalf("expected conflicting-flag error, got %v", err)
	}
	if _, err := os.Stat(configPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected config to remain absent, got %v", err)
	}
}
