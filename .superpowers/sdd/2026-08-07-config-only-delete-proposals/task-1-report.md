# Task 1 Implementation Report

Status: implemented and verified locally.

Changed files:

- `cmd/octostate/repo/delete.go`
- `cmd/octostate/repo/delete_test.go`
- `.superpowers/sdd/2026-08-07-config-only-delete-proposals/task-1-report.md`

Commit hash:

- `f57262d` (`feat(repo): add config-only repository deletion`)

Tests run:

1. Red phase

   ```text
   GOCACHE=/private/tmp/octostate-gocache go test ./cmd/octostate/repo -run 'TestDeleteRepo'
   ```

   Result:

   ```text
   FAIL
   ```

   Notes:

   - New tests failed before implementation because `repo delete` did not trim live values and did not support `--to-config`.

2. Verification phase after implementation and formatting

   ```text
   GOCACHE=/private/tmp/octostate-gocache go test -count=1 ./cmd/octostate/repo -run 'TestDeleteRepo'
   ```

   Output:

   ```text
   ok  	github.com/orang-gaboets/octostate/cmd/octostate/repo	0.259s
   ```

Concerns:

- Config-only repository deletion follows the Task 1 interface exactly: `deleteRepoToConfig(cmd *cobra.Command, path, org, name string) error`.
- Because the interface identifies the repository by `org` and `name`, this task only covers config-only deletion for repositories resolved within the organization config by that pair; it does not add any broader owner-selection behavior.

## Fix round 1

Reviewer finding addressed:

- Preserve repository-permission blocker order from `cfg.Teams` instead of sorting rendered blocker strings.

Files updated:

- `cmd/octostate/repo/delete.go`
- `cmd/octostate/repo/delete_test.go`

Tests run:

1. Red verification after changing the expectation first

   ```text
   GOCACHE=/private/tmp/octostate-gocache go test -count=1 ./cmd/octostate/repo -run 'TestDeleteRepo'
   ```

   Result:

   ```text
   FAIL
   ```

   Notes:

   - `TestDeleteRepoToConfigRejectsPermissionBlockers/multiple_blockers_follow_declared_config_order` failed because the implementation still returned `alpha(...), zebra(...)` instead of the fixture order `zebra(...), alpha(...)`.

2. Green verification after removing the sort

   ```text
   GOCACHE=/private/tmp/octostate-gocache go test -count=1 ./cmd/octostate/repo -run 'TestDeleteRepo'
   ```

   Output:

   ```text
   ok  	github.com/orang-gaboets/octostate/cmd/octostate/repo	0.488s
   ```

Concerns:

- The task report file remains under a gitignored path and still requires force-add when committed.
