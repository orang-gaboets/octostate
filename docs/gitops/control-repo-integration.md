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

## Recommended PR Workflow

A typical control-repo PR workflow is:

1. `octostate config validate`
   - Run on every desired-state change
   - Fully offline
   - Catches schema and semantic issues before any live GitHub calls
2. `octostate config plan`
   - Run on PR creation or update
   - Uses live GitHub state
   - Produces deterministic executable/skipped actions for review
3. `octostate config apply --check`
   - Run after validation and planning, before human approval
   - Uses live GitHub state and read-only probes
   - Validates the supported apply path without mutating GitHub
4. Human review
   - Review the config diff, plan output, and check output
   - Approve or reject the PR using the control repo's normal policy
5. Merge to `main`
   - The merged config becomes desired state
6. `octostate config apply`
   - Run after the PR is merged
   - Executes only supported create/update actions against live GitHub
7. `octostate audit pull`
   - Run when the control repo wants to refresh the stored actual-state snapshot
8. `octostate audit diff`
   - Run when the control repo wants offline drift detection against the stored snapshot

## Example CI Sequence

For pull requests that change `config/organization.yaml`, a control repo can run:

```bash
octostate config validate --config-dir ./config
octostate config plan --config-dir ./config --token "$GITHUB_TOKEN"
octostate config apply --config-dir ./config --token "$GITHUB_TOKEN" --check
```

After the PR is approved and merged, a post-merge workflow can run:

```bash
octostate config apply --config-dir ./config --token "$GITHUB_TOKEN"
```

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
- `config apply --check` is a preflight, not a transactional GitHub dry-run; a later apply can still fail because of permission changes, organization policy, rate limits, live state changes after preflight, unsupported validation gaps, or GitHub server-side write validation

## Related Docs

- [GitOps Overview](./overview.md)
- [GitOps Architecture](./architecture.md)
- [Config Commands](../cli/config.md)
- [Audit Commands](../cli/audit.md)
