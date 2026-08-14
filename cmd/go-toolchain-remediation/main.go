// Command go-toolchain-remediation provides workflow-facing helpers for safe
// Go toolchain remediation candidate classification, mutation, and verification.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/orang-gaboets/octostate/internal/toolchainremediation"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if err := execute(args, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

func execute(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		return errors.New("expected one of: classify, apply, existing, verify")
	}

	switch args[0] {
	case "classify":
		return runClassify(args[1:], stdout)
	case "apply":
		return runApply(args[1:], stdout)
	case "existing":
		return runExisting(args[1:], stdout)
	case "verify":
		return runVerify(args[1:], stdout)
	default:
		return fmt.Errorf("unknown subcommand %q", args[0])
	}
}

func runExisting(args []string, stdout io.Writer) error {
	var prsPath string
	var repository string
	var expectedBot string
	var expectedBranch string
	var currentVersion string
	var targetVersion string

	fs := flag.NewFlagSet("existing", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&prsPath, "prs", "", "path to open pull request JSON")
	fs.StringVar(&repository, "repository", "", "expected head repository")
	fs.StringVar(&expectedBot, "bot", "", "expected pull request author")
	fs.StringVar(&expectedBranch, "branch", "", "expected pull request head branch")
	fs.StringVar(&currentVersion, "current", "", "current Go toolchain version")
	fs.StringVar(&targetVersion, "target", "", "target Go toolchain version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if prsPath == "" || repository == "" || expectedBot == "" || expectedBranch == "" || currentVersion == "" || targetVersion == "" {
		return errors.New("existing requires --prs, --repository, --bot, --branch, --current, and --target")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("existing does not accept positional arguments: %v", fs.Args())
	}

	file, err := os.Open(prsPath)
	if err != nil {
		return fmt.Errorf("open pull request file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	var prs []toolchainremediation.ExistingPR
	if err := json.NewDecoder(file).Decode(&prs); err != nil {
		return fmt.Errorf("decode pull request JSON: %w", err)
	}
	result, err := toolchainremediation.CheckExistingWork(prs, repository, expectedBot, expectedBranch, currentVersion, targetVersion)
	if err != nil {
		return err
	}
	return writeJSON(stdout, result)
}

func runClassify(args []string, stdout io.Writer) error {
	var goModPath string
	var scanPath string

	fs := flag.NewFlagSet("classify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&goModPath, "go-mod", "", "path to go.mod")
	fs.StringVar(&scanPath, "scan", "", "path to govulncheck JSON stream")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if goModPath == "" || scanPath == "" {
		return errors.New("classify requires --go-mod and --scan")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("classify does not accept positional arguments: %v", fs.Args())
	}

	current, err := currentToolchain(goModPath)
	if err != nil {
		return err
	}
	file, err := os.Open(scanPath)
	if err != nil {
		return fmt.Errorf("open scan file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	result, err := toolchainremediation.Classify(file, current)
	if err != nil {
		return err
	}

	return writeJSON(stdout, struct {
		Eligible         bool     `json:"eligible"`
		CurrentVersion   string   `json:"current_version"`
		TargetVersion    string   `json:"target_version"`
		VulnerabilityIDs []string `json:"vulnerability_ids"`
	}{
		Eligible:         result.Eligible,
		CurrentVersion:   current.String(),
		TargetVersion:    result.TargetVersion,
		VulnerabilityIDs: nonNilStrings(result.VulnerabilityIDs),
	})
}

func runApply(args []string, stdout io.Writer) error {
	var repoRoot string
	var goModPath string
	var developmentDocPath string
	var rawTarget string

	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&repoRoot, "repo-root", "", "path to repository root")
	fs.StringVar(&goModPath, "go-mod", "", "path to go.mod")
	fs.StringVar(&developmentDocPath, "development-doc", "", "path to docs/maintainers/development.md")
	fs.StringVar(&rawTarget, "target", "", "target Go toolchain version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if repoRoot == "" || goModPath == "" || developmentDocPath == "" || rawTarget == "" {
		return errors.New("apply requires --repo-root, --go-mod, --development-doc, and --target")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("apply does not accept positional arguments: %v", fs.Args())
	}

	target, err := parseCLIGoVersion(rawTarget)
	if err != nil {
		return err
	}
	result, err := toolchainremediation.ApplyCandidate(repoRoot, goModPath, developmentDocPath, target)
	if err != nil {
		return err
	}
	return writeJSON(stdout, result)
}

func runVerify(args []string, stdout io.Writer) error {
	var repoRoot string
	var goModPath string
	var developmentDocPath string
	var scanPath string
	var rawTarget string

	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&repoRoot, "repo-root", "", "path to repository root")
	fs.StringVar(&goModPath, "go-mod", "", "path to go.mod")
	fs.StringVar(&developmentDocPath, "development-doc", "", "path to docs/maintainers/development.md")
	fs.StringVar(&scanPath, "scan", "", "path to post-update govulncheck JSON stream")
	fs.StringVar(&rawTarget, "target", "", "target Go toolchain version")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if repoRoot == "" || goModPath == "" || developmentDocPath == "" || scanPath == "" || rawTarget == "" {
		return errors.New("verify requires --repo-root, --go-mod, --development-doc, --scan, and --target")
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("verify does not accept positional arguments: %v", fs.Args())
	}

	target, err := parseCLIGoVersion(rawTarget)
	if err != nil {
		return err
	}
	result, err := toolchainremediation.VerifyCandidate(repoRoot, goModPath, developmentDocPath, scanPath, target)
	if err != nil {
		return err
	}
	return writeJSON(stdout, result)
}

func currentToolchain(goModPath string) (toolchainremediation.GoVersion, error) {
	directives, err := toolchainremediation.ReadDirectives(goModPath)
	if err != nil {
		return toolchainremediation.GoVersion{}, err
	}
	return directives.ToolchainVersion, nil
}

func parseCLIGoVersion(raw string) (toolchainremediation.GoVersion, error) {
	version := strings.TrimSpace(raw)
	if version == "" {
		return toolchainremediation.GoVersion{}, errors.New("empty Go version")
	}
	version = strings.TrimPrefix(version, "go")
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return toolchainremediation.GoVersion{}, fmt.Errorf("invalid Go version %q", raw)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return toolchainremediation.GoVersion{}, fmt.Errorf("invalid Go version %q", raw)
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return toolchainremediation.GoVersion{}, fmt.Errorf("invalid Go version %q", raw)
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return toolchainremediation.GoVersion{}, fmt.Errorf("invalid Go version %q", raw)
	}
	if major <= 0 || minor < 0 || patch < 0 {
		return toolchainremediation.GoVersion{}, fmt.Errorf("invalid Go version %q", raw)
	}
	return toolchainremediation.GoVersion{Major: major, Minor: minor, Patch: patch}, nil
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return fmt.Errorf("write JSON output: %w", err)
	}
	return nil
}
