// Package state defines the in-memory actual-state model used by the
// repo-builder GitOps engine.
//
// The package represents the GitHub organization state collected from live
// APIs and normalized for deterministic planning, audit, and snapshot
// workflows. It does not perform GitHub reads itself; collectors and commands
// build on these types to aggregate, sort, and serialize actual state in a
// stable form.
package state
