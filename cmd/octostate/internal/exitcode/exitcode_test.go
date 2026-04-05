package exitcode

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodeWithTypedError(t *testing.T) {
	t.Parallel()

	err := New(2, errors.New("drift detected"))
	got, ok := Code(err)
	if !ok {
		t.Fatalf("expected typed exit error")
	}
	if got != 2 {
		t.Fatalf("expected exit code 2, got %d", got)
	}
}

func TestCodeWithWrappedTypedError(t *testing.T) {
	t.Parallel()

	base := New(7, errors.New("wrapped"))
	err := fmt.Errorf("outer: %w", base)

	got, ok := Code(err)
	if !ok {
		t.Fatalf("expected wrapped typed error to be recognized")
	}
	if got != 7 {
		t.Fatalf("expected exit code 7, got %d", got)
	}
}

func TestCodeWithNonTypedError(t *testing.T) {
	t.Parallel()

	got, ok := Code(errors.New("normal error"))
	if ok {
		t.Fatalf("expected non-typed error")
	}
	if got != 0 {
		t.Fatalf("expected zero code for non-typed error, got %d", got)
	}
}

func TestCodeWithZeroTypedCodeDefaultsToOne(t *testing.T) {
	t.Parallel()

	err := New(0, errors.New("invalid"))
	got, ok := Code(err)
	if !ok {
		t.Fatalf("expected typed exit error")
	}
	if got != 1 {
		t.Fatalf("expected exit code 1, got %d", got)
	}
}

func TestCodeWithManualNegativeTypedCodeDefaultsToOne(t *testing.T) {
	t.Parallel()

	err := &Error{
		Code: -9,
		Err:  errors.New("invalid"),
	}

	got, ok := Code(err)
	if !ok {
		t.Fatalf("expected typed exit error")
	}
	if got != 1 {
		t.Fatalf("expected exit code 1, got %d", got)
	}
}
