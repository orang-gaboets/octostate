package members_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	memberscmd "github.com/orang-gaboets/octostate/cmd/octostate/team/members"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureListTeamMembersBySlugService struct {
	auth.MockTeamsService
	listCalled bool
	org        string
	slug       string
	role       string
}

func (s *captureListTeamMembersBySlugService) ListTeamMembersBySlug(_ context.Context, org, slug string, opts *gh.TeamListTeamMembersOptions) ([]*gh.User, *gh.Response, error) {
	s.listCalled = true
	s.org = org
	s.slug = slug
	if opts != nil {
		s.role = opts.Role
	}
	return []*gh.User{
		{
			ID:   github.Ptr(int64(321)),
			Name: github.Ptr("captured-member"),
		},
	}, &gh.Response{NextPage: 0}, nil
}

func TestListTeamMembersNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestListTeamMembersAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTeamMembersAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTeamMembersPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--slug", "s"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestListTeamMembersBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--slug", "s"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestListTeamMembersWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flag")
	}
}

func TestListTeamMembersWithMissingSlug(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing slug flag")
	}
}

func TestListTeamMembersWithInvalidRole(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--slug", "s", "--role", "invalid"})
	if err := c.Execute(); !errors.Is(err, github.ErrInvalidFieldValue) {
		t.Fatalf("expected error %v, got %v", github.ErrInvalidFieldValue, err)
	}
}

func TestListTeamMembersWritesJSONToStdout(t *testing.T) {
	auth.PrepareClient(t)
	c := memberscmd.ListCmd(nil)
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

func TestListTeamMembersUsesProvidedServiceAndRole(t *testing.T) {
	svc := &captureListTeamMembersBySlugService{}
	c := memberscmd.ListCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", " o ", "--slug", " s ", "--role", "maintainer"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.listCalled {
		t.Fatalf("expected list team members service to be called")
	}
	if svc.org != "o" || svc.slug != "s" {
		t.Fatalf("expected trimmed target o/s, got %q/%q", svc.org, svc.slug)
	}
	if svc.role != "maintainer" {
		t.Fatalf("expected role %q, got %q", "maintainer", svc.role)
	}
	if got := strings.TrimSpace(out.String()); !strings.Contains(got, "captured-member") {
		t.Fatalf("expected JSON output to contain member name, got %q", got)
	}
}
