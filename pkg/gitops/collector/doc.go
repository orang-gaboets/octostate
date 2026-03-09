// Package collector loads actual GitHub organization state into the GitOps
// actual-state model.
//
// The package coordinates read-only organization, repository, team, and
// invitation queries using the existing GitHub service layer. Callers receive a
// normalized state.OrganizationState value that is ready for snapshotting,
// planning, and drift detection.
package collector
