package main

import (
	"os"

	"github.com/orang-gaboets/repo-builder/cmd/repo-builder/internal/exitcode"
	"github.com/spf13/cobra"
)

var (
	executeFn  = Execute
	checkErrFn = cobra.CheckErr
	exitFn     = os.Exit
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

	checkErrFn(err)
	return 1
}
