package config

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/orang-gaboets/octostate/cmd/octostate/internal/exitcode"
)

func printInvalidConfigError(cmd *cobra.Command, err error) {
	if code, ok := exitcode.Code(err); ok && code == validateExitCodeInvalidConfig {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
	}
}
