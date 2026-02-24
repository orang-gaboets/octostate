// Package safety provides shared helpers for command-line safety controls
// used by mutating repo-builder CLI commands.
//
// The helpers in this package centralize common flag registration and
// validation logic for destructive operations, such as confirmation flags
// and dry-run behavior. This keeps safety checks consistent across commands
// and avoids duplicating confirmation error messages.
package safety
