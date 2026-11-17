package snapshot

import (
	"bytes"
	"encoding/json"
	"io"
	"reflect"
	"runtime"
	"sort"
	"strconv"
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

	assertActualSnapshotJSONMatchesEncoder(t, snapshot)
}

func TestWriteActualSnapshotJSONMatchesJSONEncoderForEmptyCollections(t *testing.T) {
	snapshot := ActualSnapshot{
		PulledAt:                        time.Date(2026, 11, 14, 15, 16, 17, 0, time.UTC),
		ResolvedInviteUserIDsByUsername: map[string]int64{},
		Members:                         []state.OrganizationMember{},
		PendingInvitations:              []state.PendingInvitation{{Username: "empty teams", TeamSlugs: []string{}}},
		Repositories:                    []state.Repository{{Name: "empty topics", Topics: []string{}}},
		Teams:                           []state.Team{},
		TeamMembers:                     []state.TeamMember{},
		TeamRepositoryPermissions:       []state.TeamRepositoryPermission{},
	}

	if got := assertActualSnapshotJSONMatchesEncoder(t, snapshot); !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("JSON round trip changed empty collections:\n got %#v\nwant %#v", got, snapshot)
	}
}

func TestActualSnapshotJSONFieldCoverage(t *testing.T) {
	expected := map[reflect.Type][]string{
		reflect.TypeOf(ActualSnapshot{}): {
			"pulled_at", "organization", "resolved_invite_user_ids_by_username", "members",
			"pending_invitations", "repositories", "teams", "team_members", "team_repo_permissions",
		},
		reflect.TypeOf(state.OrganizationMember{}): {"id", "username", "role", "name", "email"},
		reflect.TypeOf(state.PendingInvitation{}):  {"id", "username", "email", "role", "team_slugs"},
		reflect.TypeOf(state.Repository{}): {
			"owner", "name", "visibility", "description", "homepage", "topics", "allow_forking", "archived", "is_template",
		},
		reflect.TypeOf(state.Team{}):                     {"id", "slug", "name", "description", "privacy", "parent_slug"},
		reflect.TypeOf(state.TeamMember{}):               {"team_slug", "username", "role"},
		reflect.TypeOf(state.TeamRepositoryPermission{}): {"team_slug", "owner", "name", "permission"},
	}

	types := make([]reflect.Type, 0, len(expected))
	for typ := range expected {
		types = append(types, typ)
	}
	sort.Slice(types, func(i, j int) bool { return types[i].String() < types[j].String() })

	for _, typ := range types {
		want := expected[typ]
		got := exportedJSONFieldNames(typ)
		sort.Strings(got)
		sort.Strings(want)
		missing, extra := sortedStringListDifference(want, got), sortedStringListDifference(got, want)
		repeated := repeatedSortedStrings(got)
		if len(missing) != 0 || len(extra) != 0 {
			t.Errorf("%s JSON field coverage mismatch: missing keys %v; extra keys %v; repeated keys %v", typ, missing, extra, repeated)
		}
	}
}

func TestExportedJSONFieldNamesHandlesDashTagOptions(t *testing.T) {
	type taggedFields struct {
		Ignored string `json:"-"`
		Literal string `json:"-,omitempty"`
	}

	got := exportedJSONFieldNames(reflect.TypeOf(taggedFields{}))
	if !reflect.DeepEqual(got, []string{"-"}) {
		t.Fatalf("exportedJSONFieldNames() = %v, want [\"-\"]", got)
	}
}

func exportedJSONFieldNames(typ reflect.Type) []string {
	var names []string
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" { // unexported
			continue
		}
		rawTag := field.Tag.Get("json")
		if rawTag == "-" {
			continue
		}
		tag := rawTag
		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = field.Name
		}
		names = append(names, name)
	}
	return names
}

func sortedStringListDifference(left, right []string) []string {
	var difference []string
	leftIndex, rightIndex := 0, 0
	for leftIndex < len(left) {
		if rightIndex == len(right) || left[leftIndex] < right[rightIndex] {
			difference = append(difference, left[leftIndex])
			leftIndex++
			continue
		}
		if left[leftIndex] == right[rightIndex] {
			leftIndex++
			rightIndex++
			continue
		}
		rightIndex++
	}
	return difference
}

func repeatedSortedStrings(values []string) []string {
	var repeated []string
	for i := 1; i < len(values); i++ {
		if values[i] == values[i-1] && (len(repeated) == 0 || repeated[len(repeated)-1] != values[i]) {
			repeated = append(repeated, values[i])
		}
	}
	return repeated
}

func assertActualSnapshotJSONMatchesEncoder(t *testing.T, snapshot ActualSnapshot) ActualSnapshot {
	t.Helper()

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
	var decoded ActualSnapshot
	if err := json.Unmarshal(got.Bytes(), &decoded); err != nil {
		t.Fatalf("decode streamed JSON: %v", err)
	}
	return decoded
}

func TestWriteActualSnapshotJSONPrefersByteLimitOverStructureLimit(t *testing.T) {
	var output bytes.Buffer
	err := writeActualSnapshotJSON(&output, sampleSnapshot(), snapshotJSONLimits{}, 4)
	if err == nil || !strings.Contains(err.Error(), actualSnapshotSizeDisplay) {
		t.Fatalf("error = %v, want byte-size limit to take precedence", err)
	}
}

func TestWriteActualSnapshotJSONEnforcesOutputLimitAtByteBoundary(t *testing.T) {
	var baseline bytes.Buffer
	if err := writeActualSnapshotJSON(&baseline, ActualSnapshot{}, actualSnapshotJSONLimits(), maxActualSnapshotJSONSize); err != nil {
		t.Fatalf("encode baseline: %v", err)
	}
	maxBytes := int64(baseline.Len() + 64)

	for _, test := range []struct {
		name     string
		orgBytes int
		wantErr  bool
	}{
		{name: "exact limit accepted", orgBytes: 64},
		{name: "one byte over rejected", orgBytes: 65, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := ActualSnapshot{Organization: strings.Repeat("x", test.orgBytes)}
			var output bytes.Buffer
			err := writeActualSnapshotJSON(&output, snapshot, actualSnapshotJSONLimits(), maxBytes)
			if test.wantErr {
				if err == nil {
					t.Fatal("writeActualSnapshotJSON accepted output above the byte limit")
				}
				return
			}
			if err != nil {
				t.Fatalf("writeActualSnapshotJSON rejected output at the limit: %v", err)
			}
			if int64(output.Len()) != maxBytes {
				t.Fatalf("encoded size = %d, want %d", output.Len(), maxBytes)
			}
		})
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

func TestWriteActualSnapshotJSONRejectsOverLimitMap(t *testing.T) {
	snapshot := ActualSnapshot{ResolvedInviteUserIDsByUsername: overLimitResolvedInviteIDs()}
	err := writeActualSnapshotJSON(io.Discard, snapshot, actualSnapshotJSONLimits(), maxActualSnapshotJSONSize)
	if err == nil || !strings.Contains(err.Error(), "JSON object entry limit") {
		t.Fatalf("writeActualSnapshotJSON error = %v, want object entry limit", err)
	}
}

func overLimitResolvedInviteIDs() map[string]int64 {
	values := make(map[string]int64, maxActualSnapshotJSONObjectEntries+1)
	for userID := int64(0); userID <= maxActualSnapshotJSONObjectEntries; userID++ {
		values[strconv.FormatInt(userID, 10)] = userID
	}
	return values
}
