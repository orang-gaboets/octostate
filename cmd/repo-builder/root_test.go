package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/auth"
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
