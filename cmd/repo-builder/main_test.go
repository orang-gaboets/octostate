package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/exitcode"
)

func TestRunReturnsZeroWhenExecuteSucceeds(t *testing.T) {
	restore := setMainTestHooks(
		func() error { return nil },
		func() io.Writer { t.Fatal("stderrFn should not be called for successful execution"); return nil },
		func(int) { t.Fatal("exitFn should not be called by run") },
	)
	defer restore()

	code := run()
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}
}

func TestRunReturnsTypedExitCode(t *testing.T) {
	restore := setMainTestHooks(
		func() error { return exitcode.New(2, errors.New("invalid config")) },
		func() io.Writer { t.Fatal("stderrFn should not be called for typed exit errors"); return nil },
		func(int) { t.Fatal("exitFn should not be called by run") },
	)
	defer restore()

	code := run()
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRunFallsBackToExitCodeOneForRegularErrors(t *testing.T) {
	sentinel := errors.New("boom")

	var stderr bytes.Buffer
	restore := setMainTestHooks(
		func() error { return sentinel },
		func() io.Writer { return &stderr },
		func(int) { t.Fatal("exitFn should not be called by run") },
	)
	defer restore()

	code := run()
	if code != 1 {
		t.Fatalf("expected fallback exit code 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Error: boom") {
		t.Fatalf("expected stderr to contain error line, got %q", stderr.String())
	}
}

func TestRunWithCobraRequiredFlagErrorPrintsSingleLine(t *testing.T) {
	restore := setMainTestHooks(
		func() error {
			cmd := newRootCmd()
			cmd.SetArgs([]string{"config", "validate"})
			return cmd.Execute()
		},
		func() io.Writer { return &bytes.Buffer{} },
		func(int) { t.Fatal("exitFn should not be called by run") },
	)
	defer restore()

	var stderr bytes.Buffer
	stderrRestore := setMainStderr(&stderr)
	defer stderrRestore()

	code := run()
	if code != 1 {
		t.Fatalf("expected fallback exit code 1, got %d", code)
	}

	line := strings.TrimSpace(stderr.String())
	if line != `Error: required flag(s) "config-dir" not set` {
		t.Fatalf("unexpected stderr line: %q", line)
	}
}

func setMainTestHooks(
	execute func() error,
	stderr func() io.Writer,
	exit func(int),
) func() {
	originalExecute := executeFn
	originalStderr := stderrFn
	originalExit := exitFn

	executeFn = execute
	stderrFn = stderr
	exitFn = exit

	return func() {
		executeFn = originalExecute
		stderrFn = originalStderr
		exitFn = originalExit
	}
}

func setMainStderr(w io.Writer) func() {
	originalStderr := stderrFn
	stderrFn = func() io.Writer { return w }
	return func() {
		stderrFn = originalStderr
	}
}
