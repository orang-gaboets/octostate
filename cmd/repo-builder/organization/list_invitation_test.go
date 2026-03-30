package organization_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	gh "github.com/google/go-github/v55/github"
	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
	organizationcmd "github.com/orang-gaboets/repo-builder/cmd/repo-builder/organization"
	pkggithub "github.com/orang-gaboets/repo-builder/pkg/github"
)

type stubListInvitationService struct {
	auth.MockOrganizationService
	invitations       []*gh.Invitation
	listPendingErr    error
	listPendingCalled bool
	createCalled      bool
}

func (s *stubListInvitationService) ListPendingOrgInvitations(_ context.Context, _ string, _ *gh.ListOptions) ([]*gh.Invitation, *gh.Response, error) {
	s.listPendingCalled = true
	if s.listPendingErr != nil {
		return nil, nil, s.listPendingErr
	}
	return s.invitations, &gh.Response{NextPage: 0}, nil
}

func (s *stubListInvitationService) CreateOrgInvitation(_ context.Context, _ string, _ *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	s.createCalled = true
	return nil, nil, errors.New("unexpected mutating invitation call")
}

func TestListOrgInvitationsCmdNoRequiredFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgInvitationsCmd(nil)
	c.SetArgs([]string{})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing required flags")
	}
}

func TestListOrgInvitationsCmdAllRequiredFlagsTokenProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgInvitationsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOrgInvitationsCmdAllRequiredFlagsAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgInvitationsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListOrgInvitationsCmdPartialAppProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgInvitationsCmd(nil)
	c.SetArgs([]string{"--app-id", "123", "--org", "o"})
	if err := c.Execute(); !errors.Is(err, pkggithub.ErrNoValidCredentials) {
		t.Fatalf("expected error %v, got %v", pkggithub.ErrNoValidCredentials, err)
	}
}

func TestListOrgInvitationsCmdBothAuthMethodsProvided(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgInvitationsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--app-id", "123", "--installation-id", "456", "--app-key-path", "path/to/key.pem", "--org", "o"})
	if err := c.Execute(); !errors.Is(err, pkggithub.ErrConflictingCredentials) {
		t.Fatalf("expected error %v, got %v", pkggithub.ErrConflictingCredentials, err)
	}
}

func TestListOrgInvitationsCmdWithInvalidFlags(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgInvitationsCmd(nil)
	c.SetArgs([]string{"--token", "t", "--org", "o", "--invalid-flag"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for invalid flags")
	}
}

func TestListOrgInvitationsCmdWithMissingOrg(t *testing.T) {
	auth.PrepareClient(t)
	c := organizationcmd.ListOrgInvitationsCmd(nil)
	c.SetArgs([]string{"--token", "t"})
	if err := c.Execute(); err == nil {
		t.Fatalf("expected error for missing org flag")
	}
}

func TestListOrgInvitationsCmdPropagatesServiceError(t *testing.T) {
	wantErr := errors.New("list pending invitations failed")
	svc := &stubListInvitationService{listPendingErr: wantErr}
	c := organizationcmd.ListOrgInvitationsCmd(svc)
	c.SetArgs([]string{"--org", "o"})

	if err := c.Execute(); !errors.Is(err, wantErr) {
		t.Fatalf("expected error %v, got %v", wantErr, err)
	}
	if !svc.listPendingCalled {
		t.Fatal("expected list pending invitations call")
	}
	if svc.createCalled {
		t.Fatal("did not expect mutating invitation call")
	}
}

func TestListOrgInvitationsCmdWritesJSONToStdout(t *testing.T) {
	createdAt := time.Date(2026, time.March, 31, 12, 0, 0, 0, time.UTC)
	svc := &stubListInvitationService{
		invitations: []*gh.Invitation{{
			ID:                pkggithub.Ptr(int64(7)),
			Login:             pkggithub.Ptr("monalisa"),
			Email:             pkggithub.Ptr("monalisa@example.com"),
			Role:              pkggithub.Ptr("direct_member"),
			CreatedAt:         &gh.Timestamp{Time: createdAt},
			TeamCount:         pkggithub.Ptr(2),
			InvitationTeamURL: pkggithub.Ptr("https://api.github.com/organizations/1/invitations/7/teams"),
		}},
	}

	c := organizationcmd.ListOrgInvitationsCmd(svc)
	var out bytes.Buffer
	c.SetOut(&out)
	c.SetArgs([]string{"--org", "o"})

	if err := c.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !svc.listPendingCalled {
		t.Fatal("expected list pending invitations call")
	}
	if svc.createCalled {
		t.Fatal("did not expect mutating invitation call")
	}

	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("expected valid JSON output, got %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 invitation, got %d", len(got))
	}
	if got[0]["ID"] != float64(7) {
		t.Fatalf("expected invitation ID 7, got %#v", got[0]["ID"])
	}
	if got[0]["Login"] != "monalisa" {
		t.Fatalf("expected login monalisa, got %#v", got[0]["Login"])
	}
	if got[0]["Email"] != "monalisa@example.com" {
		t.Fatalf("expected email monalisa@example.com, got %#v", got[0]["Email"])
	}
	if got[0]["Role"] != "direct_member" {
		t.Fatalf("expected role direct_member, got %#v", got[0]["Role"])
	}
	if got[0]["TeamCount"] != float64(2) {
		t.Fatalf("expected team count 2, got %#v", got[0]["TeamCount"])
	}
	if got[0]["InvitationTeamURL"] != "https://api.github.com/organizations/1/invitations/7/teams" {
		t.Fatalf("expected invitation team URL, got %#v", got[0]["InvitationTeamURL"])
	}
	if got[0]["CreatedAt"] != createdAt.Format(time.RFC3339) {
		t.Fatalf("expected created_at %q, got %#v", createdAt.Format(time.RFC3339), got[0]["CreatedAt"])
	}
}
