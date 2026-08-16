# GitOps Architecture

This page is the code-facing companion to the higher-level
[GitOps overview](./overview.md).

Use it when you want to understand how the shipped GitOps packages fit
together, which package owns which responsibility, and how the main CLI
commands move data through the engine.

## System Flow

```mermaid
flowchart LR
    A["config/organization.yaml"] --> B["pkg/gitops/config<br/>load + normalize"]
    B --> C["semantic validation"]

    C --> D["pkg/gitops/collector<br/>live GitHub reads"]
    C --> E["pkg/gitops/snapshot<br/>state/actual/snapshot.json"]

    D --> F["pkg/gitops/state<br/>normalized OrganizationState"]
    E --> F

    F --> G["pkg/gitops/plan<br/>live reconciliation report"]
    F --> H["pkg/gitops/diff<br/>offline drift report"]

    G --> I["config plan JSON"]
    G --> J["pkg/gitops/apply<br/>preflight validation"]
    J --> K["config apply --check JSON"]
    G --> L["pkg/gitops/apply<br/>execute supported create/update actions"]
    L --> M["config apply JSON"]

    D --> N["audit pull<br/>write snapshot.json"]
    H --> O["audit diff JSON"]
```

## Package Ownership

### `pkg/gitops/config`
- Loads `config/organization.yaml`
- Uses strict YAML decoding
- Applies safe load-time normalization
- Leaves semantic validation as a separate step

### `pkg/gitops/state`
- Defines the normalized in-memory model of actual GitHub organization state
- Keeps slices non-nil where needed
- Sorts collections deterministically

### `pkg/gitops/collector`
- Reads live GitHub state into `state.OrganizationState`
- Uses bounded concurrency for read-only calls
- Assembles final state sequentially, then normalizes once

Current collector concurrency limits:
- top-level fan-out: `4`
- organization member role reads: `2`
- invitation team lookups: `8`
- per-team member / maintainer / repo-permission reads: `8`

### `pkg/gitops/plan`
- Compares desired config with live `OrganizationState`
- Produces the structured reconciliation report used by `config plan`
  and `config apply`
- Builds the repository plan first, then computes the five independent
  non-repository action phases concurrently and appends them in fixed order
  before final action normalization
- Builds managed same-organization template dependency edges for missing
  repositories and emits them in deterministic dependency-safe topological
  order, using normalized repository identity to break ready-action ties
- Uses final template state for dependency availability: existing sources use
  an explicitly managed `is_template` value or retain live state when omitted;
  new sources require `is_template: true`
- Propagates unavailable-source diagnostics transitively, reports stable cycle
  paths, and shares repository availability with team repository permissions
- Leaves external, cross-organization, live-only, and otherwise non-managed
  template references to apply
  preflight; dependency edges are internal and are not fields in public plan
  JSON

### `pkg/gitops/apply`
- Executes the supported executable subset of the plan
- Keeps writes ordered and controlled
- Reports unsupported `delete` / `remove` drift as skipped state

### `pkg/gitops/snapshot`
- Owns the JSON snapshot file contract
- Persists normalized actual-state snapshots for offline workflows

### `pkg/gitops/diff`
- Compares desired config with the stored snapshot offline
- Produces drift reports without live GitHub calls
- Reuses the same action vocabulary and deterministic reporting conventions as
  plan, with offline repository ordering appropriate to snapshot diff

### `pkg/gitops/syncfromlive`
- Builds desired-state proposals from live GitHub state
- Powers bootstrap, adopt, and materialize flows

## Command Workflows

### `octostate config plan`
1. Load and normalize `config/organization.yaml`
2. Validate desired state semantically
3. Collect live GitHub state through `collector`
4. Build the reconciliation report through `plan`
5. Normalize the final report and print JSON

### `octostate config apply`
1. Load and validate desired state
2. Collect live GitHub state
3. Build the reconciliation report
4. Execute supported `create` / `update` actions through `apply`
5. Print executed and skipped actions as JSON

### `octostate config apply --check`
1. Load and validate desired state
2. Collect live GitHub state
3. Build the reconciliation report
4. Run apply preflight validation without mutating GitHub
5. Print the preflight result as JSON

Check mode is best-effort. It consumes the same dependency-safe order as
`config apply`, validates supported executor inputs, and adds read-only probes
for supported apply targets. It continues through remaining executable actions
and aggregates preflight failures deterministically. Repository-action failures
are processed in plan order; resource-specific dependency handling may defer
some checks. It is not a guaranteed GitHub transaction dry-run and can miss
GitHub-side failures such as permission changes,
organization policy, rate limits, races after collection, live state changes
after preflight, or validation that only occurs during write-time execution.
Same-plan template updates and repository creations are visible to later
preflight checks; team repository permissions use the same repository
availability gate.

### `octostate audit pull`
1. Load desired state to determine the target organization
2. Collect live GitHub state
3. Normalize and persist `state/actual/snapshot.json`
4. Print the snapshot write result

### `octostate audit diff`
1. Load desired state
2. Load the stored snapshot
3. Compare desired vs stored actual state through `diff`
4. Print a deterministic offline drift report

## Determinism Rules

The GitOps engine relies on a few consistent rules to keep CI output and review
artifacts stable:
- normalize desired config after loading
- normalize actual state after collection
- normalize snapshots before writing
- normalize plan and diff outputs before printing
- append concurrent phase results back in a fixed order

Those rules are what make repeated runs produce stable JSON, even when live
reads or pure build phases use bounded concurrency internally.

## Control-Repo Boundary

This page is intentionally about the engine internals only. The control repo
should own:
- approval workflow
- issue intake
- branch policy
- notifications
- when these commands are run in CI or automation

For that engine-facing boundary, see
[Control-repo integration](./control-repo-integration.md).
