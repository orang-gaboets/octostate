# Getting started with Octostate

This guide takes a first-time user from a small desired-state file through
offline validation, a live reconciliation plan, non-mutating preflight, and
optional apply and audit steps.

The example values are fictional. `config validate` works with them as written,
but live commands must not be run until the target organization and member
values have been replaced with values appropriate for the organization you
intend to inspect.

## What you need

- Go 1.25.0 or newer. The repository’s `go.mod` is the source of truth for the
  language version; the [development guide](maintainers/development.md)
  explains toolchain selection.
- A GitHub organization you are authorized to inspect; you also need
  authorization to modify it before running `config apply`.
- A GitHub token held in an environment variable for the shortest path through
  this guide.

For automation, GitHub App authentication is also supported. The complete
authentication flags are documented in the [config reference](cli/config.md).

## Install and verify the CLI

Install the latest release locally:

```bash
go install github.com/orang-gaboets/octostate/cmd/octostate@latest
```

Ensure Go's install directory (`$(go env GOPATH)/bin`, unless `GOBIN` is set)
is on `PATH`.

For automation or a control repository, replace `@latest` with an explicitly
selected release so the workflow is reproducible.

Verify that the binary is available:

```bash
octostate --help
```

Create a working directory:

```text
.
├── config/
│   └── organization.yaml
└── state/
```

The `state/` directory is used by the optional snapshot workflow.

## Create desired state

Create `config/organization.yaml` with this small fictional example:

```yaml
organization: example-org
members:
  - username: alice
    role: member
teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: alice
        role: maintainer
```

The example is valid against the current schema. `alice` is also declared in
top-level `members` before being assigned to the `platform` team.

## Understand the operation boundaries

| Command | GitHub access | Mutates GitHub | Local effect |
| --- | --- | --- | --- |
| `config validate` | No | No | Reads desired state |
| `config plan` | Yes | No | Prints a live reconciliation plan |
| `config apply --check` | Yes | No | Runs read-only apply preflight |
| `config apply` | Yes | Yes | Executes supported create/update actions |
| `audit pull` | Yes | No | Writes `state/actual/snapshot.json` |
| `audit diff` | No | No | Reads the stored snapshot and prints drift |

The [CLI reference](cli/config.md), [audit reference](cli/audit.md), and
[GitOps overview](gitops/overview.md) remain authoritative for exact behavior
and limitations.

## Validate offline

Validation does not contact GitHub and is safe to run with the fictional sample:

```bash
octostate config validate --config-dir ./config
```

It checks the YAML structure and semantic rules before any live operation.

## Prepare the live example

Stop before running a live command. The plan, apply, and `audit pull` commands
below derive the target organization from `config/organization.yaml`; they do
not accept a separate `--org` flag. The separate `sync-from-live` workflows
use `--org` and are documented in the [config reference](cli/config.md).

Replace:

- `organization: example-org` with the organization you intend to inspect;
- `username: alice` with the intended GitHub username; and
- any other fictional desired-state values with values appropriate to that
  organization.

Set a token without placing it in the configuration file:

```bash
export GITHUB_TOKEN='replace-with-your-token'
```

Run validation again after replacing the fictional values:

```bash
octostate config validate --config-dir ./config
```

The live commands below are read-only against GitHub until the optional
`config apply` step; `audit pull` still writes the local snapshot.

## Inspect the live plan

Build a deterministic reconciliation preview:

```bash
octostate config plan \
  --config-dir ./config \
  --token "$GITHUB_TOKEN"
```

Review the executable actions and skipped drift. A plan is a preview; it does
not change GitHub.

## Run non-mutating apply preflight

Run the supported apply-path checks before considering any write:

```bash
octostate config apply \
  --config-dir ./config \
  --token "$GITHUB_TOKEN" \
  --check
```

`--check` is best-effort, non-mutating preflight. It is not a transactional
GitHub dry run and cannot guarantee that a later apply will succeed because
permissions, organization policy, rate limits, races, and GitHub-side
validation can change afterward.

Review the preflight result before proceeding.

## Optional: intentionally apply changes

The following command mutates GitHub. Run it only after reviewing the plan and
preflight result and confirming that the target organization and desired state
are correct:

```bash
octostate config apply \
  --config-dir ./config \
  --token "$GITHUB_TOKEN"
```

`config apply` executes the supported create/update portion of the plan. It does
not automatically execute unsupported delete/remove drift.

## Optional: pull and diff actual state

Pull a normalized actual-state snapshot:

```bash
octostate audit pull \
  --config-dir ./config \
  --state-dir ./state \
  --token "$GITHUB_TOKEN"
```

This reads GitHub without mutating it and writes:

```text
state/actual/snapshot.json
```

Compare desired state with that snapshot offline:

```bash
octostate audit diff \
  --config-dir ./config \
  --state-dir ./state
```

The audit path can run after an intentional apply or independently to inspect
current drift. If apply was skipped, drift is expected whenever the live
organization differs from the desired state.

## Already have an existing organization?

If the organization already has live state that you want to bring into desired
state, see [`config sync-from-live`](cli/config.md) for bootstrap, adopt, and
materialize workflows. Those workflows are intentionally outside this main
walkthrough.

## Where to go next

- Review the [config schema](gitops/config-schema.md) for every supported field.
- Read the [GitOps overview](gitops/overview.md) for reconciliation and drift
  semantics.
- See the [primitive CLI reference](cli/primitives.md) for direct organization,
  repository, topic, team, and user commands.
- Read [control-repository integration](gitops/control-repo-integration.md) if
  automation, approval, or branch policy belongs in a separate control repo.
- See the [documentation index](README.md) for maintainer and security docs.
