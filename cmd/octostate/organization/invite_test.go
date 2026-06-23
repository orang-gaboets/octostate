package organization_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"
	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	organizationcmd "github.com/orang-gaboets/octostate/cmd/octostate/organization"
	"github.com/orang-gaboets/octostate/pkg/github"
)

type captureInviteOrganizationService struct {
	auth.MockOrganizationService
	inviteCalled bool
}

func (s *captureInviteOrganizationService) CreateOrgInvitation(_ context.Context, _ string, _ *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	s.inviteCalled = true
	return &gh.Invitation{}, nil, nil
}

type captureInviteUserLookupService struct {
	auth.MockUserService
	getCalled bool
}

func (s *captureInviteUserLookupService) Get(_ context.Context, _ string) (*gh.User, *gh.Response, error) {
	s.getCalled = true
	return &gh.User{}, nil, nil
}

func TestInviteCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestInviteCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInviteCmdAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInviteCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrNoValidCredentials, err)
	}
}

func TestInviteCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o", "--username", "u"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestInviteCmdWithUsername(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, `"username": "u"`) {
		t.Fatalf("expected username in output data, got: %q", got)
	}
}

func TestInviteCmdWithUserID(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--id", "123"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "success"`) {
		t.Fatalf("expected success status output, got: %q", got)
	}
	if !strings.Contains(got, `"user_id": 123`) {
		t.Fatalf("expected user id in output data, got: %q", got)
	}
}

func TestInviteCmdWithBothUsernameAndUserID(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--username", "u", "--id", "123"})
	if err := c.Execute(); !errors.Is(err, github.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", github.ErrConflictingCredentials, err)
	}
}

func TestInviteCmdWithNonPositiveUserID(t *testing.T) {
	auth.PrepareClient(t)

	tests := []struct {
		name   string
		userID string
	}{
		{name: "zero", userID: "0"},
		{name: "negative", userID: "-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := organizationcmd.InviteCmd(nil, nil)
			c.SetArgs([]string{"--token", "t", "--org", "o", "--id", tc.userID})
			if err := c.Execute(); !errors.Is(err, github.ErrMissingRequiredField) {
				t.Fatalf("expected error %v, got %v", github.ErrMissingRequiredField, err)
			}
		})
	}
}

func TestInviteCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.InviteCmd(nil, nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestInviteCmdDryRunSkipsUserLookupAndOrgInvite(t *testing.T) {
	orgSvc := &captureInviteOrganizationService{}
	userSvc := &captureInviteUserLookupService{}
	c := organizationcmd.InviteCmd(orgSvc, userSvc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o", "--username", "u", "--dry-run"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if orgSvc.inviteCalled {
		t.Fatalf("expected org invite service not to be called in dry-run mode")
	}
	if userSvc.getCalled {
		t.Fatalf("expected user lookup service not to be called in dry-run mode")
	}
	got := strings.TrimSpace(out.String())
	if !strings.Contains(got, `"status": "dry-run"`) {
		t.Fatalf("expected dry-run status output, got: %q", got)
	}
	if !strings.Contains(got, "username lookup skipped") {
		t.Fatalf("unexpected dry-run output: %q", got)
	}
}
