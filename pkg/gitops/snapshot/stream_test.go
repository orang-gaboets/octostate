package snapshot

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/orang-gaboets/octostate/pkg/gitops/state"
)

func TestWriteActualSnapshotJSONMatchesJSONEncoderForEscapingAndNestedValues(t *testing.T) {
	snapshot := ActualSnapshot{
		PulledAt:                        time.Date(2026, 11, 14, 15, 16, 17, 123456789, time.FixedZone("test", 2*60*60)),
		Organization:                    "<org>&\u2028\u2029\n" + string([]byte{0xff, 'x'}),
		ResolvedInviteUserIDsByUsername: map[string]int64{"z<&>": 2, "a\u2028": 1},
		Members:                         []state.OrganizationMember{{ID: 7, Username: "<alice>", Role: "admin", Name: "line\nname", Email: "a&b@example.test"}},
		PendingInvitations:              []state.PendingInvitation{{ID: 8, Username: "bob", Email: "b@example.test", Role: "direct_member", TeamSlugs: []string{"<platform>", "second"}}},
		Repositories:                    []state.Repository{{Owner: "octostate", Name: "repo", Visibility: "private", Description: "<script>&", Homepage: "https://example.test/?a=1&b=2", Topics: []string{"a", "b"}, AllowForking: true, Archived: false, IsTemplate: true}},
		Teams:                           []state.Team{{ID: 3, Slug: "platform", Name: "Platform", Description: "multi\nline", Privacy: "closed", ParentSlug: "parent"}},
		TeamMembers:                     []state.TeamMember{{TeamSlug: "platform", Username: "alice", Role: "maintainer"}},
		TeamRepositoryPermissions:       []state.TeamRepositoryPermission{{TeamSlug: "platform", Owner: "octostate", Name: "repo", Permission: "push"}},
	}

	var want bytes.Buffer
	encoder := json.NewEncoder(&want)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(snapshot); err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err := writeActualSnapshotJSON(&got, snapshot, actualSnapshotJSONLimits(), maxActualSnapshotJSONSize); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want.Bytes()) {
		t.Fatalf("streamed JSON differs from json.Encoder:\ngot:\n%s\nwant:\n%s", got.Bytes(), want.Bytes())
	}
}

func TestWriteActualSnapshotJSONPrefersByteLimitOverStructureLimit(t *testing.T) {
	var output bytes.Buffer
	err := writeActualSnapshotJSON(&output, sampleSnapshot(), snapshotJSONLimits{}, 4)
	if err == nil || !strings.Contains(err.Error(), actualSnapshotSizeDisplay) {
		t.Fatalf("error = %v, want byte-size limit to take precedence", err)
	}
}

func TestWriteActualSnapshotJSONReturnsDeferredStructureLimit(t *testing.T) {
	snapshot := sampleSnapshot()
	snapshot.Repositories = []state.Repository{{Name: "repo"}}
	for _, test := range []struct {
		name   string
		limits snapshotJSONLimits
		want   string
	}{
		{name: "tokens", limits: snapshotJSONLimits{tokens: 1, arrayElements: 100, objectEntriesPerObject: 100}, want: "JSON token limit"},
		{name: "array elements", limits: snapshotJSONLimits{tokens: 100, arrayElements: 0, objectEntriesPerObject: 100}, want: "JSON array element limit"},
		{name: "object entries", limits: snapshotJSONLimits{tokens: 100, arrayElements: 100, objectEntriesPerObject: 0}, want: "JSON object entry limit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := writeActualSnapshotJSON(&output, snapshot, test.limits, 1<<20)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %s", err, test.want)
			}
			if output.Len() == 0 {
				t.Fatal("structural limit stopped output before the byte cap")
			}
		})
	}
}

func TestWriteActualSnapshotJSONStreamsOversizedScalarAndMapKeyBeforeRejecting(t *testing.T) {
	large := strings.Repeat("x", 2<<20)
	cases := []struct {
		name     string
		snapshot ActualSnapshot
	}{
		{name: "scalar", snapshot: ActualSnapshot{Organization: large}},
		{name: "map key", snapshot: ActualSnapshot{ResolvedInviteUserIDsByUsername: map[string]int64{large: 1}}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			var output bytes.Buffer
			err := writeActualSnapshotJSON(&output, test.snapshot, actualSnapshotJSONLimits(), 64)
			runtime.ReadMemStats(&after)
			if err == nil || !strings.Contains(err.Error(), actualSnapshotSizeDisplay) {
				t.Fatalf("error = %v, want byte-size limit", err)
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
				t.Fatalf("streaming allocated %d bytes after input construction, want under 1 MiB", allocated)
			}
		})
	}
}

func TestActualSnapshotScalarPreflightRejectsOversizedMapKeyWithoutCopyingIt(t *testing.T) {
	large := strings.Repeat("x", 2<<20)
	snapshot := ActualSnapshot{ResolvedInviteUserIDsByUsername: map[string]int64{large: 1}}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	tooLarge := actualSnapshotHasScalarAboveLimit(snapshot, 64)
	runtime.ReadMemStats(&after)
	if !tooLarge {
		t.Fatal("preflight accepted an oversized normalized map key")
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 1<<20 {
		t.Fatalf("preflight allocated %d bytes while inspecting a map key, want under 1 MiB", allocated)
	}
}

func TestActualSnapshotScalarPreflightMatchesResolvedInviteNormalization(t *testing.T) {
	tests := []struct {
		name     string
		username string
		userID   int64
		maxBytes int64
		want     bool
	}{
		{name: "nonpositive ID is discarded", username: strings.Repeat("x", 128), userID: 0, maxBytes: 64},
		{name: "blank username is discarded", username: strings.Repeat(" ", 128), userID: 1, maxBytes: 64},
		{name: "lowercased output length", username: strings.Repeat("\u212a", 30), userID: 1, maxBytes: 64},
		{name: "invalid UTF-8 replacement length", username: strings.Repeat(string([]byte{0xff}), 30), userID: 1, maxBytes: 64, want: true},
		{name: "retained oversized username", username: strings.Repeat("x", 128), userID: 1, maxBytes: 64, want: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			snapshot := ActualSnapshot{ResolvedInviteUserIDsByUsername: map[string]int64{test.username: test.userID}}
			if got := actualSnapshotHasScalarAboveLimit(snapshot, test.maxBytes); got != test.want {
				t.Fatalf("preflight = %t, want %t", got, test.want)
			}
		})
	}
}
