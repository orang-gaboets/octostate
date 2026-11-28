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

## 1. Declarative desired state, with direct administration alongside

**Decision.** The GitOps workflow centers on a declared desired state that is
validated, compared with actual state, planned, and then reconciled. Direct
administration also exists as [primitive commands](cli/primitives.md). Supported
primitives can instead record a local configuration proposal with
`--to-config`, which updates `organization.yaml` offline rather than mutating
GitHub.

**Why.** The declarative model is inspired by Terraform and Infrastructure as
Code: organization structure (members, teams, repositories, permissions) is
long-lived intent, so it is represented as version-controlled configuration and
supported changes are reconciled, rather than requiring operators to issue
every API operation by hand. That enables review, drift detection, and
automation. Direct administration currently exists alongside it because it
gives operational flexibility and a simpler entry point for discrete tasks.
Proposal mode connects the two: a primitive can produce a configuration change
for the caller to put through its own branch, pull request, and approval
process. Declarative intent remains the center of the GitOps workflow.

**Benefits.**
- Version-controlled desired state enables organization changes to be reviewed
  through normal Git workflows when the caller chooses that process.
- Reconciliation is repeatable under stable inputs and conditions; after
  successful changes, subsequent runs may correctly become no-ops, and
  unsupported drift can remain.
- Drift is visible as the difference between declared and actual state.
- Discrete tasks can still be done directly, or recorded as a proposal.

**Tradeoffs.**
- The engine has to model GitHub resources and maintain resource-specific
  reconciliation rules.
- Direct writes bypass the declared state and can introduce drift.
- Supporting both paths adds validation and maintenance obligations.
- Users must learn which resources and fields the desired state manages.

**Related.** [GitOps overview](gitops/overview.md),
[config schema](gitops/config-schema.md),
[primitive commands and proposal mode](cli/primitives.md).

## 2. One canonical `config/organization.yaml`

**Decision.** Supported desired state lives in one file, typically
`config/organization.yaml`. Loading (decode and normalize) is kept separate
from semantic validation.

**Why.** The single file was an early simplicity choice: prioritize the core
desired-state and reconciliation engine before introducing configuration
composition, merge rules, or cross-file conflict resolution. One obvious entry
point is also an intentional usability benefit.

**Benefits.**
- One obvious place to read and review supported desired state.
- No merge rules or cross-file conflicts to define or debug.

Today's deterministic loading and validation, and the effective configuration
identity used to bind apply to a reviewed configuration
(`desired_config_identity`), are separate properties. Neither inherently
requires one physical file.

**Tradeoffs.**
- The file grows with the organization.
- There is no built-in multi-file composition, which limits modular reuse.
- The design could change in the future; this page only describes the current
  choice.

**Related.** [Config schema](gitops/config-schema.md),
[Go API: bind apply to a reviewed configuration](go-api.md#bind-apply-to-a-reviewed-desired-configuration).

## 3. One organization per configuration scope

**Decision.** Each desired-state scope manages one GitHub organization.
Managed repository owners default to, and must match, the top-level
`organization`.

**Why.** A clear organization-local ownership boundary keeps configurations and
administrative operations easier to reason about, maintain, and authorize, and
limits accidental cross-organization targeting. This is a separate choice from
keeping configuration in one file: the ownership boundary would still be
meaningful if configuration composition changed.

**Benefits.**
- Clear ownership and a well-defined target for validation and reconciliation.
- Simpler operational coordination within one scope.
- Explicit cross-organization managed owners are rejected
  (`repository_owner_scope`) instead of silently targeting another
  organization.

**Tradeoffs.**
- Consumers managing several organizations coordinate separate scopes.
- Some references are deliberately not managed ownership: an external template
  source is only a reference, and an in-organization team repository grant does
  not by itself mean the repository's settings are managed.

**Related.** [Config schema: ownership boundary](gitops/config-schema.md#ownership-boundary-and-migration),
[#208](https://github.com/orang-gaboets/octostate/issues/208), [#213](https://github.com/orang-gaboets/octostate/issues/213), [#313](https://github.com/orang-gaboets/octostate/issues/313).

## 4. Incremental GitOps adoption

**Decision.** Users can adopt GitOps gradually instead of declaring every
supported aspect of an existing organization at once. They can start with
selected resources and fields and deliberately expand management over time.

**Why.** This keeps initial adoption simple and flexible and reduces the
onboarding burden for organizations that already exist and are already being
administered.

**Benefits.**
- Lower migration friction.
- Coexistence with existing administration.
- Clearer ownership of the fields that are declared.
- `config sync-from-live` supports this with `bootstrap`, `adopt`, and
  `materialize` modes for producing or extending desired state from live
  GitHub.

**Tradeoffs.**
- Reconciliation semantics are more nuanced. Omitted presence-aware optional
  fields stay unmanaged, exact-reconcile fields such as repository `topics`
  follow different rules, and omitted entities may still appear as report-only
  drift.
- A partial desired configuration is not a complete inventory of the
  organization.
- Some metadata is resource-specific. For example, pending invitation creation
  metadata is not necessarily reconciled continuously; see
  [invitation reconciliation semantics](gitops/config-schema.md#invitation-reconciliation-semantics).

**Related.** [Config schema](gitops/config-schema.md),
[config commands](cli/config.md), [#61](https://github.com/orang-gaboets/octostate/issues/61), [#69](https://github.com/orang-gaboets/octostate/issues/69), [#73](https://github.com/orang-gaboets/octostate/issues/73).

## 5. Deterministic machine-readable output

**Decision.** Structured results use stable normalization and ordering to
support predictable automation and regression testing. Command results go to
stdout, with JSON as the default machine-readable format except where a
command documents another stdout format (such as generated YAML), and
diagnostics and errors go to stderr.

**Why.** The primary motivations are predictable machine consumption and
automated regression testing. Readable pull request reviews and audit
comparisons are additional benefits.

Capture-time metadata can differ across fresh snapshots even when resource
state is unchanged: a fresh `audit pull` records `pulled_at`, and a diff
reports the snapshot's `snapshot_pulled_at`. Repeated analysis against a fixed
snapshot and a stable configuration can be reproducible. Stable ordering
removes noise from raw artifact and text comparisons; drift itself comes from
the engine's normalized, resource-specific comparison, not from ordering.

**Benefits.**
- Stable inputs for automation and regression tests.
- Smaller, readable review diffs of plans and reports.
- Callers can parse stdout without filtering out log lines.

**Tradeoffs.**
- Normalization and sorting code throughout the engine.
- Results collected concurrently must be assembled in a stable order rather
  than completion order, which is an implementation and testing cost.

**Related.** [Architecture: determinism rules](gitops/architecture.md#determinism-rules),
[audit commands](cli/audit.md).

## 6. Validation, planning, preflight, and fresh apply

**Decision.** The recommended sequence is `config validate`, `config plan`,
`config apply --check`, then `config apply`. It is recommended, not mandatory:
apply can be invoked directly.

- `config validate` checks desired configuration offline.
- `config plan` compares desired state with live state and classifies
  executable and skipped work.
- `config apply --check` is best-effort, read-only preflight of the executable
  subset; it can succeed while reporting skips.
- `config apply` reloads configuration, recollects live state, and builds a
  fresh plan instead of replaying a saved one.

Automation can optionally assert the trusted reviewed effective configuration
identity with `--expected-config-identity` and an independently authorized
organization with `--expected-org`.

**Why.** The primary motivation is reliable staged automation, particularly in
GitHub Actions, with visibility and safety as benefits. Apply builds a fresh
plan so it acts on current observations rather than on a plan that may have
gone stale. The two expected-value flags are separate safeguards: one for
*what* was reviewed, one for *where* execution is intended. The caller must
carry trusted values forward between steps. Together they support reliable
review-to-apply workflows and address the concerns raised in
[#313](https://github.com/orang-gaboets/octostate/issues/313) and [#314](https://github.com/orang-gaboets/octostate/issues/314).

**Benefits.**
- Changes are visible before mutation, and many invalid or unsupported actions
  are reported before any write.
- Read-only steps can run with less privilege than the write step.
- Apply cannot silently act on a different configuration or organization when
  the caller supplies the expected values.

**Tradeoffs.**
- Fresh actions at apply time can differ from the earlier reviewed plan.
- Neither identity verification nor preflight guarantees the same actions,
  successful writes, or atomicity. Apply is not transactional: earlier writes
  can succeed before a later failure.
- More steps and additional live GitHub reads. Approval policy stays with the
  caller.

**Related.** [Config commands](cli/config.md),
[Go API: bind apply to a reviewed configuration](go-api.md#bind-apply-to-a-reviewed-desired-configuration),
[GitOps overview: live planning and apply](gitops/overview.md#live-planning-and-apply).

## 7. Conservative reconciliation

**Decision.** Unsupported entity and relationship `delete` and `remove`
actions are reported as drift and skipped by `config apply`. Where the
available observations are not enough to reconcile safely, the engine prefers
explicit uncertainty over unsafe assumptions.

**Why.** The original limited apply scope intentionally reduced initial
development complexity and avoided destructive automatic operations. Later
research ([#352](https://github.com/orang-gaboets/octostate/issues/352)) reinforced the safety concern: for example, removing
a member's last path to a private repository can delete their private forks.

Not every omission produces removal drift. Omitted presence-aware optional
fields can be unmanaged, inherited-only team membership does not generate
removal, and unknown-origin membership is reported as unresolved. Conversely,
supported updates can still reduce or clear state: they can clear a
description, replace topics (omitted topics on a declared repository mean an
empty set), downgrade a direct team role, or reduce repository permissions.

`--require-executable` makes `config apply --check` or live apply fail when
planning identifies a desired create or update that cannot execute, or a
desired team membership with unknown observed origin. Unsupported delete or
remove drift can still be reported without failing this requirement. The flag
is incompatible with `--dry-run` and does not guarantee successful writes. A
separate guard rejects caller-supplied plan actions that target unknown-origin
memberships regardless of the flag.

The shipped example of explicit uncertainty is team membership origin (direct,
inherited, or unknown; [#371](https://github.com/orang-gaboets/octostate/issues/371), [#372](https://github.com/orang-gaboets/octostate/issues/372), and PR #373). Unknown
origin is reported separately as unresolved, generates no membership actions,
and is not materialized as desired membership by `bootstrap` or `adopt`. This
avoids treating absence of evidence as confirmed direct membership. Not every
resource implements this policy yet.

**Benefits.**
- An omission in the desired state does not trigger unsupported destructive
  deletes or removals.
- Destructive discrepancies and uncertain observations stay visible.
- Destructive remediation remains an explicit human decision.

**Tradeoffs.**
- Reconciliation does not fully converge: unsupported destructive drift
  remains until someone handles it.
- Unresolved state needs investigation, and state and snapshot handling is more
  nuanced.
- Users need to understand the difference between reported drift, unresolved
  observations, and executable actions.

**Related.** [Config commands](cli/config.md),
[config schema](gitops/config-schema.md), [#45](https://github.com/orang-gaboets/octostate/issues/45).

## 8. Separate live collection from offline analysis

**Decision.** Octostate supports both a live comparison (`config plan` reads
GitHub) and an offline one: `audit pull` stores a normalized snapshot, and
`audit diff` compares desired state with that snapshot without calling GitHub.

**Why.** The original motivation was to simplify analysis and avoid querying
GitHub again for every comparison. A normalized capture can be reused by
offline CI, review, and audit workflows. Separating collection also reduces
repeated API calls and rate-limit pressure, allows analysis without live
credentials (given access to the snapshot), and allows repeated comparison
against known input even when GitHub is unavailable.

**Benefits.**
- Drift analysis can run offline and repeatedly against the same snapshot.
- Privileged live collection can be separated from later analysis.
- Fewer live API calls for repeated comparisons.

**Tradeoffs.**
- Snapshots go stale; offline diff cannot see changes made after collection.
- Snapshots carry storage, refresh, and reader-compatibility obligations, and
  must preserve relevant uncertainty such as unknown membership origin.
- Fresh captures are not byte-identical, because capture timestamps differ.
- The CLI live apply path needs fresh observations. Offline diff is not a
  substitute for live planning or preflight.

**Related.** [Audit commands](cli/audit.md),
[GitOps overview: snapshot and offline diff](gitops/overview.md#snapshot-and-offline-diff),
[#44](https://github.com/orang-gaboets/octostate/issues/44), [#46](https://github.com/orang-gaboets/octostate/issues/46).

## 9. Reusable engine, caller-owned governance

**Decision.** Octostate is a reusable GitHub Organization as Code engine, not
an organization-specific governance platform. This repository owns the CLI,
the desired-state schema, validation, live collection, planning, preflight and
apply, snapshots and diff, and the output contracts. The caller owns
organization-specific change intake, branch policy, approvals, orchestration,
notifications, and when the engine runs. A dedicated control repository is one
way to implement that policy.

**Why.** As with the Terraform CLI, as distinct from hosted policy products,
the engine exposes capabilities while callers decide their approvals,
credentials, orchestration, and execution policy. Approval and governance
differ between organizations, so keeping them out of the engine keeps it
reusable.

The engine is CLI-first, not CLI-only. Shell and CI integration, with GitHub
Actions as a primary intended integration, is the main interface. Supported Go
APIs let developers reuse configuration, state, plan, diff, and snapshot
building blocks in custom integrations.

**Benefits.**
- The engine can serve different organizations without hard-coded governance.
- Approval and workflow policy can change without engine releases.
- Custom integrations can reuse supported Go building blocks.

**Tradeoffs.**
- Production integrations may involve another repository or layer, while
  direct CLI use remains supported.
- Integration responsibility (storing review identity, wiring approvals,
  pinning engine versions) is deliberately left to the caller. The optional
  reusable configuration-review workflow does not perform live apply or own
  later approval policy.
- Supported Go APIs carry compatibility and maintenance obligations. Not every
  exported package is a supported stable API; see the
  [Go API guide](go-api.md#current-package-surface).

Octostate provides building blocks such as `--expected-org` and
`--expected-config-identity`, but it does not own or enforce any
organization's review policy.

**Related.** [Control-repo integration](gitops/control-repo-integration.md),
[optional reusable workflow](gitops/control-repo-integration.md#optional-reusable-configuration-review-workflow),
[Go API guide](go-api.md).

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
