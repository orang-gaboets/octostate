package permissions_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	permissionscmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/team/repo/permissions"
	"github.com/orang-gaboets/repo-builder/pkg/github"
)

type captureRemoveTeamRepoBySlugService struct {
	auth.MockTeamsService
	removeCalled bool
	org          string
	slug         string
	repoOwner    string
	repoName     string
}

func (s *captureRemoveTeamRepoBySlugService) RemoveTeamRepoBySlug(_ context.Context, org, slug, owner, repo string) (*gh.Response, error) {
	s.removeCalled = true
	s.org = org
	s.slug = slug
	s.repoOwner = owner
	s.repoName = repo
	return &gh.Response{}, nil
}

func TestRemoveTeamRepoPermissionNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestRemoveTeamRepoPermissionAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTeamRepoPermissionAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTeamRepoPermissionPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestRemoveTeamRepoPermissionBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestRemoveTeamRepoPermissionWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestRemoveTeamRepoPermissionWithWhitespaceRepoRejected(t *testing.T) {
	c := permissionscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "   "})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestRemoveTeamRepoPermissionDryRunSkipsRemoveService(t *testing.T) {
	svc := &captureRemoveTeamRepoBySlugService{}
	c := permissionscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--repo", "r", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.removeCalled {
		t.Fatalf("expected remove team repo permission service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "Dry run: would remove team o/s access to repository o/r") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestRemoveTeamRepoPermissionWritesSuccessToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.RemoveCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--repo", "r"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Removed team o/s access to repository o/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestRemoveTeamRepoPermissionUsesProvidedServiceAndDefaultsRepoOrg(t *testing.T) {
	svc := &captureRemoveTeamRepoBySlugService{}
	c := permissionscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--repo", " r "})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.removeCalled {
		t.Fatalf("expected remove team repo permission service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.repoOwner != "o" || svc.repoName != "r" {
		t.Fatalf("expected default repo target o/r, got %q/%q", svc.repoOwner, svc.repoName)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Removed team o/s access to repository o/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestRemoveTeamRepoPermissionUsesProvidedServiceAndExplicitRepoOrg(t *testing.T) {
	svc := &captureRemoveTeamRepoBySlugService{}
	c := permissionscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--repo-org", " ro ", "--repo", " r "})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.removeCalled {
		t.Fatalf("expected remove team repo permission service to be called")
	}
	if svc.repoOwner != "ro" || svc.repoName != "r" {
		t.Fatalf("expected explicit repo target ro/r, got %q/%q", svc.repoOwner, svc.repoName)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, "Removed team o/s access to repository ro/r") {
		t.Fatalf("unexpected success output: %q", got)
	}
}
