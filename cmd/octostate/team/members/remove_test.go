package members_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	memberscmd "github.com/orang-gaboets/octostate/cmd/octostate/team/members"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureRemoveTeamMembershipBySlugService struct {
	auth.MockTeamsService
	removeCalled bool
	org          string
	slug         string
	username     string
}

func (s *captureRemoveTeamMembershipBySlugService) RemoveTeamMembershipBySlug(_ context.Context, org, slug, user string) (*gh.Response, error) {
	s.removeCalled = true
	s.org = org
	s.slug = slug
	s.username = user
	return &gh.Response{}, nil
}

func TestRemoveTeamMemberNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestRemoveTeamMemberAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTeamMemberAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveTeamMemberPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestRemoveTeamMemberBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestRemoveTeamMemberWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestRemoveTeamMemberWithWhitespaceUsernameRejected(t *testing.T) {
	c := memberscmd.RemoveCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "   "})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestRemoveTeamMemberDryRunSkipsRemoveService(t *testing.T) {
	svc := &captureRemoveTeamMembershipBySlugService{}
	c := memberscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "u", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.removeCalled {
		t.Fatalf("expected remove team membership service not to be called in dry-run mode")
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, `Dry run: would remove user "u" from team o/s`) {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestRemoveTeamMemberWritesSuccessToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.RemoveCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, `Removed user "u" from team o/s`) {
		t.Fatalf("unexpected success output: %q", got)
	}
}

func TestRemoveTeamMemberUsesProvidedService(t *testing.T) {
	svc := &captureRemoveTeamMembershipBySlugService{}
	c := memberscmd.RemoveCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--username", " u "})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.removeCalled {
		t.Fatalf("expected remove team membership service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.username != "u" {
		t.Fatalf("expected trimmed username %q, got %q", "u", svc.username)
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, `Removed user "u" from team o/s`) {
		t.Fatalf("unexpected success output: %q", got)
	}
}
