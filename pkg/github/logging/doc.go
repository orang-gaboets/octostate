// Package logging provides context-based diagnostic logging helpers
// for GitHub service operations in the octostate engine.
//
// The helpers in this package allow command-layer code to enable
// verbose logs for a request flow without forcing service packages
// to write logs unconditionally. Diagnostic logs are intended for
// stderr output and are disabled by default.
package logging
