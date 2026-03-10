// Package snapshot persists normalized GitOps actual-state snapshots.
//
// The package owns the JSON file contract used by read-side audit commands so
// later audit and diff workflows can share the same snapshot shape and file
// layout. It keeps serialization deterministic by reusing the normalized
// OrganizationState model.
package snapshot
