// Package exitcode provides typed non-zero exit errors for CLI command flows.
//
// Commands that already emit structured output to stdout (for example JSON
// validation reports) can return an exitcode error to stop execution with a
// specific status code while avoiding duplicate human-readable error text on
// stderr.
//
// The main entrypoint can detect these typed errors and call os.Exit directly.
// Other errors continue to use the default Cobra error handling behavior.
package exitcode
