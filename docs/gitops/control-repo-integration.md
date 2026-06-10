# Control-Repo Integration

This repository is the GitOps **engine**. A separate admin or control
repository should own the organization-specific automation, review process,
and policy around that engine.

This page documents the engine-facing integration contract only: what a
control repo is expected to provide, what `octostate` reads and writes, and
when the main GitOps commands are intended to run.

## Boundary

`octostate` owns:
- the CLI
- the desired-state schema
- validation, planning, apply, snapshot, and offline diff behavior
- deterministic output contracts for plans, applies, and snapshots

A control repo should own:
- change intake and issue templates
- PR policy and approvals
- branch protection and merge strategy
- workflow orchestration and notifications
- organization-specific automation around when engine commands run

If detailed approval or branching workflow docs are needed, they should live in
that control repo rather than in this engine repository.

## Expected Inputs

The canonical desired-state input is:

```text
config/
  organization.yaml
```

The control repo should treat `config/organization.yaml` as the source of truth for
supported GitOps state such as:
- members
- invites
- repositories
- teams
- team memberships
- team repository permissions

## Expected Outputs

The engine works with a state directory that can hold actual-state snapshots
and later derived artifacts:

```text
state/
  actual/
    snapshot.json
```

Current shipped behavior:
- `audit pull` writes `state/actual/snapshot.json`
- `audit diff` reads `state/actual/snapshot.json`
- `config plan` and `config apply --dry-run` print deterministic JSON to stdout
- `config apply --check` prints a preflight JSON envelope to stdout
- `config apply` prints an apply result envelope to stdout

A control repo may choose to persist additional plan, diff, or event artifacts,
but that storage policy belongs to the control repo, not this engine.

## Recommended Command Sequence

A typical integration sequence is:

1. `octostate config validate`
   - Run on every desired-state change
   - Catches schema and semantic issues before any live GitHub calls
2. `octostate config plan`
   - Run when you want a live reconciliation preview for review
   - Uses live GitHub state and produces deterministic executable/skipped actions
3. `octostate config apply`
   - Run after the desired-state change is accepted
   - Executes only supported create/update actions against live GitHub
4. `octostate config apply --check`
   - Run when you want to validate the apply path without mutating GitHub
5. `octostate audit pull`
   - Run when you want to refresh the stored actual-state snapshot
6. `octostate audit diff`
   - Run when you want offline drift detection against the stored snapshot

## Example Layout

A control repo could use a layout like this:

```text
config/
  organization.yaml
state/
  actual/
```

That layout is intentionally minimal. The engine does not require a particular
PR structure, notification model, or deployment workflow around those files.

## Engine Assumptions

When integrating this CLI into automation, it helps to assume:
- `config validate` is fully offline
- `config plan`, `config apply`, `config apply --check`, and `audit pull` need GitHub authentication
- `audit diff` is offline once the snapshot exists
- plan, apply, and snapshot outputs are normalized for deterministic ordering
- `invites` are transitional desired state; long-term membership should move to stable membership declarations once invites are accepted

## Related Docs

- [GitOps Overview](./overview.md)
- [GitOps Architecture](./architecture.md)
- [Config Commands](../cli/config.md)
- [Audit Commands](../cli/audit.md)
