package contributors

import (
	"reflect"
	"strings"
	"testing"
)

func TestSelectOrdersContributorsAlphabeticallyRegardlessOfInputOrder(t *testing.T) {
	t.Parallel()

	// Deliberately reverse-alphabetical, and the first entry would win any
	// contribution-volume ordering. Neither may influence the result.
	fetched := []Contributor{
		{Login: "zoe", Contributions: 900},
		{Login: "Alice", Contributions: 1},
		{Login: "bob", Contributions: 50},
	}

	got := Select(fetched, Config{})

	want := []string{"Alice", "bob", "zoe"}
	if logins := logins(got); !reflect.DeepEqual(logins, want) {
		t.Fatalf("ordering = %v, want %v", logins, want)
	}
}

func TestSelectExcludesBotAccounts(t *testing.T) {
	t.Parallel()

	fetched := []Contributor{
		{Login: "dependabot[bot]"},
		{Login: "release-please[bot]"},
		{Login: "service-account", Type: "Bot"},
		{Login: "alice", Type: "User"},
	}

	if got := logins(Select(fetched, Config{})); !reflect.DeepEqual(got, []string{"alice"}) {
		t.Fatalf("bots must not appear in the human showcase, got %v", got)
	}
}

func TestSelectAppliesExcludeOverride(t *testing.T) {
	t.Parallel()

	fetched := []Contributor{{Login: "alice"}, {Login: "mallory"}}
	cfg := Config{Exclude: []string{"MALLORY"}}

	if got := logins(Select(fetched, cfg)); !reflect.DeepEqual(got, []string{"alice"}) {
		t.Fatalf("exclude override should drop the login case-insensitively, got %v", got)
	}
}

func TestSelectAppliesIncludeOverrideForContributorsAutomaticDiscoveryMisses(t *testing.T) {
	t.Parallel()

	fetched := []Contributor{{Login: "alice"}}
	cfg := Config{Include: []Contributor{{Login: "carol", Name: "Carol Reviewer"}}}

	got := Select(fetched, cfg)
	if l := logins(got); !reflect.DeepEqual(l, []string{"alice", "carol"}) {
		t.Fatalf("include override should add a contributor, got %v", l)
	}
	if got[1].Name != "Carol Reviewer" {
		t.Fatalf("include override should carry its display name, got %q", got[1].Name)
	}
}

func TestSelectDoesNotDuplicateAnIncludedContributorAlreadyDiscovered(t *testing.T) {
	t.Parallel()

	fetched := []Contributor{{Login: "alice"}}
	cfg := Config{Include: []Contributor{{Login: "Alice", Name: "Alice Example"}}}

	got := Select(fetched, cfg)
	if len(got) != 1 {
		t.Fatalf("expected one entry, got %v", logins(got))
	}
	if got[0].Name != "Alice Example" {
		t.Fatalf("explicit override should win over discovered data, got %q", got[0].Name)
	}
}

func TestSelectExcludeWinsOverInclude(t *testing.T) {
	t.Parallel()

	cfg := Config{Exclude: []string{"carol"}, Include: []Contributor{{Login: "carol"}}}
	if got := Select(nil, cfg); len(got) != 0 {
		t.Fatalf("exclude must win over include, got %v", logins(got))
	}
}

func TestRenderLinksEachAvatarToItsProfileWithAccessibleAltText(t *testing.T) {
	t.Parallel()

	block := Render([]Contributor{{Login: "alice", Name: "Alice Example"}, {Login: "bob"}})

	for _, want := range []string{
		`href="https://github.com/alice"`,
		`src="https://github.com/alice.png?size=100"`,
		`alt="Alice Example"`,
		`width="100"`,
		`height="100"`,
		`alt="bob"`,
	} {
		if !strings.Contains(block, want) {
			t.Fatalf("rendered block missing %s:\n%s", want, block)
		}
	}
}

func TestRenderEscapesContributorText(t *testing.T) {
	t.Parallel()

	block := Render([]Contributor{{Login: "eve", Name: `Eve" onerror="alert(1)`}})
	if strings.Contains(block, `onerror="alert(1)"`) {
		t.Fatalf("contributor display name must be escaped:\n%s", block)
	}
}

func TestApplyReplacesOnlyTheMarkedRegion(t *testing.T) {
	t.Parallel()

	readme := "# Title\n\nbefore\n\n" + startMarker + "\nstale\n" + endMarker + "\n\nafter\n"

	got, err := Apply(readme, "fresh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatalf("content outside the markers must be preserved:\n%s", got)
	}
	if strings.Contains(got, "stale") {
		t.Fatalf("stale content must be replaced:\n%s", got)
	}
	if !strings.Contains(got, "fresh") {
		t.Fatalf("new content must be written:\n%s", got)
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	t.Parallel()

	readme := "# Title\n\n" + startMarker + "\nold\n" + endMarker + "\n"
	block := Render([]Contributor{{Login: "alice"}})

	once, err := Apply(readme, block)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Apply(once, block)
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Fatalf("re-running against unchanged state must produce no diff:\n--- once ---\n%s\n--- twice ---\n%s", once, twice)
	}
}

func TestApplyRejectsMalformedMarkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		readme string
	}{
		{"missing start", "# Title\n" + endMarker + "\n"},
		{"missing end", "# Title\n" + startMarker + "\n"},
		{"reversed", "# Title\n" + endMarker + "\n" + startMarker + "\n"},
		{"duplicate start", "# Title\n" + startMarker + "\n" + startMarker + "\n" + endMarker + "\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := Apply(tt.readme, "x"); err == nil {
				t.Fatal("expected an error rather than a silently mangled README")
			}
		})
	}
}

func logins(contributors []Contributor) []string {
	out := make([]string, 0, len(contributors))
	for _, c := range contributors {
		out = append(out, c.Login)
	}
	return out
}
