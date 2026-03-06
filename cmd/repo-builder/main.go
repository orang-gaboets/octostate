package main

import (
	"fmt"
	"io"
	"os"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/exitcode"
)

var (
	executeFn = Execute
	stderrFn  = func() io.Writer { return os.Stderr }
	exitFn    = os.Exit
)

func main() {
	exitFn(run())
}

func run() int {
	err := executeFn()
	if err == nil {
		return 0
	}

	if code, ok := exitcode.Code(err); ok {
		return code
	}

	_, _ = fmt.Fprintf(stderrFn(), "Error: %v\n", err)
	return 1
}
