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

type captureListTeamReposBySlugService struct {
	auth.MockTeamsService
	listCalled bool
	org        string
	slug       string
}

func (s *captureListTeamReposBySlugService) ListTeamReposBySlug(_ context.Context, org, slug string, _ *gh.ListOptions) ([]*gh.Repository, *gh.Response, error) {
	s.listCalled = true
	s.org = org
	s.slug = slug
	return []*gh.Repository{
		{
			Name: github.Ptr("captured-repo"),
			Owner: &gh.User{
				Login: github.Ptr("captured-org"),
			},
			Permissions: map[string]bool{
				"pull":  true,
				"push":  false,
				"admin": false,
			},
		},
	}, &gh.Response{NextPage: 0}, nil
}

func TestListTeamRepoPermissionsNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.ListCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestListTeamRepoPermissionsAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTeamRepoPermissionsAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.ListCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTeamRepoPermissionsPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.ListCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestListTeamRepoPermissionsBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestListTeamRepoPermissionsWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestListTeamRepoPermissionsWithMissingSlug(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing slug flag")
	}
}

func TestListTeamRepoPermissionsWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := permissionscmd.ListCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got == "" {
		t.Fatalf("expected stdout output, got empty string")
	}
	if !strings.HasPrefix(got, "[") {
		t.Fatalf("expected JSON array output, got %q", got)
	}
}

func TestListTeamRepoPermissionsUsesProvidedService(t *testing.T) {
	svc := &captureListTeamReposBySlugService{}
	c := permissionscmd.ListCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s "})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.listCalled {
		t.Fatalf("expected list team repos service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, "captured-repo") {
		t.Fatalf("expected JSON output to contain repo name, got %q", got)
	}
	if !strings.Contains(got, "\"Permissions\"") {
		t.Fatalf("expected JSON output to contain permissions, got %q", got)
	}
}
