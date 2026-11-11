package sizeguard

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProcessAcceptsInputAtOrBelowLimit(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"abc", "abcd"} {
		t.Run(input, func(t *testing.T) {
			t.Parallel()

			var parsed strings.Builder
			err := Process(strings.NewReader(input), 4, func(r io.Reader) error {
				_, err := io.Copy(&parsed, r)
				return err
			})
			if err != nil {
				t.Fatalf("Process returned error: %v", err)
			}
			if got := parsed.String(); got != input {
				t.Fatalf("parser received %q, want %q", got, input)
			}
		})
	}
}

func TestProcessRejectsInputOverLimitWithoutPassingExtraBytesToParser(t *testing.T) {
	t.Parallel()

	var parsed strings.Builder
	err := Process(strings.NewReader("abcde"), 4, func(r io.Reader) error {
		_, err := io.Copy(&parsed, r)
		return err
	})
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("Process error = %v, want ErrLimitExceeded", err)
	}
	if got := parsed.String(); got != "abcd" {
		t.Fatalf("parser received %q, want only the first four bytes", got)
	}
}

func TestProcessRejectsFileThatGrowsAfterSizeInspection(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "input.yaml")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create input file: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.WriteString("abc"); err != nil {
		t.Fatalf("write initial file contents: %v", err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatalf("stat input file: %v", err)
	}
	if info.Size() > 3 {
		t.Fatalf("initial file size = %d, want no more than the three-byte limit", info.Size())
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek input file: %v", err)
	}

	err = Process(file, 3, func(input io.Reader) error {
		if _, err := io.Copy(io.Discard, input); err != nil {
			return err
		}
		grown, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		defer func() { _ = grown.Close() }()
		_, err = grown.WriteString("d")
		return err
	})
	if !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("Process error = %v, want ErrLimitExceeded after the file grows", err)
	}
}

func TestProcessDoesNotDrainAfterParserError(t *testing.T) {
	t.Parallel()

	parseErr := errors.New("malformed input")
	r := &countingReader{reader: strings.NewReader("abcdef")}
	err := Process(r, 4, func(input io.Reader) error {
		var one [1]byte
		_, readErr := input.Read(one[:])
		if readErr != nil {
			return readErr
		}
		return parseErr
	})
	if !errors.Is(err, parseErr) {
		t.Fatalf("Process error = %v, want parser error %v", err, parseErr)
	}
	if r.reads != 1 {
		t.Fatalf("underlying reader was read %d times, want one read", r.reads)
	}
}

func TestProcessReportsReadErrorsWhileDraining(t *testing.T) {
	t.Parallel()

	readErr := errors.New("disk read failed")
	r := &failingReader{data: "a", err: readErr}
	err := Process(r, 4, func(io.Reader) error { return nil })
	var got *ReadError
	if !errors.As(err, &got) || !errors.Is(err, readErr) {
		t.Fatalf("Process error = %v, want ReadError wrapping %v", err, readErr)
	}
}

func TestProcessReportsReadErrorsWhileProbingAtLimit(t *testing.T) {
	t.Parallel()

	readErr := errors.New("disk read failed")
	r := &failingReader{data: "abcd", err: readErr}
	err := Process(r, 4, func(input io.Reader) error {
		_, err := io.Copy(io.Discard, input)
		return err
	})
	var got *ReadError
	if !errors.As(err, &got) || !errors.Is(err, readErr) {
		t.Fatalf("Process error = %v, want ReadError wrapping %v", err, readErr)
	}
}

func TestCheckSizeAllowsExactLimitAndRejectsOverflow(t *testing.T) {
	t.Parallel()

	if err := CheckSize([]byte("abcd"), 4); err != nil {
		t.Fatalf("CheckSize rejected exact-limit output: %v", err)
	}
	if err := CheckSize([]byte("abcde"), 4); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("CheckSize error = %v, want ErrLimitExceeded", err)
	}
}

type countingReader struct {
	reader io.Reader
	reads  int
}

func (r *countingReader) Read(p []byte) (int, error) {
	r.reads++
	return r.reader.Read(p)
}

type failingReader struct {
	data string
	err  error
}

func (r *failingReader) Read(p []byte) (int, error) {
	if r.data != "" {
		n := copy(p, r.data)
		r.data = r.data[n:]
		return n, nil
	}
	return 0, r.err
}
