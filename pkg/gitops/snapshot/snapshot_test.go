package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/orang-gaboets/repo-builder/pkg/gitops/state"
)

func TestNewActualSnapshotNormalizesAndClones(t *testing.T) {
	t.Parallel()

	pulledAt := time.Date(2026, 3, 10, 15, 4, 5, 0, time.FixedZone("SGT", 8*60*60))
	actual := &state.OrganizationState{
		Organization: "orang-gaboets",
		PendingInvitations: []state.PendingInvitation{
			{Username: "zoe", TeamSlugs: []string{"writers", "admins"}},
		},
		Repositories: []state.Repository{
			{Name: "repo-builder", Owner: "orang-gaboets", Topics: []string{"zeta", "alpha"}},
		},
		Teams: []state.Team{
			{ID: 2, Slug: "zeta"},
			{ID: 1, Slug: "alpha"},
		},
	}

	snapshot := NewActualSnapshot(pulledAt, actual)

	if !snapshot.PulledAt.Equal(pulledAt.UTC()) {
		t.Fatalf("unexpected pulled time: got %v want %v", snapshot.PulledAt, pulledAt.UTC())
	}
	if !reflect.DeepEqual(snapshot.PendingInvitations[0].TeamSlugs, []string{"admins", "writers"}) {
		t.Fatalf("unexpected team slugs: %#v", snapshot.PendingInvitations[0].TeamSlugs)
	}
	if !reflect.DeepEqual(snapshot.Repositories[0].Topics, []string{"alpha", "zeta"}) {
		t.Fatalf("unexpected topics: %#v", snapshot.Repositories[0].Topics)
	}
	if !reflect.DeepEqual(snapshot.Teams, []state.Team{
		{ID: 1, Slug: "alpha"},
		{ID: 2, Slug: "zeta"},
	}) {
		t.Fatalf("unexpected teams: %#v", snapshot.Teams)
	}

	if !reflect.DeepEqual(actual.PendingInvitations[0].TeamSlugs, []string{"writers", "admins"}) {
		t.Fatalf("expected input invitation team slugs to remain unchanged, got %#v", actual.PendingInvitations[0].TeamSlugs)
	}
	if !reflect.DeepEqual(actual.Repositories[0].Topics, []string{"zeta", "alpha"}) {
		t.Fatalf("expected input repository topics to remain unchanged, got %#v", actual.Repositories[0].Topics)
	}
	if !reflect.DeepEqual(actual.Teams, []state.Team{
		{ID: 2, Slug: "zeta"},
		{ID: 1, Slug: "alpha"},
	}) {
		t.Fatalf("expected input teams to remain unchanged, got %#v", actual.Teams)
	}

	actual.PendingInvitations[0].TeamSlugs[0] = "mutated"
	actual.Repositories[0].Topics[0] = "mutated"
	actual.Teams[0].Slug = "mutated"
	if snapshot.PendingInvitations[0].TeamSlugs[0] != "admins" {
		t.Fatalf("expected cloned invitation team slugs, got %#v", snapshot.PendingInvitations[0].TeamSlugs)
	}
	if snapshot.Repositories[0].Topics[0] != "alpha" {
		t.Fatalf("expected cloned repository topics, got %#v", snapshot.Repositories[0].Topics)
	}
	if snapshot.Teams[0].Slug != "alpha" {
		t.Fatalf("expected cloned teams, got %#v", snapshot.Teams)
	}
}

func TestNewActualSnapshotPreservesEmptyNestedSlices(t *testing.T) {
	t.Parallel()

	snapshot := NewActualSnapshot(time.Date(2026, 3, 10, 15, 4, 5, 0, time.UTC), &state.OrganizationState{
		Organization: "orang-gaboets",
		PendingInvitations: []state.PendingInvitation{
			{Username: "zoe", TeamSlugs: []string{}},
		},
		Repositories: []state.Repository{
			{Name: "repo-builder", Owner: "orang-gaboets", Topics: []string{}},
		},
	})

	if snapshot.PendingInvitations[0].TeamSlugs == nil {
		t.Fatal("expected pending invitation team slugs to stay non-nil")
	}
	if snapshot.Repositories[0].Topics == nil {
		t.Fatal("expected repository topics to stay non-nil")
	}
}

func TestNewActualSnapshotNilActualInitializesSlices(t *testing.T) {
	t.Parallel()

	pulledAt := time.Date(2026, 3, 10, 15, 4, 5, 0, time.FixedZone("SGT", 8*60*60))

	snapshot := NewActualSnapshot(pulledAt, nil)

	if !snapshot.PulledAt.Equal(pulledAt.UTC()) {
		t.Fatalf("unexpected pulled time: got %v want %v", snapshot.PulledAt, pulledAt.UTC())
	}
	if snapshot.Organization != "" {
		t.Fatalf("expected empty organization, got %q", snapshot.Organization)
	}
	if snapshot.Members == nil {
		t.Fatal("expected members to be initialized")
	}
	if snapshot.PendingInvitations == nil {
		t.Fatal("expected pending invitations to be initialized")
	}
	if snapshot.Repositories == nil {
		t.Fatal("expected repositories to be initialized")
	}
	if snapshot.Teams == nil {
		t.Fatal("expected teams to be initialized")
	}
	if snapshot.TeamMembers == nil {
		t.Fatal("expected team members to be initialized")
	}
	if snapshot.TeamRepositoryPermissions == nil {
		t.Fatal("expected team repository permissions to be initialized")
	}
}

func TestWriteActualWritesSnapshotFile(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	snapshot := ActualSnapshot{
		PulledAt:     time.Date(2026, 3, 10, 7, 8, 9, 0, time.UTC),
		Organization: "orang-gaboets",
		Members: []state.OrganizationMember{
			{ID: 1, Username: "alice"},
		},
	}

	path, err := WriteActual(stateDir, snapshot)
	if err != nil {
		t.Fatalf("WriteActual returned error: %v", err)
	}

	wantPath := filepath.Join(stateDir, "actual", "snapshot.json")
	if path != wantPath {
		t.Fatalf("unexpected path: got %q want %q", path, wantPath)
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}

	var got ActualSnapshot
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("decode snapshot JSON: %v; payload=%q", err, string(payload))
	}

	if !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("unexpected snapshot contents:\n got %#v\nwant %#v", got, snapshot)
	}
}

func TestWriteActualOverwritesExistingSnapshotFile(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	actualDir := filepath.Join(stateDir, "actual")
	if err := os.MkdirAll(actualDir, 0o755); err != nil {
		t.Fatalf("mkdir actual dir: %v", err)
	}

	existingPath := filepath.Join(actualDir, "snapshot.json")
	if err := os.WriteFile(existingPath, []byte(`{"old":true}`), 0o600); err != nil {
		t.Fatalf("write existing snapshot: %v", err)
	}

	snapshot := ActualSnapshot{
		PulledAt:     time.Date(2026, 3, 10, 7, 8, 9, 0, time.UTC),
		Organization: "orang-gaboets",
	}

	path, err := WriteActual(stateDir, snapshot)
	if err != nil {
		t.Fatalf("WriteActual returned error: %v", err)
	}
	if path != existingPath {
		t.Fatalf("unexpected path: got %q want %q", path, existingPath)
	}

	payload, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	if string(payload) == `{"old":true}` {
		t.Fatalf("expected existing snapshot to be replaced, got %q", string(payload))
	}

	matches, err := filepath.Glob(filepath.Join(actualDir, "snapshot-*.json"))
	if err != nil {
		t.Fatalf("glob temp snapshots: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected no temporary snapshot files, got %#v", matches)
	}
}

func TestWriteActualRejectsEmptyStateDir(t *testing.T) {
	t.Parallel()

	if _, err := WriteActual("   ", ActualSnapshot{}); err == nil {
		t.Fatal("expected error for empty state dir")
	}
}
