package output

import "github.com/spf13/cobra"

// OperationResultStatus is the status value written for command operation output.
type OperationResultStatus string

const (
	// OperationResultStatusSuccess indicates a successful command operation.
	OperationResultStatusSuccess OperationResultStatus = "success"
	// OperationResultStatusDryRun indicates a dry-run command operation preview.
	OperationResultStatusDryRun OperationResultStatus = "dry-run"
)

// OperationResult is a structured payload for mutating command output.
type OperationResult struct {
	Status  OperationResultStatus `json:"status"`
	Message string                `json:"message"`
	Data    any                   `json:"data,omitempty"`
}

// PrintOperationResult writes a structured operation result to command stdout.
func PrintOperationResult(cmd *cobra.Command, result OperationResult) error {
	return PrintJSON(cmd, result)
}

// PrintSuccess writes a structured success result to command stdout.
func PrintSuccess(cmd *cobra.Command, message string, data any) error {
	return PrintOperationResult(cmd, OperationResult{
		Status:  OperationResultStatusSuccess,
		Message: message,
		Data:    data,
	})
}

// PrintDryRun writes a structured dry-run result to command stdout.
func PrintDryRun(cmd *cobra.Command, message string, data any) error {
	return PrintOperationResult(cmd, OperationResult{
		Status:  OperationResultStatusDryRun,
		Message: message,
		Data:    data,
	})
}
