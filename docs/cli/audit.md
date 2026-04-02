# Audit Commands

The `audit` command group works with stored actual-state snapshots.

Use these commands when you want to capture a stable JSON snapshot from live
GitHub and later compare desired state against that snapshot offline.

Authentication rules:
- `repo-builder audit pull` requires GitHub auth.
- `repo-builder audit diff` is fully offline once the snapshot exists.

Examples below use `$GITHUB_TOKEN` where a live read is required, but
`audit pull` also supports GitHub App authentication with `--app-id`,
`--installation-id`, and `--app-key-path`.

## `repo-builder audit pull`

Pull an actual-state snapshot from live GitHub.

```bash
repo-builder audit pull --config-dir ./config --state-dir ./state --token <token>
```

Flags:
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--state-dir` (required): Path to the state directory where the actual-state snapshot will be written
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Loads `<config-dir>/organization.yaml` to determine the target organization
- Collects current GitHub actual state using the bounded-concurrency GitOps collector layer
- Writes a stable JSON snapshot to `<state-dir>/actual/snapshot.json`
- Prints a structured success result to stdout
- Does not mutate GitHub state (read-only)

Snapshot fields:
- `pulled_at`
- `organization`
- `resolved_invite_user_ids_by_username`
- `members`
- `pending_invitations`
- `repositories`
- `teams`
- `team_members`
- `team_repo_permissions`

Example success output:

```json
{
  "status": "success",
  "message": "wrote actual-state snapshot",
  "data": {
    "path": "state/actual/snapshot.json",
    "organization": "orang-gaboets",
    "pulled_at": "2026-03-10T01:30:00Z"
  }
}
```

This snapshot feeds offline GitOps workflows such as `audit diff` and can also
support later reconciliation planning.

If you are upgrading from an older snapshot format that did not record
organization member roles, run `repo-builder audit pull` once before using
`audit diff` so the stored snapshot includes the current `members[].role`
values.

## `repo-builder audit diff`

Diff desired state against the stored snapshot.

```bash
repo-builder audit diff --config-dir ./config --state-dir ./state
repo-builder audit diff --config-dir ./config --state-dir ./state --fail-on-drift
```

Flags:
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--state-dir` (required): Path to the state directory containing `actual/snapshot.json`
- `--fail-on-drift`: Exit with code `2` when any drift is detected

Behavior:
- Loads `<config-dir>/organization.yaml`
- Runs semantic validation before building the offline diff
- Loads the stored snapshot from `<state-dir>/actual/snapshot.json`
- Builds a deterministic offline drift report without calling GitHub APIs
- Prints the JSON drift report to stdout
- Uses the latest `audit pull` snapshot as its source of actual state

Drift report fields:
- `organization`
- `snapshot_pulled_at`
- `summary`
- `actions`

Drift behavior:
- Uses the same resource ordering and action schema as `config plan`
- Reports `create` / `update` drift for desired state that is missing or changed
- Reports `delete` / `remove` drift for unsupported extra snapshot state
- Does not mutate GitHub state

Exit codes:
- `0`: no drift, or drift detected without `--fail-on-drift`
- `2`: drift detected and `--fail-on-drift` is set
- `1`: load/decode/validation/runtime failure

Example offline diff:

```bash
go run ./cmd/repo-builder audit diff --config-dir ./config --state-dir ./state
```

Example CI-style drift gate:

```bash
go run ./cmd/repo-builder audit diff --config-dir ./config --state-dir ./state --fail-on-drift
```
