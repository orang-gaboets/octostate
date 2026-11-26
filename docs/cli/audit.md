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

Snapshot JSON read by `audit diff` is limited to 128 MiB (128 × 1024 × 1024
bytes), 12,000,000 JSON decoder tokens, 1,000,000 total array elements, and
250,000 members per object, inclusive. Token count includes object keys,
scalar values, and delimiters; snapshots above any limit are rejected before
decoding. `audit pull` does not persist a generated snapshot above any of these
limits. The desired-state YAML loaded by `audit pull` is subject to the size,
source-complexity, and node limits in the
[configuration schema](../gitops/config-schema.md).

These are aggregate structural limits, not per-resource allowances. The
1,000,000 snapshot array elements are counted across all arrays. A snapshot
with 1,000 teams each having permissions for 1,000 repositories would exceed
that budget by itself once the team rows are included, before members,
invitations, or other arrays are counted. YAML capacity also depends on the
shape and mix of configuration nodes.

## `octostate audit pull`

Pull an actual-state snapshot from live GitHub.

```bash
export OCTOSTATE_GITHUB_TOKEN="<token>"
octostate audit pull --config-dir ./config --state-dir ./state --expected-org orang-gaboets
```

Flags:
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--state-dir` (required): Path to the state directory where the actual-state snapshot will be written
- `--expected-org`: Optional caller-authorized organization, supplied independently of `organization.yaml`
- `--token`: Optional explicit GitHub personal access token; prefer `OCTOSTATE_GITHUB_TOKEN` for PAT authentication
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Loads `<config-dir>/organization.yaml` to determine the target organization
- When `--expected-org` is supplied, requires a valid GitHub organization login and compares trimmed values without regard to case; an empty, invalid, or mismatched value is rejected before authentication, live reads, or snapshot writing. Omission retains legacy behavior
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
distinguished have no `origin` field. Such legacy rows load as `unknown`, are
reported in `unresolved_team_memberships`, and do not generate membership
actions. New snapshots include an `origin` field on every `team_members` row:
`direct`, `inherited`, or `unknown`. GitHub's explicit direct/inherited labels
are used when present; a missing or null label and role-only fallback reads do
not prove direct membership, so they are recorded as `unknown`. Run
`octostate audit pull` to refresh the snapshot from current API evidence.
Snapshot readers that validate a fixed schema must accept the new field. The
v1.2.0 CLI strictly decodes the previous snapshot schema and rejects this new
`team_members[].origin` field. The control repository currently pins that
version, so upgrade its CLI before a newer `audit pull` writes this snapshot
schema; keep `audit pull` and `audit diff` on compatible CLI versions.

The [#371 research report](https://github.com/orang-gaboets/octostate/issues/371)
distinguishes documented semantics from live observations. GitHub documents
`inherited: false` as direct and `inherited: true` as inherited, but does not
specify how omitted or null values should be interpreted. The authorized
sandbox exposed no teams, so no live team-member response shapes or overlapping
direct/inherited memberships were observed. Because those response shapes are
undocumented and unobserved, this implementation conservatively treats
missing/null values and role-only fallback rows as unknown.

## `octostate audit diff`

Diff desired state against the stored snapshot.

```bash
octostate audit diff --config-dir ./config --state-dir ./state
octostate audit diff --config-dir ./config --state-dir ./state --fail-on-drift
```

Flags:
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--state-dir` (required): Path to the state directory containing `actual/snapshot.json`
- `--fail-on-drift`: Exit with code `2` when actions or unresolved memberships
  are reported

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
- `unresolved_team_memberships`

Drift behavior:
- Uses the same resource ordering and action schema as `config plan`
- Reports `create` / `update` drift for desired state that is missing or changed
- Reports `delete` / `remove` drift for unsupported extra snapshot state
- Reports unknown-origin membership rows as unresolved and non-actionable
- Keeps `summary.has_changes` action-only; `summary.has_unresolved` and
  `summary.unresolved_memberships` separately describe unresolved evidence
- Does not mutate GitHub state

Exit codes:
- `0`: no actions or unresolved memberships, or either is reported without
  `--fail-on-drift`
- `2`: actions or unresolved memberships are reported and `--fail-on-drift` is
  set
- `1`: load/decode/validation/runtime failure

Example offline diff:

```bash
octostate audit diff --config-dir ./config --state-dir ./state
```

Example CI-style drift gate:

```bash
octostate audit diff --config-dir ./config --state-dir ./state --fail-on-drift
```
