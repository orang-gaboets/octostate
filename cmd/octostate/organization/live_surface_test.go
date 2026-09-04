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
)

// captureInvite records exactly what the CLI hands the invitation service.
type captureInvite struct {
	auth.MockOrganizationService
	calls []*gh.CreateOrgInvitationOptions
	err   error
}

func (s *captureInvite) CreateOrgInvitation(_ context.Context, _ string, opts *gh.CreateOrgInvitationOptions) (*gh.Invitation, *gh.Response, error) {
	s.calls = append(s.calls, opts)
	if s.err != nil {
		return nil, nil, s.err
	}
	return &gh.Invitation{}, nil, nil
}

type rejectingUserLookup struct {
	auth.MockUserService
	called bool
}

func (s *rejectingUserLookup) Get(_ context.Context, _ string) (*gh.User, *gh.Response, error) {
	s.called = true
	return &gh.User{ID: gh.Ptr(int64(7))}, nil, nil
}

// captureTeams resolves slugs to ids, or fails, so team wiring is testable
// without a live GitHub call.
type captureTeams struct {
	auth.MockTeamsService
	ids  map[string]int64
	err  error
	seen []string
}

func (s *captureTeams) GetTeamBySlug(_ context.Context, _, slug string) (*gh.Team, *gh.Response, error) {
	s.seen = append(s.seen, slug)
	if s.err != nil {
		return nil, nil, s.err
	}
	id, ok := s.ids[slug]
	if !ok {
		// A team returned without a usable ID.
		return &gh.Team{Slug: gh.Ptr(slug)}, nil, nil
	}
	return &gh.Team{ID: gh.Ptr(id), Slug: gh.Ptr(slug)}, nil, nil
}

func runInvite(t *testing.T, org *captureInvite, users *rejectingUserLookup, team *captureTeams, args ...string) error {
	t.Helper()

	var userSvc = (*rejectingUserLookup)(nil)
	if users != nil {
		userSvc = users
	}
	var teamSvc = (*captureTeams)(nil)
	if team != nil {
		teamSvc = team
	}

	cmd := organizationcmd.InviteCmd(org, userSvc, teamSvc)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestInviteLiveEmailUsesEmailWithoutUserLookup(t *testing.T) {
	org := &captureInvite{}
	users := &rejectingUserLookup{}

	if err := runInvite(t, org, users, nil, "--org", "o", "--email", "alice@example.com", "--token", "t"); err != nil {
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
	if users.called {
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
			if err := runInvite(t, org, nil, nil, args...); err != nil {
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

	if err := runInvite(t, org, nil, team,
		"--org", "o", "--email", "a@example.com", "--token", "t",
		"--team-slug", "platform", "--team-slug", "backend"); err != nil {
		t.Fatal(err)
	}

	got := org.calls[0].TeamID
	if len(got) != 2 || got[0] != 11 || got[1] != 22 {
		t.Fatalf("team IDs = %#v, want [11 22] in caller order", got)
	}
}

func TestInviteLiveTeamLookupFailurePreventsInvitation(t *testing.T) {
	org := &captureInvite{}
	team := &captureTeams{err: errors.New("boom")}

	err := runInvite(t, org, nil, team, "--org", "o", "--email", "a@example.com", "--token", "t", "--team-slug", "platform")
	if err == nil {
		t.Fatal("a failed team lookup must fail the command")
	}
	if len(org.calls) != 0 {
		t.Fatal("no invitation may be sent when a requested team cannot be resolved")
	}
}

func TestInviteLiveUnusableTeamIDPreventsInvitation(t *testing.T) {
	org := &captureInvite{}
	team := &captureTeams{ids: map[string]int64{}} // resolves, but without an ID

	err := runInvite(t, org, nil, team, "--org", "o", "--email", "a@example.com", "--token", "t", "--team-slug", "platform")
	if err == nil {
		t.Fatal("a team without a usable ID must fail the command")
	}
	if len(org.calls) != 0 {
		t.Fatal("no invitation may be sent when a team ID is unusable")
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

func TestMembershipSetLiveForwardsRequest(t *testing.T) {
	svc := &captureMembership{}

	cmd := organizationcmd.MembershipSetCmd(svc)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--org", " acme ", "--username", " alice ", "--role", "admin", "--token", "t"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if len(svc.orgs) != 1 {
		t.Fatalf("expected exactly one membership mutation, got %d", len(svc.orgs))
	}
	if svc.orgs[0] != "acme" || svc.usernames[0] != "alice" || svc.roles[0] != "admin" {
		t.Fatalf("forwarded org=%q user=%q role=%q", svc.orgs[0], svc.usernames[0], svc.roles[0])
	}
	// The wording must not claim the user is already an active member.
	if !strings.Contains(out.String(), "Requested organization membership") {
		t.Fatalf("unexpected message: %s", out.String())
	}
}

func TestMembershipSetLivePropagatesServiceError(t *testing.T) {
	wantErr := errors.New("membership boom")
	svc := &captureMembership{err: wantErr}

	cmd := organizationcmd.MembershipSetCmd(svc)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--org", "acme", "--username", "alice", "--token", "t"})

	err := cmd.Execute()
	if err == nil || !errors.Is(err, wantErr) {
		t.Fatalf("expected the service error to propagate, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("no success output may be emitted on failure, got %q", out.String())
	}
	if len(svc.orgs) != 1 {
		t.Fatalf("expected exactly one mutation attempt, got %d", len(svc.orgs))
	}
}
