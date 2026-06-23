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

type captureAddTeamMembershipBySlugService struct {
	auth.MockTeamsService
	addCalled bool
	org       string
	slug      string
	username  string
	role      string
}

func (s *captureAddTeamMembershipBySlugService) AddTeamMembershipBySlug(_ context.Context, org, slug, user string, opts *gh.TeamAddTeamMembershipOptions) (*gh.Membership, *gh.Response, error) {
	s.addCalled = true
	s.org = org
	s.slug = slug
	s.username = user
	if opts != nil {
		s.role = opts.Role
	}
	return &gh.Membership{
		State: github.Ptr("active"),
		Role:  github.Ptr(s.role),
	}, &gh.Response{}, nil
}

func TestAddTeamMemberNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestAddTeamMemberAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTeamMemberAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAddTeamMemberPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestAddTeamMemberBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestAddTeamMemberWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestAddTeamMemberWithInvalidRole(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u", "--role", "invalid"})
	if err := c.Execute(); !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("expected error %v, got %v", github.ErrInvalidFieldValue, err)
	}
}

func TestAddTeamMemberWithWhitespaceUsernameRejected(t *testing.T) {
	c := memberscmd.AddCmd(nil)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "   "})
	if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
		t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
	}
}

func TestAddTeamMemberDryRunSkipsAddService(t *testing.T) {
	svc := &captureAddTeamMembershipBySlugService{}
	c := memberscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--slug", "s", "--username", "u", "--role", "maintainer", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.addCalled {
		t.Fatalf("expected add team membership service not to be called in dry-run mode")
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, `Dry run: would add user "u" to team o/s with role maintainer`) {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}

func TestAddTeamMemberWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.AddCmd(nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if got == "" {
		t.Fatalf("expected stdout output, got empty string")
	}
	if !strings.HasPrefix(got, "{") {
		t.Fatalf("expected JSON object output, got %q", got)
	}
}

func TestAddTeamMemberUsesProvidedServiceAndRole(t *testing.T) {
	svc := &captureAddTeamMembershipBySlugService{}
	c := memberscmd.AddCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--username", " u ", "--role", "maintainer"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.addCalled {
		t.Fatalf("expected add team membership service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.username != "u" {
		t.Fatalf("expected trimmed username %q, got %q", "u", svc.username)
	}
	if svc.role != "maintainer" {
		t.Fatalf("expected role %q, got %q", "maintainer", svc.role)
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, `"Role": "maintainer"`) {
		t.Fatalf("expected JSON output to contain membership role, got %q", got)
	}
}
