package syncfromlive

import (
	"strings"
	"testing"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func TestEnsureNoUnsupportedDirectMembersAllowsTeamBackedMembers(t *testing.T) {
	t.Parallel()

	err := ensureNoUnsupportedDirectMembers(
		[]state.OrganizationMember{
			{Username: " Alice "},
			{Username: "bob"},
		},
		[]state.TeamMember{
			{TeamSlug: "platform", Username: "alice", Role: "maintainer"},
			{TeamSlug: "platform", Username: " bob ", Role: "member"},
		},
	)
	if err != nil {
		t.Fatalf("expected no error for team-backed members, got %v", err)
	}
}

func TestEnsureNoUnsupportedDirectMembersRejectsDirectMembers(t *testing.T) {
	t.Parallel()

	err := ensureNoUnsupportedDirectMembers(
		[]state.OrganizationMember{
			{Username: "bob"},
			{Email: "carol@example.com"},
		},
		[]state.TeamMember{
			{TeamSlug: "platform", Username: "alice", Role: "member"},
		},
	)
	if err == nil {
		t.Fatal("expected unsupported direct member error")
	}
	if !strings.Contains(err.Error(), "direct organization members outside teams") {
		t.Fatalf("expected direct member error, got %v", err)
	}
	if !strings.Contains(err.Error(), "bob") || !strings.Contains(err.Error(), "carol@example.com") {
		t.Fatalf("expected member labels in error, got %v", err)
	}
}
