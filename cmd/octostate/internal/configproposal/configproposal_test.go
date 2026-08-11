package configproposal

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
			t.Fatalf("unexpected file mode: got %v want %v", got, want)
		}
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
	var loadErr *gitopsconfig.LoadError
	if !errors.As(err, &loadErr) {
		t.Fatalf("expected load error, got %T", err)
	}
	if loadErr.Kind != gitopsconfig.LoadErrorMissingFile {
		t.Fatalf("expected missing-file load error, got %q", loadErr.Kind)
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
	if !strings.Contains(err.Error(), `members[0].role (invalid_enum): organization member role "owner" is not supported`) {
		t.Fatalf("expected formatted validation error, got %v", err)
	}
	if changed {
		t.Fatal("expected validation error to not change config")
	}
	assertFileUnchanged(t, path, before)
}

func TestApplyToConfigFileLoadedValidationErrorDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "organization: orang-gaboets\nmembers:\n  - username: octocat\n    role: owner\ninvites: []\nrepositories: []\nteams: []\n")
	before := readFile(t, path)

	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(*gitopsconfig.OrganizationConfig) error {
		t.Fatal("mutation should not run")
		return nil
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), `members[0].role (invalid_enum): organization member role "owner" is not supported`) {
		t.Fatalf("expected formatted validation error, got %v", err)
	}
	if changed {
		t.Fatal("expected validation error to not change config")
	}
	assertFileUnchanged(t, path, before)
}

func TestApplyToConfigFileLoadedOwnershipValidationErrorDoesNotMutateOrWrite(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, "organization: orang-gaboets\nmembers: []\ninvites: []\nrepositories:\n  - owner: shared-platform\n    name: octostate\n    visibility: private\nteams: []\n")
	before := readFile(t, path)

	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(*gitopsconfig.OrganizationConfig) error {
		t.Fatal("mutation should not run")
		return nil
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), `validate loaded config: repositories[0].owner (repository_owner_scope): repository owner "shared-platform" must match organization "orang-gaboets"`) {
		t.Fatalf("expected repository owner scope detail, got %v", err)
	}
	if changed {
		t.Fatal("expected validation error to not change config")
	}
	assertFileUnchanged(t, path, before)
}

func TestApplyToConfigFileMutatedOwnershipValidationErrorDoesNotWrite(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, validConfigContents)
	before := readFile(t, path)

	changed, err := ApplyToConfigFile(path, "orang-gaboets", func(cfg *gitopsconfig.OrganizationConfig) error {
		cfg.Repositories = append(cfg.Repositories, gitopsconfig.RepositorySpec{
			Owner:      "shared-platform",
			Name:       "octostate",
			Visibility: "private",
		})
		return nil
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), `validate mutated config: repositories[0].owner (repository_owner_scope): repository owner "shared-platform" must match organization "orang-gaboets"`) {
		t.Fatalf("expected repository owner scope detail, got %v", err)
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

func TestApplyToConfigFileRejectsSymlinkTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "target.yaml")
	linkPath := filepath.Join(dir, "organization.yaml")
	before := []byte("organization: orang-gaboets\nmembers:\n  - username: octocat\n    role: member\n")

	if err := os.WriteFile(targetPath, before, 0o600); err != nil {
		t.Fatal(err)
	}
	mustCreateSymlink(t, targetPath, linkPath)

	changed, err := ApplyToConfigFile(linkPath, "orang-gaboets", func(*gitopsconfig.OrganizationConfig) error {
		t.Fatal("mutation should not run")
		return nil
	})
	if err == nil {
		t.Fatal("expected symlink target error")
	}
	if changed {
		t.Fatal("expected rejected symlink to not change config")
	}
	if got, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("stat symlink: %v", err)
	} else if got.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected symlink to remain, got mode %v", got.Mode())
	}
	if got := readFile(t, targetPath); string(got) != string(before) {
		t.Fatalf("symlink target changed:\n%s\nwant:\n%s", got, before)
	}
	assertNoTempFiles(t, dir)
}

func TestApplyToConfigFileRejectsDirectoryTarget(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	targetPath := filepath.Join(dir, "organization.yaml")
	if err := os.Mkdir(targetPath, 0o755); err != nil {
		t.Fatal(err)
	}

	changed, err := ApplyToConfigFile(targetPath, "orang-gaboets", func(*gitopsconfig.OrganizationConfig) error {
		t.Fatal("mutation should not run")
		return nil
	})
	if err == nil {
		t.Fatal("expected directory target error")
	}
	if changed {
		t.Fatal("expected rejected directory to not change config")
	}
	assertNoTempFiles(t, dir)
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

func mustCreateSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.Errno(1314)) || os.IsPermission(err) {
			t.Skipf("symlink creation unavailable: %v", err)
		}
		t.Fatalf("create symlink: %v", err)
	}
}

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".organization.yaml-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("unexpected temp files left behind: %v", matches)
	}
}
