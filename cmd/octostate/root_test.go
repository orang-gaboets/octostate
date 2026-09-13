package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/auth"
)

func TestRootVerboseWritesLogsToStderrAndKeepsJSONOnStdout(t *testing.T) {
	auth.PrepareClient(t)

	cmd := newRootCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--verbose", "organization", "get-by-name", "--token", "t", "--org", "o"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdout := strings.TrimSpace(out.String())
	stderr := strings.TrimSpace(errBuf.String())

	if stdout == "" || !strings.HasPrefix(stdout, "{") {
		t.Fatalf("expected JSON object on stdout, got %q", stdout)
	}
	if stderr == "" {
		t.Fatalf("expected verbose logs on stderr, got empty output")
	}
	if !strings.Contains(stderr, "verbose:") {
		t.Fatalf("expected verbose prefix in stderr, got %q", stderr)
	}
}

func TestRootWithoutVerboseDoesNotWriteDiagnosticLogs(t *testing.T) {
	auth.PrepareClient(t)

	cmd := newRootCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"organization", "get-by-name", "--token", "t", "--org", "o"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stdout := strings.TrimSpace(out.String())
	stderr := strings.TrimSpace(errBuf.String())

	if stdout == "" || !strings.HasPrefix(stdout, "{") {
		t.Fatalf("expected JSON object on stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected no diagnostic stderr output without --verbose, got %q", stderr)
	}
}

func TestRootVersionPrintsInjectedReleaseVersionWithoutAuthentication(t *testing.T) {
	originalBuildVersion := buildVersion
	buildVersion = "v1.4.0"
	defer func() { buildVersion = originalBuildVersion }()

	cmd := newRootCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.String(); got != "octostate v1.4.0\n" {
		t.Fatalf("version output = %q, want %q", got, "octostate v1.4.0\n")
	}
	if errBuf.Len() != 0 {
		t.Fatalf("version command wrote diagnostics to stderr: %q", errBuf.String())
	}
	if flag := cmd.Flag("version"); flag == nil || flag.Shorthand != "" {
		t.Fatalf("version flag = %#v, want a long-only flag", flag)
	}
}

func TestRootShortVerboseFlagRemainsVerbose(t *testing.T) {
	auth.PrepareClient(t)

	cmd := newRootCmd()
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"-v", "organization", "get-by-name", "--token", "t", "--org", "o"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(errBuf.String(), "verbose:") {
		t.Fatalf("expected -v to enable verbose diagnostics, got %q", errBuf.String())
	}
}
