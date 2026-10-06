# Audit Commands

The `audit` command group works with stored actual-state snapshots.

Use these commands when you want to capture a stable JSON snapshot from live
GitHub and later compare desired state against that snapshot offline.

Authentication rules:
- `octostate audit pull` requires GitHub auth.
- `octostate audit diff` is fully offline once the snapshot exists.

For PAT authentication, export `OCTOSTATE_GITHUB_TOKEN` before running a live
read. `audit pull` also supports GitHub App authentication with `--app-id`,
`--installation-id`, and `--app-key-path`.

## State-directory paths

`audit pull` and `audit diff` reject symbolic links and, on Windows, all
reparse points in every component of the cleaned absolute path to the snapshot,
including ancestors of `--state-dir`, `<state-dir>`, `actual`, and an existing
`snapshot.json`. Existing snapshot files must be regular files. Errors identify
the unsafe path component without displaying a link target and advise passing
the resolved physical path.

Pass a physical path whose components contain no symbolic links or reparse
points. For example, on macOS, `/tmp` and `/var` are commonly symlink aliases;
resolve those aliases before passing a path beneath them. The same rule applies
to relative paths: if the current working directory was reached through a
symlink, `./state` can be rejected. Use the physical working directory (for
example, `pwd -P` on Unix) or pass a resolved physical path. Snapshot reads and
writes use rooted filesystem operations and keep directory handles open while
traversing, so replacing a checked path component cannot redirect the file
operation to another directory. Snapshot access fails closed on platforms where
Go cannot provide stable rooted filesystem operations.

## `octostate audit pull`

Pull an actual-state snapshot from live GitHub.

```bash
export OCTOSTATE_GITHUB_TOKEN="<token>"
octostate audit pull --config-dir ./config --state-dir ./state
```

Flags:
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--state-dir` (required): Path to the state directory where the actual-state snapshot will be written
- `--token`: Optional explicit GitHub personal access token; prefer `OCTOSTATE_GITHUB_TOKEN` for PAT authentication
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
organization member roles, run `octostate audit pull` once before using
`audit diff` so the stored snapshot includes the current `members[].role`
values.

Snapshots written before direct and inherited team membership were
distinguished may contain inherited-only parent-team rows. The existing
`team_members` snapshot shape has no provenance field, so those legacy rows
are deterministically treated as direct memberships and can continue to
produce removal drift. Run `octostate audit pull` once to refresh the snapshot;
new snapshots include only direct team memberships. No snapshot format change
is required.

## `octostate audit diff`

Diff desired state against the stored snapshot.

```bash
octostate audit diff --config-dir ./config --state-dir ./state
octostate audit diff --config-dir ./config --state-dir ./state --fail-on-drift
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
octostate audit diff --config-dir ./config --state-dir ./state
```

Example CI-style drift gate:

```bash
octostate audit diff --config-dir ./config --state-dir ./state --fail-on-drift
```
