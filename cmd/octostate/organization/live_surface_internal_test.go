package organization

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
	"github.com/orang-gaboets/octostate/pkg/github/organizations"
	"github.com/orang-gaboets/octostate/pkg/github/teams"
	"github.com/orang-gaboets/octostate/pkg/github/users"
)

// captureInvite records exactly what the CLI hands the invitation service.
type captureInvite struct {
	auth.MockOrganizationService
	calls []*gh.CreateOrgInvitationOptions
}

func (s *captureInvite) CreateOrgInvitation(_ context.Context, _ string, opts *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	s.calls = append(s.calls, opts)
	return &gh.Invitation{}, nil, nil
}

type recordingUserLookup struct {
	auth.MockUserService
	called bool
}

func (s *recordingUserLookup) Get(_ context.Context, _ string) (*gh.User, *gh.Response, error) {
	s.called = true
	return &gh.User{ID: gh.Ptr(int64(7))}, nil, nil
}

// captureTeams resolves slugs to IDs, or fails, so team wiring is testable
// without a live GitHub call.
type captureTeams struct {
	auth.MockTeamsService
	ids map[string]int64
	err error
}

func (s *captureTeams) GetTeamBySlug(_ context.Context, _, slug string) (*gh.Team, *gh.Response, error) {
	if s.err != nil {
		return nil, nil, s.err
	}
	id, ok := s.ids[slug]
	if !ok {
		// Resolves, but without a usable ID.
		return &gh.Team{Slug: gh.Ptr(slug)}, nil, nil
	}
	return &gh.Team{ID: gh.Ptr(id), Slug: gh.Ptr(slug)}, nil, nil
}

// runInvite builds the command through the unexported seam. Each service is
// left as a nil interface unless a mock was supplied, so a nil concrete pointer
// never becomes a non-nil interface that bypasses the production fallback.
func runInvite(org organizations.Service, user users.Service, team teams.Service, args ...string) (string, error) {
	cmd := inviteCmd(org, user, team)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestInviteLiveEmailUsesEmailWithoutUserLookup(t *testing.T) {
	org := &captureInvite{}
	user := &recordingUserLookup{}

	if _, err := runInvite(org, user, nil, "--org", "o", "--email", "alice@example.com", "--token", "t"); err != nil {
		t.Fatal(err)
	}

	if len(org.calls) != 1 {
		t.Fatalf("expected one invitation call, got %d", len(org.calls))
	}
	opts := org.calls[0]
	if opts.GetEmail() != "alice@example.com" {
		t.Fatalf("email = %q", opts.GetEmail())
	}
	if opts.InviteeID != nil {
		t.Fatalf("email identity must not send an invitee ID, got %d", opts.GetInviteeID())
	}
	if user.called {
		t.Fatal("email identity must not perform a username lookup")
	}
}

func TestInviteLiveForwardsRole(t *testing.T) {
	for _, tc := range []struct{ flag, want string }{
		{"admin", "admin"},
		{"billing_manager", "billing_manager"},
		{"", "direct_member"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			org := &captureInvite{}
			args := []string{"--org", "o", "--email", "a@example.com", "--token", "t"}
			if tc.flag != "" {
				args = append(args, "--role", tc.flag)
			}
			if _, err := runInvite(org, nil, nil, args...); err != nil {
				t.Fatal(err)
			}
			if got := org.calls[0].GetRole(); got != tc.want {
				t.Fatalf("role = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestInviteLiveResolvesTeamSlugsInCallerOrder(t *testing.T) {
	org := &captureInvite{}
	team := &captureTeams{ids: map[string]int64{"platform": 11, "backend": 22}}

	if _, err := runInvite(org, nil, team,
		"--org", "o", "--email", "a@example.com", "--token", "t",
		"--team-slug", "platform", "--team-slug", "backend"); err != nil {
		t.Fatal(err)
	}

	got := org.calls[0].TeamID
	if len(got) != 2 || got[0] != 11 || got[1] != 22 {
		t.Fatalf("team IDs = %#v, want [11 22] in caller order", got)
	}
}

func TestInviteLiveTeamResolutionFailurePreventsInvitation(t *testing.T) {
	for name, team := range map[string]*captureTeams{
		"lookup fails": {err: errors.New("boom")},
		"no usable ID": {ids: map[string]int64{}},
	} {
		t.Run(name, func(t *testing.T) {
			org := &captureInvite{}
			_, err := runInvite(org, nil, team, "--org", "o", "--email", "a@example.com", "--token", "t", "--team-slug", "platform")
			if err == nil {
				t.Fatal("unresolvable team must fail the command")
			}
			if len(org.calls) != 0 {
				t.Fatal("no invitation may be sent when a requested team cannot be resolved")
			}
		})
	}
}

// The exact inputs that previously produced a successful dry-run preview of a
// request that could never work.
func TestInviteRejectsUnusableInputBeforeDryRun(t *testing.T) {
	for name, args := range map[string][]string{
		"malformed email": {"--org", "o", "--email", "not-an-email", "--dry-run"},
		"blank username":  {"--org", "o", "--username", " ", "--dry-run"},
		"non-positive id": {"--org", "o", "--id", "0", "--dry-run"},
		"blank team slug": {"--org", "o", "--email", "a@example.com", "--team-slug", " ", "--dry-run"},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := runInvite(nil, nil, nil, args...)
			if err == nil {
				t.Fatalf("expected rejection, got output: %s", out)
			}
			if strings.Contains(out, "dry-run") {
				t.Fatalf("no dry-run preview may be emitted for an unusable request: %s", out)
			}
		})
	}
}

// --- #282 live membership ---

type captureMembership struct {
	auth.MockOrganizationService
	orgs      []string
	usernames []string
	roles     []string
	err       error
}

func (s *captureMembership) EditOrgMembership(_ context.Context, user, org string, membership *gh.Membership) (*gh.Membership, *gh.Response, error) {
	s.orgs = append(s.orgs, org)
	s.usernames = append(s.usernames, user)
	s.roles = append(s.roles, membership.GetRole())
	if s.err != nil {
		return nil, nil, s.err
	}
	return &gh.Membership{}, nil, nil
}

func runMembership(svc organizations.Service, args ...string) (string, error) {
	cmd := MembershipSetCmd(svc)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestMembershipSetLiveForwardsRequest(t *testing.T) {
	svc := &captureMembership{}

	out, err := runMembership(svc, "--org", " acme ", "--username", " alice ", "--role", "admin", "--token", "t")
	if err != nil {
		t.Fatal(err)
	}

	if len(svc.orgs) != 1 {
		t.Fatalf("expected exactly one membership mutation, got %d", len(svc.orgs))
	}
	if svc.orgs[0] != "acme" || svc.usernames[0] != "alice" || svc.roles[0] != "admin" {
		t.Fatalf("forwarded org=%q user=%q role=%q", svc.orgs[0], svc.usernames[0], svc.roles[0])
	}
	// The wording must not claim the user is already an active member.
	if !strings.Contains(out, "Requested organization membership") {
		t.Fatalf("unexpected message: %s", out)
	}
}

func TestMembershipSetLivePropagatesServiceError(t *testing.T) {
	wantErr := errors.New("membership boom")
	svc := &captureMembership{err: wantErr}

	out, err := runMembership(svc, "--org", "acme", "--username", "alice", "--token", "t")
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected the service error to propagate, got %v", err)
	}
	if out != "" {
		t.Fatalf("no success output may be emitted on failure, got %q", out)
	}
	if len(svc.orgs) != 1 {
		t.Fatalf("expected exactly one mutation attempt, got %d", len(svc.orgs))
	}
}
