// Package orderedtasks runs bounded concurrent work while preserving
// deterministic error selection by task order.
package orderedtasks

import (
	"context"
	"errors"

	"golang.org/x/sync/errgroup"
)

// Task is one bounded concurrent unit of work.
type Task func(context.Context) error

// Run executes tasks with bounded concurrency while preserving deterministic
// first-error selection by task order.
func Run(ctx context.Context, limit int, tasks []Task) error {
	if len(tasks) == 0 {
		return nil
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, groupCtx := errgroup.WithContext(runCtx)
	g.SetLimit(normalizeLimit(limit))
	errs := make([]error, len(tasks))

	for i, task := range tasks {
		if task == nil {
			continue
		}

		index := i
		run := task
		g.Go(func() error {
			err := run(groupCtx)
			switch {
			case err == nil:
				return nil
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				if runCtx.Err() != nil || groupCtx.Err() != nil {
					return nil
				}
				errs[index] = err
				cancel()
				return nil
			default:
				errs[index] = err
				cancel()
				return nil
			}
		})
	}

	_ = g.Wait() //nolint:errcheck // task errors are captured in errs and reported deterministically below
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 1
	}
	return limit
}
