# Design Decisions and Rationale

This page explains *why* Octostate is shaped the way it is. It is written for
newcomers who want to judge whether a change fits the existing design.

It complements the other GitOps pages rather than repeating them:

- [GitOps overview](gitops/overview.md) describes what the engine does and how
  the shipped commands behave.
- [GitOps architecture](gitops/architecture.md) describes how the engine is
  structured and how data flows through its packages.
- This page describes a few durable design choices, why they exist, and what
  they cost.

The code, tests, and the reference docs above stay authoritative. If this page
disagrees with them, they win, and the mismatch should be raised as an issue.

## 1. Declarative desired state, not only imperative commands

**Decision.** The GitOps workflow centers on a declared desired state that is
validated, compared with actual state, planned, and then reconciled. Direct
imperative commands still exist as [primitive commands](cli/primitives.md), but
they are not the GitOps path.

**Why.** Organization structure (members, teams, repositories, permissions) is
long-lived intent. Keeping that intent in version control makes it reviewable
and repeatable, and lets the engine report where GitHub has drifted from it.

**Benefits.**
- Organization changes go through normal Git review.
- The same desired state can be reconciled repeatedly with the same result.
- Drift is visible as the difference between declared and actual state.

**Tradeoffs.**
- The engine has to model GitHub resources and maintain careful reconciliation
  semantics for each supported resource.
- Not every GitHub operation fits a declarative model, so some work stays in
  primitive commands or manual administration.
- Users must learn which resources and fields the desired state manages.

**Related.** [GitOps overview](gitops/overview.md),
[config schema](gitops/config-schema.md).

## 2. One canonical `config/organization.yaml`

**Decision.** Supported desired state lives in one file, typically
`config/organization.yaml`. Loading (decode and normalize) is kept separate
from semantic validation.

**Why.** One file gives one obvious source of truth for supported state, a
single place to review changes, and no ambiguity about which of several files
defines a resource.

**Benefits.**
- Simple mental model: the file *is* the desired contract.
- Deterministic loading and validation, and a single identity for a reviewed
  configuration (`desired_config_identity`).
- Conflicting definitions of the same resource across files cannot occur.

**Tradeoffs.**
- The file grows with the organization, and there is no built-in multi-file
  composition.
- Reusing shared structure across organizations needs mechanisms outside the
  file format.
- The design could change in the future; this page only describes the current
  choice.

**Related.** [Config schema](gitops/config-schema.md),
[control-repo integration](gitops/control-repo-integration.md).

## 3. Deterministic machine-readable output is a product requirement

**Decision.** Plans, apply results, snapshots, and diff reports are normalized
and sorted so that the same inputs produce the same output. Command results go
to stdout, with JSON as the default machine-readable format, and diagnostics
and errors go to stderr.

**Why.** Octostate's output is consumed by automation and reviewed in pull
requests. Output that changed order between runs would create noise in reviews
and false drift in comparisons.

**Benefits.**
- Reproducible CI output and stable inputs for automation.
- Meaningful drift comparison and smaller, readable review diffs.
- Callers can parse stdout without filtering out log lines.

**Tradeoffs.**
- Extra normalization and sorting code throughout the engine.
- Concurrency has to be structured so results are assembled in a stable order
  rather than in completion order.
- Some complexity exists only because ordering matters.

**Related.** [Architecture: determinism rules](gitops/architecture.md#determinism-rules).

## 4. Plan and preflight before mutation

**Decision.** The normal workflow is `config validate`, then `config plan`,
then `config apply --check`, then an intentional `config apply`. Apply can also
be bound to the reviewed configuration with `--expected-config-identity` and to
an independently supplied organization with `--expected-org`.

**Why.** Separating observation from execution gives reviewers and automation a
chance to see what will change, and to catch unsupported or invalid actions,
before anything is written to GitHub.

`config apply --check` is a best-effort, non-mutating preflight. It validates
the planned actions against collected state and uses read-only probes, but it
is **not** a transactional simulation of GitHub writes.

**Benefits.**
- Changes are visible and reviewable before mutation.
- Many invalid or unsupported actions are reported before any write.
- Clear boundaries for automation: read-only steps can run with less risk than
  the write step.

**Tradeoffs.**
- More workflow steps and additional live GitHub reads.
- Preflight cannot guarantee that a later write succeeds: permissions,
  organization policy, rate limits, and GitHub-side validation can still fail
  at write time.
- State can change between planning, checking, and applying.

**Related.** [Config commands](cli/config.md),
[GitOps overview: live planning and apply](gitops/overview.md#live-planning-and-apply).

## 5. Report unsupported destructive drift instead of executing it

**Decision.** When actual state has something the desired state does not, the
plan reports it as `delete` or `remove` drift, and `config apply` skips it.
Apply executes only supported `create` and `update` actions.

**Why.** Destructive changes in a GitHub organization can be hard or impossible
to undo. For example, the research in
[#352](https://github.com/orang-gaboets/octostate/issues/352) notes that
removing a member's last path to a private repository can delete their private
forks. Keeping destructive discrepancies visible but non-executable limits the
blast radius of a mistaken or incomplete desired state.

**Benefits.**
- A typo or omission in the desired state does not silently delete resources.
- Destructive discrepancies stay visible in plans and diffs.
- Destructive remediation remains an explicit human decision.

**Tradeoffs.**
- Reconciliation does not fully converge: unsupported destructive drift
  remains until someone handles it.
- Some remediation stays manual, or waits for explicit future support.
- Users need to understand the difference between reported drift and
  executable actions (`--require-executable` helps automation enforce this).

**Related.** [GitOps overview](gitops/overview.md),
[config commands](cli/config.md).

## 6. Separate live collection from offline analysis

**Decision.** Octostate supports both a live comparison (`config plan` reads
GitHub) and an offline one: `audit pull` stores a normalized snapshot, and
`audit diff` compares desired state with that snapshot without calling GitHub.

**Why.** Not every analysis step needs, or should hold, live GitHub
credentials. A captured snapshot lets drift detection and review-time reporting
run against known, reproducible input.

**Benefits.**
- Drift analysis can run offline and repeatedly against the same snapshot.
- Privileged live collection can be separated from later analysis.
- Analysis does not depend on GitHub being reachable at that moment.

**Tradeoffs.**
- Snapshots go stale; offline diff cannot see changes made after collection.
- Live plan and offline diff serve different purposes and do not have
  identical capabilities (for example, only the live path feeds apply and its
  read-only preflight probes).
- Users and automation have to manage the snapshot lifecycle.

**Related.** [Audit commands](cli/audit.md),
[GitOps overview: snapshot and offline diff](gitops/overview.md#snapshot-and-offline-diff).

## 7. Reusable engine here, organization policy in a control repository

**Decision.** This repository owns reusable engine behavior: the CLI, the
desired-state schema, validation, live collection, planning, preflight and
apply, snapshots and diff, and the deterministic output contracts. A separate
control repository owns organization-specific concerns: change intake, branch
policy, approvals, workflow orchestration, notifications, and when engine
commands run.

**Why.** Approval and governance differ between organizations. Keeping them out
of the engine keeps the engine reusable and lets each organization's policy
evolve on its own schedule.

**Benefits.**
- The engine can serve more than one organization without hard-coded
  governance.
- Approval and workflow policy can change without engine releases.
- The engine/control boundary is explicit and easier to reason about.

**Tradeoffs.**
- Complete production use involves more than one repository.
- Newcomers have to understand one more conceptual boundary.
- Integration responsibility (storing review identity, wiring approvals,
  pinning engine versions) is deliberately left to the control repository.

Octostate provides building blocks such as `--expected-org` and
`--expected-config-identity`, but it does not own or enforce any
organization's review policy.

**Related.** [Control-repo integration](gitops/control-repo-integration.md).

## Keeping this page current

- This page describes major current design choices. It is not an architecture
  decision record (ADR) system or an approval gate.
- Pull requests do not need to update it, and implementation should not wait
  on it.
- When a material change alters one of these decisions, update the relevant
  section when practical. Add a new section only for a durable decision that a
  newcomer would benefit from understanding.
- Do not use this page to declare intended behavior. If the code, the reference
  docs, and a rationale here disagree, raise a separate issue instead of
  resolving it here.
