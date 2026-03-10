// Package audit provides read-only GitOps audit commands for repo-builder.
//
// The package contains commands that snapshot or compare actual GitHub state
// for use in control-repository workflows. These commands build on the GitOps
// collector layer and emit structured machine-readable output for CI and PR
// automation.
package audit
