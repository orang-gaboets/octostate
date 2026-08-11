// Package orderedtasks runs bounded concurrent work while preserving
// deterministic error selection by task order.
//
// Tasks execute concurrently under a caller-supplied limit, but results and
// the reported error are resolved by task index rather than completion order.
// That keeps output stable across runs even when scheduling varies, which the
// GitOps engine relies on for reproducible reports.
package orderedtasks
