// Package sizeguard provides bounded handling for structured file inputs and
// their encoded outputs.
package sizeguard

import (
	"errors"
	"fmt"
	"io"
)

// ErrLimitExceeded identifies input or output that exceeds its configured size
// limit.
var ErrLimitExceeded = errors.New("size limit exceeded")

// LimitExceededError reports the configured maximum size for an input or
// encoded output.
type LimitExceededError struct {
	Limit int64
}

func (e *LimitExceededError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("size exceeds the supported limit of %d bytes", e.Limit)
}

// Unwrap lets callers classify size-limit errors without parsing messages.
func (e *LimitExceededError) Unwrap() error {
	if e == nil {
		return nil
	}
	return ErrLimitExceeded
}

// ReadError identifies an input read failure. Process wraps non-EOF errors
// encountered while draining or probing after a successful parse so callers
// can distinguish them from parser errors.
type ReadError struct {
	Err error
}

func (e *ReadError) Error() string {
	if e == nil || e.Err == nil {
		return "read bounded input"
	}
	return fmt.Sprintf("read bounded input: %v", e.Err)
}

// Unwrap returns the underlying read error.
func (e *ReadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Process gives parse at most limit bytes from input. If parse succeeds, it
// drains the remaining bounded input to detect trailing bytes beyond the
// parser's read-ahead, then probes for one byte past the limit. It does not
// drain input after a parse error.
func Process(input io.Reader, limit int64, parse func(io.Reader) error) error {
	if limit < 0 {
		return fmt.Errorf("size limit must not be negative")
	}
	if parse == nil {
		return fmt.Errorf("parser is required")
	}

	limited := &io.LimitedReader{R: input, N: limit}
	if err := parse(limited); err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, limited); err != nil {
		return &ReadError{Err: err}
	}
	if limited.N > 0 {
		return nil
	}

	var probe [1]byte
	n, err := io.ReadFull(input, probe[:])
	if n > 0 {
		return &LimitExceededError{Limit: limit}
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return &ReadError{Err: err}
	}
	return nil
}

// CheckSize rejects encoded output whose byte length exceeds limit.
func CheckSize(encoded []byte, limit int64) error {
	if limit < 0 {
		return fmt.Errorf("size limit must not be negative")
	}
	if int64(len(encoded)) > limit {
		return &LimitExceededError{Limit: limit}
	}
	return nil
}
