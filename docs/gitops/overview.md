# GitOps Overview

`octostate` ships a GitOps engine for managing a GitHub organization from a
single desired-state file, typically `config/organization.yaml`.

At a high level, the engine has five responsibilities:

1. Load desired state from `config/organization.yaml`
2. Read actual state from either live GitHub APIs or a stored snapshot
3. Compare desired state to actual state
4. Produce deterministic machine-readable reports
5. Apply the supported subset of changes back to GitHub

This repository owns the engine and CLI. It does **not** own the full
organization-specific approval workflow or automation policy around that engine.

## Core Flow

The main GitOps flow looks like this:

1. Load and normalize desired config from `config/organization.yaml`
2. Validate that config semantically
3. Build actual organization state:
   - from live GitHub for `config plan`, `config apply`, `config apply --check`, and `audit pull`
   - from `state/actual/snapshot.json` for `audit diff`
4. Compare desired and actual state
5. Emit a deterministic report
6. Optionally execute the supported portion of that report

## Main Packages

- `pkg/gitops/config`
  - Defines the desired-state schema loaded from `config/organization.yaml`
  - Performs strict decoding and load-time normalization
  - Separates loading from semantic validation
- `pkg/gitops/state`
  - Defines the normalized in-memory representation of GitHub organization state
  - Normalizes and sorts collections deterministically
- `pkg/gitops/collector`
  - Reads live GitHub organization state into `state.OrganizationState`
  - Uses bounded concurrency for read-only GitHub calls while preserving deterministic assembled output
- `pkg/gitops/plan`
  - Compares desired config with live `OrganizationState`
  - Produces the structured reconciliation report used by `config plan` and `config apply`
- `pkg/gitops/apply`
  - Executes the supported executable portion of the plan against live GitHub
  - Keeps unsupported `delete` / `remove` drift as reported-but-skipped state
- `pkg/gitops/snapshot`
  - Persists normalized actual-state snapshots for offline workflows
- `pkg/gitops/diff`
  - Compares desired config against a stored snapshot for offline drift detection
- `pkg/gitops/syncfromlive`
  - Builds desired-state proposals from live GitHub state for bootstrap, adopt, and materialize flows

## Desired State: `config/organization.yaml`

The desired-state model centers on one canonical file:

```text
config/
  organization.yaml
```

That file can declare:
- top-level organization membership
- pending invitations
- repositories
- teams
- team memberships
- team repository permissions

Validation is deliberately split into two steps:
- load: can the file be decoded and normalized?
- validate: does the desired state make semantic sense?

For the field-by-field schema and validation semantics, see
[Config schema](./config-schema.md).

## Actual State and Determinism

`state.OrganizationState` is the normalized in-memory model of GitHub state.
It includes:
- members
- pending invitations
- repositories
- teams
- team memberships
- team repository permissions

Normalization is a core design rule across the GitOps engine:
- slices are made non-nil where needed
- collections are sorted deterministically
- repository topics and invitation team slugs are sorted consistently

That normalization keeps snapshots, plans, diff reports, and apply output
stable across repeated runs.

## Live Read Path

The live read path uses `pkg/gitops/collector`.

The collector currently reads:
- organization members
- pending invitations
- repositories
- teams
- team members
- team maintainers
- team repository permissions

The collector uses bounded concurrency for read-only GitHub calls:
- top-level collector fan-out: `4`
- organization member role reads: `2`
- invitation team lookups: `8`
- per-team member / maintainer / repo-permission reads: `8`

Important implementation rule:
- concurrent branches write into per-phase buffers
- `state.OrganizationState` is assembled sequentially after concurrent reads finish
- `Normalize()` still runs once at the end so final output stays deterministic

## Live Planning and Apply

`config plan` compares desired state with live state and produces a structured
`Report`.

The planner computes independent action phases concurrently, then appends them
in a fixed order:
1. repositories
2. teams
3. organization members
4. invites
5. team members
6. team repository permissions

`Report.Normalize()` stays sequential because it is the final global
sort/summarize pass that defines the public plan ordering. Managed repositories
in the desired organization form dependency edges when a missing repository is
created from another managed repository in that same organization. The planner
emits a deterministic dependency-safe topological order, using normalized
repository identity to break ties among ready actions, so a required source
create or `is_template: true` enabling update precedes its consumers. External,
cross-organization, live-only, and other non-managed template references are
not managed dependency edges; their availability remains an apply-preflight
concern. Dependency metadata is internal and is not added to the public plan
JSON.

For an existing source, an explicitly managed `is_template` value is the final
state used by dependents; when it is omitted (or null at the planner layer), the
live template state is retained. A newly created source is usable as a template
only when its final desired state explicitly sets `is_template: true`. A false,
omitted, or null new-source state makes dependent creates non-executable.
Explicit `is_template: null` is accepted by CLI validation and is treated as
unmanaged by the planner.

Availability failures propagate transitively to dependent creates and team
repository permissions. Cycles produce stable diagnostics such as `template
dependency cycle: org/a -> org/b -> org/a`. Team permissions reuse the same
repository availability gate, including its diagnostics.

`config apply` then executes only the supported executable `create` and
`update` actions from that plan. Unsupported `delete` and `remove` drift is
reported back as skipped state rather than being executed.

`config apply --check` runs the same live read and plan build, then performs
apply preflight validation without mutating GitHub. Because it consumes the
same dependency-safe plan order as `config apply`, a repository updated to
`is_template: true` earlier in the plan is available to later same-plan
repository creates that use it as a template. Check mode continues through
independent actions and returns best-effort aggregate errors in plan order;
it is not a transaction or a guarantee that a later apply will succeed.

This check mode is best-effort: it validates the supported apply executor inputs
against the collected live state and uses read-only GitHub probes for supported
apply targets, but it is not a guaranteed GitHub transaction dry-run. A later
apply can still fail because of permissions, organization policy, rate limits,
races after collection, live state changes after preflight, or GitHub-side
validation that only occurs during write-time execution.

## Snapshot and Offline Diff

`audit pull` writes a normalized JSON snapshot to:

```text
state/actual/snapshot.json
```

`audit diff` then compares `config/organization.yaml` against that stored snapshot
without calling GitHub APIs.

This offline path is useful for:
- CI drift detection
- review-time reporting
- separating live reads from later diff analysis

## Command Meanings

- `octostate config validate`
  - Validate `config/organization.yaml` offline
- `octostate config sync-from-live --mode bootstrap`
  - Generate a baseline desired config from live GitHub state
- `octostate config sync-from-live --mode adopt`
  - Merge supported live state back into an existing desired config
- `octostate config sync-from-live --mode materialize`
  - Fill unmanaged optional repository fields from live state for already-declared repositories
- `octostate config plan`
  - Preview deterministic reconciliation actions from desired vs live state
- `octostate config apply`
  - Reconcile GitHub to match desired config for supported create/update actions
- `octostate config apply --check`
  - Preflight the supported apply actions without mutating GitHub
- `octostate audit pull`
  - Snapshot live GitHub state into `state/actual/snapshot.json`
- `octostate audit diff`
  - Compare desired config against the stored snapshot offline

## Mental Model

A good working model is:
- `config/organization.yaml` is the desired contract
- live GitHub or `snapshot.json` is the actual state source
- `plan` and `diff` explain the gap
- `apply` executes the supported part of the live gap
- snapshots make offline drift workflows possible


## Related Docs

- [GitOps architecture](./architecture.md)
- [Control-repo integration](./control-repo-integration.md)
