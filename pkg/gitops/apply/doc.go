// Package apply executes the supported, non-destructive portion of a GitOps
// reconciliation plan against live GitHub state.
//
// The package consumes desired config, collected actual state, and a planner
// report, then performs executable create and update actions in a controlled
// order. Unsupported delete and remove drift is reported back to callers as
// skipped state rather than being executed.
package apply
