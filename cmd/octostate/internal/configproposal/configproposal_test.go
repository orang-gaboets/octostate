package configproposal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	gitopsconfig "github.com/orang-gaboets/octostate/pkg/gitops/config"
)

func TestApplyToConfigFileSuccess(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "organization: ' ORANG-GABOETS '\nmembers: []\n")

	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(cfg *gitopsconfig.OrganizationConfig) error {
		cfg.Members = append(cfg.Members, gitopsconfig.OrganizationMemberSpec{
			Username: "alice",
			Role:     "member",
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected config to change")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "organization: ORANG-GABOETS\nmembers:\n  - username: alice\n    role: member\ninvites: []\nrepositories: []\nteams: []\n"
	if string(got) != want {
		t.Fatalf("unexpected rewritten config:\n%s\nwant:\n%s", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Fatalf("unexpected file mode: got %v want %v", got, want)
	}
}

func TestApplyToConfigFileMissingFileDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "organization.yaml")
	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(*gitopsconfig.OrganizationConfig) error {
		t.Fatal("mutation should not run")
		return nil
	})
	if err == nil {
		t.Fatal("expected missing-file error")
	}
	if changed {
		t.Fatal("expected missing file to not change config")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file to remain missing, got %v", err)
	}
}

func TestApplyToConfigFileOrganizationMismatchDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, validConfigContents)
	before := readFile(t, path)

	changed, err := ApplyToConfigFile(path, "other-org", func(*gitopsconfig.OrganizationConfig) error {
		t.Fatal("mutation should not run")
		return nil
	})
	if err == nil {
		t.Fatal("expected organization mismatch error")
	}
	if changed {
		t.Fatal("expected organization mismatch to not change config")
	}
	assertFileUnchanged(t, path, before)
}

func TestApplyToConfigFileMutationErrorDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, validConfigContents)
	before := readFile(t, path)
	mutationErr := errors.New("mutation failed")

	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(*gitopsconfig.OrganizationConfig) error {
		return mutationErr
	})
	if !errors.Is(err, mutationErr) {
		t.Fatalf("expected mutation error, got %v", err)
	}
	if changed {
		t.Fatal("expected mutation error to not change config")
	}
	assertFileUnchanged(t, path, before)
}

func TestApplyToConfigFileValidationErrorDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, validConfigContents)
	before := readFile(t, path)

	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(cfg *gitopsconfig.OrganizationConfig) error {
		cfg.Members[0].Role = "owner"
		return nil
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if changed {
		t.Fatal("expected validation error to not change config")
	}
	assertFileUnchanged(t, path, before)
}

func TestApplyToConfigFileNoOpDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "# keep me\norganization: ' ORANG-GABOETS '\nmembers: []\n")
	before := readFile(t, path)

	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(*gitopsconfig.OrganizationConfig) error {
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("expected no-op mutation to not change config")
	}
	assertFileUnchanged(t, path, before)
}

func TestApplyToConfigFileOrganizationMutationDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, validConfigContents)
	before := readFile(t, path)

	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(cfg *gitopsconfig.OrganizationConfig) error {
		cfg.Organization = "another-org"
		return nil
	})
	if err == nil {
		t.Fatal("expected organization mismatch error")
	}
	if changed {
		t.Fatal("expected organization mutation to not change config")
	}
	assertFileUnchanged(t, path, before)
}

const validConfigContents = "organization: orang-gaboets\nmembers:\n  - username: octocat\n    role: member\ninvites: []\nrepositories: []\nteams: []\n"

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "organization.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return contents
}

func assertFileUnchanged(t *testing.T, path string, want []byte) {
	t.Helper()
	if got := readFile(t, path); string(got) != string(want) {
		t.Fatalf("config changed after error:\n%s\nwant:\n%s", got, want)
	}
}
