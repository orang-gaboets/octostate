package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestSplitRepositoryRejectsMalformedInput(t *testing.T) {
	t.Parallel()

	for _, repository := range []string{"", "octostate", "/name", "owner/", "   ", "owner/repo/extra", "owner//name"} {
		if _, _, err := splitRepository(repository); err == nil {
			t.Fatalf("expected %q to be rejected", repository)
		}
	}
}

func TestSplitRepositoryParsesOwnerAndName(t *testing.T) {
	t.Parallel()

	owner, name, err := splitRepository("  orang-gaboets/octostate  ")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "orang-gaboets" || name != "octostate" {
		t.Fatalf("owner=%q name=%q", owner, name)
	}
}

func TestRunReportsFlagErrorsOnStderr(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	if code := run([]string{"-unknown-flag"}, &stdout, &stderr); code == 0 {
		t.Fatal("expected a non-zero exit code")
	}
	if stdout.Len() != 0 {
		t.Fatalf("diagnostics must not go to stdout, got %q", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "Error: ") {
		t.Fatalf("expected an error on stderr, got %q", stderr.String())
	}
}

func TestRunReportsMalformedRepositoryWithoutContactingGitHub(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	if code := run([]string{"-repository", "not-a-repo"}, &stdout, &stderr); code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "owner/name") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
