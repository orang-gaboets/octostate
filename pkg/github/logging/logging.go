package logging

import (
	"context"
	"io"
	"log"
)

type contextKey struct{}

// WithVerbose returns a context that enables diagnostic logging when verbose is true.
// When verbose is false, the returned context does not carry a logger.
func WithVerbose(ctx context.Context, verbose bool, w io.Writer) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if !verbose || w == nil {
		return ctx
	}
	logger := log.New(w, "verbose: ", 0)
	return context.WithValue(ctx, contextKey{}, logger)
}

// Debugf writes a formatted diagnostic message when a verbose logger exists in the context.
func Debugf(ctx context.Context, format string, args ...any) {
	if ctx == nil {
		return
	}
	logger, ok := ctx.Value(contextKey{}).(*log.Logger)
	if !ok || logger == nil {
		return
	}
	logger.Printf(format, args...)
}
