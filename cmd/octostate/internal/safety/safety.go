package safety

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// ErrConfirmationRequired indicates a destructive command was invoked
	// without the explicit confirmation flag.
	ErrConfirmationRequired = errors.New("confirmation required")
)

// AddDryRunFlag registers a --dry-run flag on the provided command.
func AddDryRunFlag(cmd *cobra.Command, dryRun *bool) {
	cmd.Flags().BoolVar(dryRun, "dry-run", false, "Preview the operation without making changes")
}

// AddYesFlag registers a --yes flag on the provided command.
func AddYesFlag(cmd *cobra.Command, yes *bool) {
	cmd.Flags().BoolVar(yes, "yes", false, "Confirm the destructive operation")
}

// RequireYesOrDryRun validates that a destructive operation is either explicitly
// confirmed with --yes or invoked in --dry-run mode.
func RequireYesOrDryRun(yes, dryRun bool) error {
	if yes || dryRun {
		return nil
	}
	return fmt.Errorf("%w: pass --yes to proceed or use --dry-run to preview", ErrConfirmationRequired)
}
