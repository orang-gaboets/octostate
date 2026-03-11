// Package plan defines the structured reconciliation plan model used by the
// repo-builder GitOps engine.
//
// The package owns the machine-readable report, summary, and action types that
// later planning, apply, and audit workflows share. It keeps plan output
// deterministic by normalizing action ordering, field-change ordering, and
// summary counts independently of the command layer.
package plan
