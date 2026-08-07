# Config-Only Destructive Delete Proposals Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add safe `--to-config` proposal modes to repository and team deletion while preserving existing live-delete behavior.

**Architecture:** Keep proposal logic local to the existing delete commands. Reuse `configproposal.ApplyToConfigFile` and its trimmed, case-insensitive lookup helpers; do not introduce a shared delete abstraction.

**Tech Stack:** Go, Cobra, YAML config validation, existing config proposal and atomic replacement helpers.

## Global Constraints

- Proposal mode must branch before live confirmation and authentication.
- `--to-config` and `--dry-run` are mutually exclusive.
- An explicitly supplied empty or whitespace-only `--to-config` must remain proposal mode and fail safely.
- Missing targets and dependency blockers must leave config bytes unchanged.
- Preserve unrelated configuration entries and use the existing atomic validated mutation path.
- Keep live deletion and `--yes` semantics unchanged.
- Return the standard JSON success envelope.
- Do not add template-reference blocking, new validation codes, shared delete abstractions, or #189 end-to-end coverage.
- Deliver #187 and #188 together; leave #189 separate.

### Task 1: Add config-only repository deletion

**Files:** `cmd/octostate/repo/delete.go`, `cmd/octostate/repo/delete_test.go`

**Interface:** `deleteRepoToConfig(cmd *cobra.Command, path, org, name string) error`

- [ ] Add table-driven tests for successful deletion, trimmed/case-insensitive matching, missing target with unchanged bytes, one and multiple permission blockers, omitted permission owner, empty/whitespace paths, flag conflicts, proposal without credentials or `--yes`, and unchanged live behavior.
- [ ] Run the focused delete tests and confirm the new tests fail before implementation.
- [ ] Add `--to-config` and separate Cobra help examples for proposal, live, and dry-run modes.
- [ ] Branch in this order: conflict check, dry-run, `Changed("to-config")` proposal path, then live confirmation/authentication and the existing GitHub deletion.
- [ ] Implement the local helper with `configproposal.ApplyToConfigFile`, `FindRepositoryIndex`, and a scan of every team using `FindTeamRepositoryIndex`. Aggregate blockers in deterministic team order; config validation prevents duplicate repository permissions within a team, so one match per team finds all valid blockers. Remove only the top-level repository and print the standard success envelope with owner, name, config path, and `changed: true`.
- [ ] Run focused tests, format changed Go files, and commit `feat(repo): add config-only repository deletion`.

### Task 2: Add config-only team deletion

**Files:** `cmd/octostate/team/delete.go`, `cmd/octostate/team/delete_test.go`

**Interface:** `deleteTeamToConfig(cmd *cobra.Command, path, org, slug string) error`

- [ ] Add table-driven tests for successful nested deletion, trimmed/case-insensitive matching, missing target with unchanged bytes, child-team blockers, invite blockers, combined deterministic blockers, empty/whitespace paths, flag conflicts, proposal without credentials or `--yes`, and unchanged live behavior.
- [ ] Run the focused delete tests and confirm the new tests fail before implementation.
- [ ] Add `--to-config` and separate Cobra help examples for proposal, live, and dry-run modes.
- [ ] Use the same conflict → dry-run → proposal → live-confirmation/authentication ordering as repository deletion.
- [ ] Implement the local helper with `configproposal.ApplyToConfigFile` and `FindTeamIndex`. Scan all teams for `parent_slug` references and all invites for `team_slugs` references, aggregate blockers before mutation, remove only the target team, and print the standard success envelope. Explain safety as preserving the config validator’s child-team invariant, not as a live-delete preflight.
- [ ] Run focused tests, format changed Go files, and commit `feat(team): add config-only team deletion`.

### Task 3: Update CLI documentation

**Files:** `docs/cli/primitives.md`, plus Cobra examples in both delete command files

- [ ] Update the proposal overview and destructive-delete sections.
- [ ] Document separate proposal, live, and dry-run examples for both commands, clearly stating that only live mode requires authentication and `--yes`.
- [ ] Document conflict and dependency-safety behavior, and keep #189 explicitly separate.
- [ ] Verify command names, flags, paths, and links, then commit `docs(cli): document config-only delete proposals`.

### Task 4: Final verification

- [ ] Run `gofmt -w` on changed Go files, `gofmt -l .`, `git diff --check`, the focused config replacement tests, `go test ./...`, `go vet ./...`, and `golangci-lint run`.
- [ ] Review the final diff for unrelated changes, accidental live-path changes, output-contract changes, missing issue requirements, or generated `graphify-out/` files.
