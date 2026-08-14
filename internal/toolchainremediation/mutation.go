package toolchainremediation

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const developmentDocSnippet = "" +
	"   The module includes a `toolchain %s` directive, so Go commands will\n" +
	"   automatically prefer the patched %s toolchain when toolchain switching\n" +
	"   is enabled."

// MutationResult reports the exact files changed by ApplyCandidate.
type MutationResult struct {
	CurrentVersion string   `json:"current_version"`
	TargetVersion  string   `json:"target_version"`
	ChangedFiles   []string `json:"changed_files"`
}

// VerificationResult reports the exact files validated by VerifyCandidate.
type VerificationResult struct {
	Verified      bool     `json:"verified"`
	TargetVersion string   `json:"target_version"`
	ChangedFiles  []string `json:"changed_files"`
}

type goModState struct {
	goToken          string
	goVersion        GoVersion
	toolchain        GoVersion
	toolchainValueLo int
	toolchainValueHi int
}

// Directives exposes the validated go and toolchain directives from go.mod.
type Directives struct {
	GoDirective      string
	ToolchainVersion GoVersion
}

// ApplyCandidate updates only the pinned toolchain directive in go.mod and the
// designated documentation snippet that duplicates that version.
func ApplyCandidate(repoRoot, goModPath, developmentDocPath string, target GoVersion) (MutationResult, error) {
	repoRoot, goModPath, developmentDocPath, err := resolveApplyPaths(repoRoot, goModPath, developmentDocPath)
	if err != nil {
		return MutationResult{}, err
	}

	goModData, err := readRegularFile(goModPath)
	if err != nil {
		return MutationResult{}, err
	}
	state, err := parseGoMod(goModData)
	if err != nil {
		return MutationResult{}, err
	}
	if target.Major != state.toolchain.Major || target.Minor != state.toolchain.Minor {
		return MutationResult{}, fmt.Errorf("target version %s crosses the current Go minor line", target.String())
	}
	if target.String() == state.toolchain.String() {
		return MutationResult{}, fmt.Errorf("target version %s matches the current toolchain", target.String())
	}

	docData, err := readRegularFile(developmentDocPath)
	if err != nil {
		return MutationResult{}, err
	}

	nextGoMod := replaceRange(goModData, state.toolchainValueLo, state.toolchainValueHi, []byte(target.String()))
	nextDoc, err := replaceDevelopmentDocSnippet(docData, state.toolchain, target)
	if err != nil {
		return MutationResult{}, err
	}

	if err := os.WriteFile(goModPath, nextGoMod, 0o644); err != nil {
		return MutationResult{}, fmt.Errorf("write go.mod: %w", err)
	}
	if err := os.WriteFile(developmentDocPath, nextDoc, 0o644); err != nil {
		return MutationResult{}, fmt.Errorf("write development doc: %w", err)
	}

	return MutationResult{
		CurrentVersion: state.toolchain.String(),
		TargetVersion:  target.String(),
		ChangedFiles:   changedFiles(repoRoot, goModPath, developmentDocPath),
	}, nil
}

// ReadDirectives parses go.mod and returns the validated directives.
func ReadDirectives(goModPath string) (Directives, error) {
	goModData, err := readRegularFile(goModPath)
	if err != nil {
		return Directives{}, err
	}
	state, err := parseGoMod(goModData)
	if err != nil {
		return Directives{}, err
	}
	return Directives{
		GoDirective:      state.goToken,
		ToolchainVersion: state.toolchain,
	}, nil
}

// VerifyCandidate confirms the candidate diff is exact, the toolchain target
// remains pinned as expected, and the post-update structured scan is clean.
func VerifyCandidate(repoRoot, goModPath, developmentDocPath, scanPath string, target GoVersion) (VerificationResult, error) {
	repoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("resolve repo root: %w", err)
	}
	goModPath, err = filepath.Abs(goModPath)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("resolve go.mod path: %w", err)
	}
	developmentDocPath, err = filepath.Abs(developmentDocPath)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("resolve development doc path: %w", err)
	}
	scanPath, err = filepath.Abs(scanPath)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("resolve scan path: %w", err)
	}

	if err := verifyStatus(repoRoot, goModPath, developmentDocPath); err != nil {
		return VerificationResult{}, err
	}

	headGoMod, err := gitShow(repoRoot, goModPath)
	if err != nil {
		return VerificationResult{}, err
	}
	headDoc, err := gitShow(repoRoot, developmentDocPath)
	if err != nil {
		return VerificationResult{}, err
	}
	workGoMod, err := readRegularFile(goModPath)
	if err != nil {
		return VerificationResult{}, err
	}
	workDoc, err := readRegularFile(developmentDocPath)
	if err != nil {
		return VerificationResult{}, err
	}

	headState, err := parseGoMod(headGoMod)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("parse committed go.mod: %w", err)
	}
	workState, err := parseGoMod(workGoMod)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("parse candidate go.mod: %w", err)
	}
	if headState.goToken != workState.goToken {
		return VerificationResult{}, errors.New("go directive changed")
	}
	if workState.toolchain != target {
		return VerificationResult{}, fmt.Errorf("candidate toolchain does not match expected target %s", target.String())
	}

	expectedGoMod := replaceRange(headGoMod, headState.toolchainValueLo, headState.toolchainValueHi, []byte(target.String()))
	if !bytes.Equal(workGoMod, expectedGoMod) {
		return VerificationResult{}, errors.New("unexpected go.mod diff")
	}

	expectedDoc, err := replaceDevelopmentDocSnippet(headDoc, headState.toolchain, target)
	if err != nil {
		return VerificationResult{}, fmt.Errorf("rebuild expected development doc: %w", err)
	}
	if !bytes.Equal(workDoc, expectedDoc) {
		return VerificationResult{}, errors.New("unexpected development doc diff")
	}

	if err := validateCleanScan(scanPath, target); err != nil {
		return VerificationResult{}, err
	}

	return VerificationResult{
		Verified:      true,
		TargetVersion: target.String(),
		ChangedFiles:  changedFiles(repoRoot, goModPath, developmentDocPath),
	}, nil
}

func parseGoMod(data []byte) (goModState, error) {
	lines := splitLines(data)

	var state goModState
	sawGo := false
	sawToolchain := false
	offset := 0

	for _, rawLine := range lines {
		lineText := strings.TrimSuffix(string(rawLine), "\n")
		commentless := stripLineComment(lineText)
		fields := strings.Fields(commentless)
		if len(fields) == 0 {
			offset += len(rawLine)
			continue
		}

		switch fields[0] {
		case "go":
			if sawGo {
				return goModState{}, errors.New("duplicate go directive")
			}
			if len(fields) != 2 {
				return goModState{}, errors.New("invalid go directive")
			}
			version, err := parseGoDirectiveVersion(fields[1])
			if err != nil {
				return goModState{}, fmt.Errorf("invalid go directive: %w", err)
			}
			state.goToken = fields[1]
			state.goVersion = version
			sawGo = true
		case "toolchain":
			if sawToolchain {
				return goModState{}, errors.New("duplicate toolchain directive")
			}
			if len(fields) != 2 {
				return goModState{}, errors.New("invalid toolchain directive")
			}
			version, err := parseToolchainDirectiveVersion(fields[1])
			if err != nil {
				return goModState{}, fmt.Errorf("invalid toolchain directive: %w", err)
			}
			valueIndex := strings.Index(commentless, fields[1])
			if valueIndex < 0 {
				return goModState{}, errors.New("could not locate toolchain directive value")
			}
			state.toolchain = version
			state.toolchainValueLo = offset + valueIndex
			state.toolchainValueHi = state.toolchainValueLo + len(fields[1])
			sawToolchain = true
		}

		offset += len(rawLine)
	}

	if !sawGo {
		return goModState{}, errors.New("missing go directive")
	}
	if !sawToolchain {
		return goModState{}, errors.New("missing toolchain directive")
	}

	return state, nil
}

func parseGoDirectiveVersion(raw string) (GoVersion, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "go") {
		return GoVersion{}, fmt.Errorf("language version must not use go prefix: %q", raw)
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return GoVersion{}, fmt.Errorf("expected major.minor.patch language version, got %q", raw)
	}
	return parseVersion(strings.Join(parts, "."))
}

func parseToolchainDirectiveVersion(raw string) (GoVersion, error) {
	s := strings.TrimSpace(raw)
	if !strings.HasPrefix(s, "go") {
		return GoVersion{}, fmt.Errorf("toolchain version must use go prefix: %q", raw)
	}
	return parseVersion(s)
}

func replaceDevelopmentDocSnippet(data []byte, current, target GoVersion) ([]byte, error) {
	oldSnippet := []byte(fmt.Sprintf(developmentDocSnippet, current.String(), numericVersion(current)))
	newSnippet := []byte(fmt.Sprintf(developmentDocSnippet, target.String(), numericVersion(target)))
	count := bytes.Count(data, oldSnippet)
	if count != 1 {
		return nil, fmt.Errorf("documentation replacement match count = %d, want 1", count)
	}
	return bytes.Replace(data, oldSnippet, newSnippet, 1), nil
}

func numericVersion(v GoVersion) string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func replaceRange(data []byte, lo, hi int, replacement []byte) []byte {
	next := make([]byte, 0, len(data)-hi+lo+len(replacement))
	next = append(next, data[:lo]...)
	next = append(next, replacement...)
	next = append(next, data[hi:]...)
	return next
}

func changedFiles(repoRoot string, paths ...string) []string {
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			out = append(out, filepath.ToSlash(path))
			continue
		}
		out = append(out, filepath.ToSlash(rel))
	}
	sort.Strings(out)
	return out
}

func readRegularFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing symlink path %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("expected regular file at %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func resolveApplyPaths(repoRoot, goModPath, developmentDocPath string) (string, string, string, error) {
	if err := rejectOriginalSymlinkPath(repoRoot, goModPath); err != nil {
		return "", "", "", err
	}
	if err := rejectOriginalSymlinkPath(repoRoot, developmentDocPath); err != nil {
		return "", "", "", err
	}

	canonicalRoot, err := canonicalRepositoryRoot(repoRoot)
	if err != nil {
		return "", "", "", err
	}

	expectedGoMod := filepath.Join(canonicalRoot, "go.mod")
	expectedDoc := filepath.Join(canonicalRoot, "docs", "maintainers", "development.md")

	canonicalGoMod, err := canonicalRegularPath(goModPath)
	if err != nil {
		return "", "", "", err
	}
	canonicalDoc, err := canonicalRegularPath(developmentDocPath)
	if err != nil {
		return "", "", "", err
	}
	if canonicalGoMod != expectedGoMod {
		return "", "", "", fmt.Errorf("path %s is outside the apply allowlist", goModPath)
	}
	if canonicalDoc != expectedDoc {
		return "", "", "", fmt.Errorf("path %s is outside the apply allowlist", developmentDocPath)
	}

	return canonicalRoot, canonicalGoMod, canonicalDoc, nil
}

func rejectOriginalSymlinkPath(repoRoot, path string) error {
	if hasParentTraversalComponent(path) {
		return fmt.Errorf("refusing parent traversal path %s", path)
	}

	rootAbs, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("resolve repository root for symlink check: %w", err)
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path for symlink check %s: %w", path, err)
	}
	rel, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return fmt.Errorf("relate path %s to repository root: %w", path, err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil
	}

	current := rootAbs
	if err := rejectSymlinkEntry(current); err != nil {
		return err
	}
	for _, component := range strings.Split(rel, string(os.PathSeparator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		if err := rejectSymlinkEntry(current); err != nil {
			return err
		}
	}
	return nil
}

func hasParentTraversalComponent(path string) bool {
	trimmed := strings.TrimPrefix(path, filepath.VolumeName(path))
	components := strings.Split(trimmed, string(os.PathSeparator))
	if os.PathSeparator == '\\' {
		components = strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == '\\' || r == '/'
		})
	}
	for _, component := range components {
		if component == ".." {
			return true
		}
	}
	return false
}

func rejectSymlinkEntry(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlink path %s", path)
	}
	return nil
}

func canonicalRepositoryRoot(path string) (string, error) {
	if path == "" {
		return "", errors.New("repository root path is required")
	}
	if filepath.IsAbs(path) && path != filepath.Clean(path) {
		return "", fmt.Errorf("repository root path must be canonical: %s", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve repository root: %w", err)
	}
	evaluated, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve repository root symlinks: %w", err)
	}
	info, err := os.Lstat(evaluated)
	if err != nil {
		return "", fmt.Errorf("stat repository root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository root is not a directory: %s", path)
	}
	return evaluated, nil
}

func canonicalRegularPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %s: %w", path, err)
	}
	evaluated, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("resolve path symlinks %s: %w", path, err)
	}
	if _, err := readRegularFile(evaluated); err != nil {
		return "", err
	}
	return evaluated, nil
}

func splitLines(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	lines := bytes.SplitAfter(data, []byte("\n"))
	if len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func stripLineComment(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		return line[:idx]
	}
	return line
}

func verifyStatus(repoRoot, goModPath, developmentDocPath string) error {
	cmd, err := gitCommand(repoRoot, "status", "--porcelain=v2", "--untracked-files=all")
	if err != nil {
		return err
	}
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}

	allowed := map[string]struct{}{}
	for _, path := range changedFiles(repoRoot, goModPath, developmentDocPath) {
		allowed[path] = struct{}{}
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(allowed) {
		if strings.TrimSpace(string(out)) == "" {
			return errors.New("candidate tree has no changes to verify")
		}
	}

	seen := map[string]struct{}{}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		switch line[0] {
		case '1':
			fields := strings.Fields(line)
			if len(fields) < 9 {
				return fmt.Errorf("unrecognized git status entry %q", line)
			}
			xy := fields[1]
			if xy != ".M" {
				return fmt.Errorf("unexpected repository change %q", line)
			}
			if fields[3] != fields[5] || fields[4] != fields[5] {
				return fmt.Errorf("unexpected mode change %q", line)
			}
			path := filepath.ToSlash(fields[8])
			if _, ok := allowed[path]; !ok {
				return fmt.Errorf("unexpected modified path %s", path)
			}
			seen[path] = struct{}{}
		case '?', '2', 'u', '!':
			return fmt.Errorf("unexpected repository change %q", line)
		default:
			return fmt.Errorf("unrecognized git status entry %q", line)
		}
	}
	if len(seen) != len(allowed) {
		return errors.New("candidate tree does not contain the exact expected changed files")
	}
	return nil
}

func gitShow(repoRoot, path string) ([]byte, error) {
	rel, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return nil, fmt.Errorf("resolve path %s relative to repo: %w", path, err)
	}
	if strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("path %s is outside repository root %s", path, repoRoot)
	}
	cmd, err := gitCommand(repoRoot, "show", "HEAD:"+filepath.ToSlash(rel))
	if err != nil {
		return nil, err
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git show %s: %w", rel, err)
	}
	return out, nil
}

func gitCommand(repoRoot string, args ...string) (*exec.Cmd, error) {
	gitDir, err := resolveGitDir(repoRoot)
	if err != nil {
		return nil, err
	}
	fullArgs := append([]string{"--git-dir=" + gitDir, "--work-tree=" + repoRoot}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = repoRoot
	cmd.Env = filteredGitEnvironment(os.Environ())
	return cmd, nil
}

func resolveGitDir(repoRoot string) (string, error) {
	dotGit := filepath.Join(repoRoot, ".git")
	info, err := os.Lstat(dotGit)
	if err != nil {
		return "", fmt.Errorf("stat repository metadata: %w", err)
	}
	if info.IsDir() {
		return dotGit, nil
	}
	data, err := os.ReadFile(dotGit)
	if err != nil {
		return "", fmt.Errorf("read repository metadata: %w", err)
	}
	line := strings.TrimSpace(string(data))
	const prefix = "gitdir: "
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unsupported .git file contents %q", line)
	}
	gitDir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Clean(filepath.Join(repoRoot, gitDir))
	}
	return gitDir, nil
}

func filteredGitEnvironment(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		if strings.HasPrefix(entry, "GIT_") {
			continue
		}
		out = append(out, entry)
	}
	return out
}

func validateCleanScan(scanPath string, target GoVersion) error {
	data, err := readRegularFile(scanPath)
	if err != nil {
		return err
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	first, err := decodeMessage(dec)
	if err != nil {
		return fmt.Errorf("decode post-update scan: %w", err)
	}
	if first.Config == nil {
		return errors.New("post-update scan must start with a config message")
	}
	if err := validateConfig(first.Config); err != nil {
		return fmt.Errorf("validate post-update scan config: %w", err)
	}
	if first.Config.GoVersion != "" {
		version, err := parseVersion(first.Config.GoVersion)
		if err != nil {
			return fmt.Errorf("invalid post-update scan Go version %q: %w", first.Config.GoVersion, err)
		}
		if version != target {
			return fmt.Errorf("post-update scan Go version %s does not match expected target %s", version.String(), target.String())
		}
	}

	for {
		msg, err := decodeMessage(dec)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("decode post-update scan: %w", err)
		}
		if msg.Config != nil {
			return errors.New("post-update scan config message must appear exactly once at the start of the stream")
		}
		if msg.Finding != nil {
			return errors.New("post-update scan still reports findings")
		}
	}
}
