package orderedtasks

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
)

type concurrencyTracker struct {
	mu      sync.Mutex
	current int
	max     int
}

func (t *concurrencyTracker) Start() func() {
	t.mu.Lock()
	t.current++
	if t.current > t.max {
		t.max = t.current
	}
	t.mu.Unlock()

	return func() {
		t.mu.Lock()
		t.current--
		t.mu.Unlock()
	}
}

func (t *concurrencyTracker) Snapshot() (current, maxSeen int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.current, t.max
}

func waitForTrackerMaxAtLeast(t *testing.T, tracker *concurrencyTracker, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		_, maxSeen := tracker.Snapshot()
		if maxSeen >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}

	current, maxSeen := tracker.Snapshot()
	t.Fatalf("timed out waiting for max concurrency >= %d; current=%d max=%d", want, current, maxSeen)
}

func waitForSignals(t *testing.T, ch <-chan string, count int) []string {
	t.Helper()

	values := make([]string, 0, count)
	deadline := time.After(2 * time.Second)
	for len(values) < count {
		select {
		case value := <-ch:
			values = append(values, value)
		case <-deadline:
			t.Fatalf("timed out waiting for %d signals; got %#v", count, values)
		}
	}
	return values
}

func TestRunBoundsConcurrency(t *testing.T) {
	t.Parallel()

	tracker := &concurrencyTracker{}
	release := make(chan struct{})
	tasks := make([]Task, 0, 6)
	for range 6 {
		tasks = append(tasks, func(context.Context) error {
			done := tracker.Start()
			defer done()
			<-release
			return nil
		})
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), 2, tasks)
	}()

	waitForTrackerMaxAtLeast(t, tracker, 2)
	close(release)

	if err := <-errCh; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	_, maxSeen := tracker.Snapshot()
	if maxSeen > 2 {
		t.Fatalf("expected max concurrency <= 2, got %d", maxSeen)
	}
}

func TestRunReturnsFirstErrorByTaskOrder(t *testing.T) {
	t.Parallel()

	firstErr := errors.New("first task failed")
	secondErr := errors.New("second task failed")

	err := Run(context.Background(), 2, []Task{
		func(context.Context) error {
			time.Sleep(20 * time.Millisecond)
			return firstErr
		},
		func(context.Context) error {
			return secondErr
		},
	})
	if !errors.Is(err, firstErr) {
		t.Fatalf("unexpected error: got %v want %v", err, firstErr)
	}
}

func TestRunCancelsSiblingTasksOnError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("task failed")
	started := make(chan string, 3)
	canceled := make(chan string, 2)
	releaseError := make(chan struct{})

	blockUntilCanceled := func(name string) Task {
		return func(ctx context.Context) error {
			started <- name
			<-ctx.Done()
			canceled <- name
			return ctx.Err()
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- Run(context.Background(), 3, []Task{
			blockUntilCanceled("first"),
			blockUntilCanceled("second"),
			func(context.Context) error {
				started <- "error"
				<-releaseError
				return wantErr
			},
		})
	}()

	gotStarted := waitForSignals(t, started, 3)
	slices.Sort(gotStarted)
	if !reflect.DeepEqual(gotStarted, []string{"error", "first", "second"}) {
		t.Fatalf("unexpected started tasks: got %#v", gotStarted)
	}

	close(releaseError)

	err := <-errCh
	if !errors.Is(err, wantErr) {
		t.Fatalf("unexpected error: got %v want %v", err, wantErr)
	}

	gotCanceled := waitForSignals(t, canceled, 2)
	slices.Sort(gotCanceled)
	if !reflect.DeepEqual(gotCanceled, []string{"first", "second"}) {
		t.Fatalf("unexpected canceled tasks: got %#v", gotCanceled)
	}
}

func TestRunTreatsUnexpectedContextErrorAsFailure(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), 2, []Task{
		func(context.Context) error {
			return context.Canceled
		},
		func(context.Context) error {
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: got %v want %v", err, context.Canceled)
	}
}
