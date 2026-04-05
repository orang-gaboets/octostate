package permissions_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	permissionscmd "github.com/orang-gaboets/octostate/cmd/octostate/team/repo/permissions"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureAddTeamRepoBySlugService struct {
	auth.MockTeamsService
	addCalled  bool
	org        string
	slug       string
	repoOwner  string
	repoName   string
	permission string
}

func (s *captureAddTeamRepoBySlugService) AddTeamRepoBySlug(_ context.Context, org, slug, owner, repo string, opts *gh.TeamAddTeamRepoOptions) (*gh.Response, error) {
	s.addCalled = true
	s.org = org
	s.slug = slug
	s.repoOwner = owner
	s.repoName = repo
	if opts != nil {
		s.permission = opts.Permission
	}
	return &gh.Response{}, nil
}

func TestAddTeamRepoPermissionNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestAddTeamRepoPermissionAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTeamRepoPermissionAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTeamRepoPermissionPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestAddTeamRepoPermissionBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestAddTeamRepoPermissionWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestAddTeamRepoPermissionWithInvalidPermission(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r", "--permission", "invalid"})
	if err := c.Execute(); !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("expected error %v, got %v", github.ErrInvalidFieldValue, err)
	}
}

func TestAddTeamRepoPermissionWithWhitespaceRepoRejected(t *testing.T) {
	c := permissionscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "   "})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestAddTeamRepoPermissionDryRunSkipsAddService(t *testing.T) {
	svc := &captureAddTeamRepoBySlugService{}
	c := permissionscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "r", "--permission", "push", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.addCalled {
		t.Fatalf("expected add team repo permission service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would grant team o/s permission push on repository o/r") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestAddTeamRepoPermissionWritesSuccessToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.AddCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r", "--permission", "maintain"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Granted team o/s permission maintain on repository o/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestAddTeamRepoPermissionUsesProvidedServiceAndDefaultsRepoOrg(t *testing.T) {
	svc := &captureAddTeamRepoBySlugService{}
	c := permissionscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--repo", " r ", "--permission", "triage"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.addCalled {
		t.Fatalf("expected add team repo permission service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.repoOwner != "o" || svc.repoName != "r" {
		t.Fatalf("expected default repo target o/r, got %q/%q", svc.repoOwner, svc.repoName)
	}
	if svc.permission != "triage" {
		t.Fatalf("expected permission %q, got %q", "triage", svc.permission)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Granted team o/s permission triage on repository o/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestAddTeamRepoPermissionUsesProvidedServiceAndExplicitRepoOrg(t *testing.T) {
	svc := &captureAddTeamRepoBySlugService{}
	c := permissionscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--repo-org", " ro ", "--repo", " r ", "--permission", "admin"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.addCalled {
		t.Fatalf("expected add team repo permission service to be called")
	}
	if svc.repoOwner != "ro" || svc.repoName != "r" {
		t.Fatalf("expected explicit repo target ro/r, got %q/%q", svc.repoOwner, svc.repoName)
	}
	if svc.permission != "admin" {
		t.Fatalf("expected permission %q, got %q", "admin", svc.permission)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Granted team o/s permission admin on repository ro/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}
