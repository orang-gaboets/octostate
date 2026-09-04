package organization_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	organizationcmd "github.com/orang-gaboets/octostate/cmd/octostate/organization"
	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

type surfaceData struct {
	Organization string   `json:"organization"`
	Username     string   `json:"username"`
	Email        string   `json:"email"`
	UserID       int64    `json:"user_id"`
	Role         string   `json:"role"`
	TeamSlugs    []string `json:"team_slugs"`
	ConfigPath   string   `json:"config_path"`
	Changed      bool     `json:"changed"`
}

type surfaceResult struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Data    surfaceData `json:"data"`
}

// Teams are declared because config validation requires an invite team slug to
// reference a declared team, so proposal mode cannot attach an invitation to a
// team the config does not know about.
const surfaceBaseConfig = `organization: o
members: []
invites: []
repositories: []
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members: []
    repositories: []
  - slug: backend
    name: Backend
    privacy: closed
    members: []
    repositories: []
`

func surfaceConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(path, []byte(surfaceBaseConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func decodeSurface(t *testing.T, output string) surfaceResult {
	t.Helper()

	var result surfaceResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("decode output: %v\nraw: %s", err, output)
	}
	return result
}

// --- #280: invite identity, role, and team assignment ---

func TestInviteToConfigRecordsEmailRoleAndTeamSlugs(t *testing.T) {
	path := surfaceConfig(t)

	cmd := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{
		"--org", "o", "--email", " alice@example.com ",
		"--role", "admin", "--team-slug", "platform", "--team-slug", "backend",
		"--to-config", path,
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeSurface(t, out.String())
	if result.Status != "success" {
		t.Fatalf("status = %q", result.Status)
	}
	if result.Data.Email != "alice@example.com" || result.Data.Role != "admin" {
		t.Fatalf("unexpected identity/role: %#v", result.Data)
	}
	if strings.Join(result.Data.TeamSlugs, ",") != "platform,backend" {
		t.Fatalf("team slugs = %#v", result.Data.TeamSlugs)
	}

	// Asserted against the loaded config rather than by substring: the base
	// fixture already contains "platform" and "backend" in its team
	// declarations, so a substring check would pass even if team_slugs were
	// never written to the invite.
	cfg, err := gitopsconfig.LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Invites) != 1 {
		t.Fatalf("invites = %#v", cfg.Invites)
	}
	invite := cfg.Invites[0]
	if !invite.Email.Present || invite.Email.Value != "alice@example.com" {
		t.Fatalf("unexpected invite email: %#v", invite.Email)
	}
	if invite.Role != "admin" {
		t.Fatalf("role = %q", invite.Role)
	}
	if len(invite.TeamSlugs) != 2 || invite.TeamSlugs[0] != "platform" || invite.TeamSlugs[1] != "backend" {
		t.Fatalf("invite team slugs = %#v, want [platform backend]", invite.TeamSlugs)
	}
}

func TestInviteRejectsMoreThanOneIdentity(t *testing.T) {
	cmd := organizationcmd.InviteCmd(nil, nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--org", "o", "--username", "octocat", "--email", "a@example.com"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("providing two identities must fail")
	}
}

func TestInviteRejectsUnsupportedRole(t *testing.T) {
	cmd := organizationcmd.InviteCmd(nil, nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	// A durable membership role is not an invitation role.
	cmd.SetArgs([]string{"--org", "o", "--email", "a@example.com", "--role", "member"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("member is not a valid invitation role")
	}
}

func TestInviteDryRunReportsRoleAndTeamsWithoutMutating(t *testing.T) {
	cmd := organizationcmd.InviteCmd(nil, nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--org", "o", "--email", "a@example.com", "--role", "billing_manager", "--team-slug", "platform", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeSurface(t, out.String())
	if result.Status != "dry-run" {
		t.Fatalf("status = %q", result.Status)
	}
	if result.Data.Role != "billing_manager" || len(result.Data.TeamSlugs) != 1 {
		t.Fatalf("dry run must report the requested shape: %#v", result.Data)
	}
}

func TestInviteToConfigEmailNoOpReportsRetainedShape(t *testing.T) {
	path := surfaceConfig(t)

	for i := 0; i < 2; i++ {
		cmd := organizationcmd.InviteCmd(nil, nil)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		args := []string{"--org", "o", "--email", "a@example.com", "--to-config", path}
		if i == 1 {
			// Second run asks for a different role; the retained entry wins.
			args = append(args, "--role", "admin")
		}
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}

		result := decodeSurface(t, out.String())
		if i == 1 {
			if result.Data.Changed {
				t.Fatalf("second run must be a no-op: %#v", result.Data)
			}
			if result.Data.Role != "direct_member" {
				t.Fatalf("no-op must report the retained role, got %q", result.Data.Role)
			}
		}
	}
}

// --- #282: durable organization membership ---

func TestMembershipSetToConfigAddsMember(t *testing.T) {
	path := surfaceConfig(t)

	cmd := organizationcmd.MembershipSetCmd(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--org", "o", "--username", " alice ", "--role", "admin", "--to-config", path})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	result := decodeSurface(t, out.String())
	if !result.Data.Changed || result.Data.Username != "alice" || result.Data.Role != "admin" {
		t.Fatalf("unexpected result: %#v", result.Data)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "username: alice") || !strings.Contains(string(written), "role: admin") {
		t.Fatalf("member not written:\n%s", written)
	}
}

func TestMembershipSetToConfigUpdatesRoleThenNoOps(t *testing.T) {
	path := surfaceConfig(t)

	run := func(role string) surfaceResult {
		cmd := organizationcmd.MembershipSetCmd(nil)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--org", "o", "--username", "alice", "--role", role, "--to-config", path})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		return decodeSurface(t, out.String())
	}

	run("member")
	if updated := run("admin"); !updated.Data.Changed {
		t.Fatal("role change must be reported as changed")
	}
	if noop := run("admin"); noop.Data.Changed {
		t.Fatalf("identical proposal must be a no-op: %#v", noop.Data)
	}
}

func TestMembershipSetRejectsInvitationOnlyRole(t *testing.T) {
	cmd := organizationcmd.MembershipSetCmd(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--org", "o", "--username", "alice", "--role", "direct_member"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("direct_member is an invitation role, not a durable membership role")
	}
}

func TestMembershipSetRejectsToConfigWithDryRun(t *testing.T) {
	cmd := organizationcmd.MembershipSetCmd(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SilenceUsage = true
	cmd.SetArgs([]string{"--org", "o", "--username", "alice", "--to-config", surfaceConfig(t), "--dry-run"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("--to-config and --dry-run must be mutually exclusive")
	}
}

func TestMembershipSetDryRunDoesNotMutate(t *testing.T) {
	cmd := organizationcmd.MembershipSetCmd(nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--org", "o", "--username", "alice", "--role", "admin", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if result := decodeSurface(t, out.String()); result.Status != "dry-run" || result.Data.Role != "admin" {
		t.Fatalf("unexpected dry run: %#v", result)
	}
}
