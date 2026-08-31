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

The examples below use `$GITHUB_TOKEN` as a placeholder for a token with the
required organization, repository, and team permissions. In some control
repos, that may need to be a GitHub App installation token or PAT stored as a
secret rather than the default Actions `GITHUB_TOKEN`.

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

## Optional Reusable Configuration-Review Workflow

Octostate publishes
`.github/workflows/config-review.yml` as optional convenience infrastructure
for the common pre-merge review sequence. It is a public integration contract,
not a replacement for the control repository's workflow policy.

The called workflow runs these steps in order:

1. `config validate`
2. `config plan`
3. `config apply --check`

It never runs live `config apply`, creates repository objects, or emits
workflow artifacts or custom outputs. The existing Octostate stdout, stderr,
and exit-code behavior remains visible in the individual workflow steps.

### Caller example

The caller chooses when the review runs and pins both the workflow
implementation and the CLI version:

```yaml
name: Configuration review

on:
  pull_request:
    paths:
      - config/**

jobs:
  config-review:
    uses: orang-gaboets/octostate/.github/workflows/config-review.yml@cc31d1e2332e71006f4d1cc7c70fa337ce7b8598
    permissions:
      contents: read
    with:
      config_dir: ./config
      octostate_version: v1.2.0
    secrets:
      octostate_token: ${{ secrets.OCTOSTATE_TOKEN }}
```

The immutable commit reference selects the reusable workflow file. The
`octostate_version` input selects the CLI installed by that workflow; the two
pins are independent and must be kept at compatible, trusted revisions. The
input accepts a release-style tag such as `v1.2.0` or a full 40-character
commit SHA. Branch names and `latest` are rejected. Use an immutable commit
SHA for the strongest reproducibility, and use a release tag only when the
repository's normal release process is the intended compatibility boundary.
The selected CLI revision must build with Go `1.25.13`; the workflow sets
`GOTOOLCHAIN=local`, so an incompatible revision fails during installation
instead of downloading a newer toolchain automatically.

### Contract and safety boundaries

- `config_dir` is optional and defaults to `./config`; it must contain the
  desired-state `organization.yaml` required by Octostate.
- `octostate_token` is required and is passed explicitly as `--token` only to
  the live plan and preflight steps. It may be a PAT or a pre-created GitHub
  App installation token; this workflow does not mint credentials or accept a
  private key.
- The caller-supplied token must have the organization, repository, team, and
  other read/preflight capabilities needed by the declared configuration.
- The caller must grant `contents: read` so the reusable workflow can check out
  the control repository. Reusable workflows cannot elevate a more restrictive
  `GITHUB_TOKEN` permission set supplied by the caller.
- The workflow's `contents: read` permission controls its automatic repository
  token and does not reduce or expand the separate caller-supplied token.
- Use a short-lived, least-privilege token. Because the current CLI accepts
  authentication through `--token`, the token can be visible to process
  inspection on a shared runner even though it is not echoed or persisted by
  this workflow.
- The caller owns triggers, trusted refs, fork-PR handling, approvals, branch
  protection, environments, notifications, and any later live apply. Do not
  check out an untrusted pull-request head while exposing
  `octostate_token` to the job.
- `config apply --check` is best-effort, non-mutating preflight. It is not a
  transactional dry run and does not guarantee that a later `config apply`
  will succeed.

Advanced consumers may continue invoking the CLI directly when they need
custom triggers, authentication handling, output persistence, approval policy,
or command sequences. This workflow is intentionally one common Octostate
automation pattern; it does not establish a reusable wrapper for every CLI
command or flag combination.

This optional consumer workflow is separate from the trusted live-integration
sandbox tracked by [issue #248](https://github.com/orang-gaboets/octostate/issues/248)
and does not depend on that issue's #247 prerequisite. It does not provide
mutation credentials, test the `octostate-test` organization, or replace that
maintainer-operated workflow.

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
