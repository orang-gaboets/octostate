package main

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/google/go-github/v88/github"

	"github.com/orang-gaboets/octostate/pkg/github"
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

// A server whose Link header always points back at page 2 would otherwise be
// polled forever; the list must stop after reading page 2 a single time.
func TestListContributorsRejectsNonAdvancingPagination(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 10 {
			http.Error(w, "pagination did not stop", http.StatusTeapot)
			return
		}
		w.Header().Set("Link", `<http://`+r.Host+r.URL.Path+`?page=2>; rel="next"`)
		_, _ = w.Write([]byte(`[{"login":"alice","type":"User"}]`))
	}))
	t.Cleanup(server.Close)

	client, err := gh.NewClient(gh.WithURLs(gh.Ptr(server.URL+"/"), nil))
	if err != nil {
		t.Fatal(err)
	}

	got, err := listContributors(context.Background(), client, "acme", "octostate")
	if !errors.Is(err, github.ErrValidationFailed) {
		t.Fatalf("error = %v, want %v", err, github.ErrValidationFailed)
	}
	if got != nil || requests != 2 {
		t.Fatalf("got %d contributors after %d requests, want none after 2", len(got), requests)
	}
}
