package toolchainremediation

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestClassify(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		fixture string
		current GoVersion
		want    Result
		wantErr bool
	}{
		{
			name:    "no findings",
			fixture: "no_findings.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want:    Result{},
		},
		{
			name:    "one eligible finding",
			fixture: "one_eligible.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want: Result{
				Eligible:         true,
				TargetVersion:    "go1.24.7",
				VulnerabilityIDs: []string{"GO-2025-0001"},
			},
		},
		{
			name:    "multiple eligible findings",
			fixture: "multiple_eligible.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want: Result{
				Eligible:         true,
				TargetVersion:    "go1.24.9",
				VulnerabilityIDs: []string{"GO-2025-0001", "GO-2025-0002"},
			},
		},
		{
			name:    "same patch duplicates",
			fixture: "same_patch_duplicates.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want: Result{
				Eligible:         true,
				TargetVersion:    "go1.24.7",
				VulnerabilityIDs: []string{"GO-2025-0001"},
			},
		},
		{
			name:    "unordered duplicates",
			fixture: "unordered_duplicates.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want: Result{
				Eligible:         true,
				TargetVersion:    "go1.24.8",
				VulnerabilityIDs: []string{"GO-2025-0001", "GO-2025-0002"},
			},
		},
		{
			name:    "mixed third-party findings",
			fixture: "mixed_third_party.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "missing fix",
			fixture: "missing_fix.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "cross minor fix",
			fixture: "cross_minor_fix.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want:    Result{},
		},
		{
			name:    "eligible plus cross minor fix",
			fixture: "eligible_plus_cross_minor.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want:    Result{},
		},
		{
			name:    "eligible plus non increasing fix",
			fixture: "eligible_plus_non_increasing.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want:    Result{},
		},
		{
			name:    "malformed trailing data",
			fixture: "malformed_trailing_data.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "incomplete stream",
			fixture: "incomplete_stream.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "conflicting duplicates",
			fixture: "conflicting_duplicates.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "conflicting module identities",
			fixture: "conflicting_module_identities.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "non increasing fix",
			fixture: "non_increasing_fix.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			want:    Result{},
		},
		{
			name:    "unsupported scanner output",
			fixture: "unsupported_scanner.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "missing scanner name",
			fixture: "missing_scanner_name.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "malformed fixed version",
			fixture: "malformed_fixed_version.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
		{
			name:    "prerelease fixed version",
			fixture: "prerelease_fixed_version.json",
			current: GoVersion{Major: 1, Minor: 24, Patch: 6},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			data := mustReadFixture(t, tc.fixture)
			got, err := Classify(bytes.NewReader(data), tc.current)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Classify(%s) = %#v, want error", tc.fixture, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Classify(%s) returned error: %v", tc.fixture, err)
			}
			if got.Eligible != tc.want.Eligible {
				t.Fatalf("Eligible = %v, want %v", got.Eligible, tc.want.Eligible)
			}
			if got.TargetVersion != tc.want.TargetVersion {
				t.Fatalf("TargetVersion = %q, want %q", got.TargetVersion, tc.want.TargetVersion)
			}
			if !bytes.Equal([]byte(joinIDs(got.VulnerabilityIDs)), []byte(joinIDs(tc.want.VulnerabilityIDs))) {
				t.Fatalf("VulnerabilityIDs = %#v, want %#v", got.VulnerabilityIDs, tc.want.VulnerabilityIDs)
			}
		})
	}
}

func mustReadFixture(t *testing.T, name string) []byte {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %q: %v", name, err)
	}
	return data
}

func joinIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	var buf bytes.Buffer
	for i, id := range ids {
		if i > 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(id)
	}
	return buf.String()
}
