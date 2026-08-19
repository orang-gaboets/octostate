package contributors

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadConfigTreatsAMissingFileAsNoOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := LoadConfig(filepath.Join(t.TempDir(), "absent.yml"))
	if err != nil {
		t.Fatalf("a missing override file is normal, not an error: %v", err)
	}
	if len(cfg.Exclude) != 0 || len(cfg.Include) != 0 {
		t.Fatalf("expected empty config, got %#v", cfg)
	}
}

func TestLoadConfigReadsOverrides(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contributors.yml")
	if err := os.WriteFile(path, []byte("exclude:\n  - mallory\ninclude:\n  - login: carol\n    name: Carol Reviewer\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Exclude, []string{"mallory"}) {
		t.Fatalf("exclude = %#v", cfg.Exclude)
	}
	if len(cfg.Include) != 1 || cfg.Include[0].Login != "carol" || cfg.Include[0].Name != "Carol Reviewer" {
		t.Fatalf("include = %#v", cfg.Include)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "contributors.yml")
	if err := os.WriteFile(path, []byte("excludes:\n  - typo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil {
		t.Fatal("a misspelled key must fail loudly rather than silently doing nothing")
	}
}

func writeReadme(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "README.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestUpdateReportsChangeAndWritesTheShowcase(t *testing.T) {
	t.Parallel()

	path := writeReadme(t, "# Title\n\n"+startMarker+"\n"+endMarker+"\n")

	changed, err := Update(path, []Contributor{{Login: "alice"}}, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("writing a new showcase must report a change")
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := `href="https://github.com/alice"`; !contains(string(body), want) {
		t.Fatalf("README missing %s:\n%s", want, body)
	}
}

func TestUpdateIsIdempotentAndLeavesTheFileByteIdentical(t *testing.T) {
	t.Parallel()

	path := writeReadme(t, "# Title\n\n"+startMarker+"\n"+endMarker+"\n")
	discovered := []Contributor{{Login: "bob"}, {Login: "alice"}}

	if _, err := Update(path, discovered, Config{}); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	changed, err := Update(path, discovered, Config{})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("re-running against unchanged state must report no change")
	}

	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("re-running must not alter the file:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

func TestUpdateFailsWhenTheReadmeHasNoMarkers(t *testing.T) {
	t.Parallel()

	path := writeReadme(t, "# Title\n\nno markers here\n")
	if _, err := Update(path, []Contributor{{Login: "alice"}}, Config{}); err == nil {
		t.Fatal("expected an error rather than silently leaving the README unchanged")
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
