package main

import (
	"errors"
	"testing"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/exitcode"
)

func TestRunReturnsZeroWhenExecuteSucceeds(t *testing.T) {
	restore := setMainTestHooks(
		func() error { return nil },
		func(any) { t.Fatal("checkErrFn should not be called for successful execution") },
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
		func(any) { t.Fatal("checkErrFn should not be called for typed exit errors") },
		func(int) { t.Fatal("exitFn should not be called by run") },
	)
	defer restore()

	code := run()
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d", code)
	}
}

func TestRunFallsBackToCheckErrForRegularErrors(t *testing.T) {
	sentinel := errors.New("boom")

	called := false
	restore := setMainTestHooks(
		func() error { return sentinel },
		func(msg any) {
			called = true
			err, ok := msg.(error)
			if !ok {
				t.Fatalf("expected checkErrFn to receive error, got %T", msg)
			}
			if !errors.Is(err, sentinel) {
				t.Fatalf("expected checkErrFn to receive sentinel error, got %v", err)
			}
		},
		func(int) { t.Fatal("exitFn should not be called by run") },
	)
	defer restore()

	code := run()
	if !called {
		t.Fatalf("expected checkErrFn to be called for regular errors")
	}
	if code != 1 {
		t.Fatalf("expected fallback exit code 1, got %d", code)
	}
}

func setMainTestHooks(
	execute func() error,
	checkErr func(any),
	exit func(int),
) func() {
	originalExecute := executeFn
	originalCheckErr := checkErrFn
	originalExit := exitFn

	executeFn = execute
	checkErrFn = checkErr
	exitFn = exit

	return func() {
		executeFn = originalExecute
		checkErrFn = originalCheckErr
		exitFn = originalExit
	}
}
