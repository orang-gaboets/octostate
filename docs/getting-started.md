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

The shell snippets below use Bash/POSIX syntax. On Windows, use the equivalent
PowerShell commands for `PATH` and environment variables.

## Install and verify the CLI

For normal CLI use, download the archive for your platform from the
[GitHub Release](https://github.com/orang-gaboets/octostate/releases), verify
the archive against `checksums.txt`, extract it, and put the executable on
`PATH`. Release archives contain only the executable, `LICENSE`, `README.md`,
and `CHANGELOG.md`.

To verify one downloaded archive, replace the placeholder with its filename:

```bash
archive='octostate_<version>_<platform-archive>'
grep -F "  $archive" checksums.txt | sha256sum -c - # Linux
grep -F "  $archive" checksums.txt | shasum -a 256 -c - # macOS
```

If you prefer to build from source, install a pinned release with Go:

```bash
go install github.com/orang-gaboets/octostate/cmd/octostate@v<version>
```

Ensure Go's install directory is on `PATH`: use `$(go env GOBIN)` when it is
non-empty; otherwise use `$(go env GOPATH)/bin`.

```bash
go_bin="$(go env GOBIN)"
[ -n "$go_bin" ] || go_bin="$(go env GOPATH)/bin"
export PATH="$go_bin:$PATH"
```

For automation or a control repository, use an explicitly selected release
archive or `@v<version>` so the workflow is reproducible. Go programs embedding
Octostate should use the module imports; contributors should clone the
repository and use the development workflow.

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
printf 'GitHub token: '
read -r -s OCTOSTATE_GITHUB_TOKEN
printf '\n'
export OCTOSTATE_GITHUB_TOKEN
```

The commands below use the environment-backed token source, so the token is
not placed in the Octostate process arguments. Use a short-lived,
least-privilege credential and avoid running these commands on shared systems.
The supported `--token` flag remains available for compatibility, but its value
may be visible to process inspection or diagnostics.

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
  --config-dir ./config
```

Review the executable actions and skipped drift. A plan is a preview; it does
not change GitHub.

## Run non-mutating apply preflight

Run the supported apply-path checks before considering any write:

```bash
octostate config apply \
  --config-dir ./config \
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
  --config-dir ./config
```

`config apply` executes the supported create/update portion of the plan. It does
not automatically execute unsupported delete/remove drift.

## Optional: pull and diff actual state

Pull a normalized actual-state snapshot:

```bash
octostate audit pull \
  --config-dir ./config \
  --state-dir ./state
```

This reads GitHub without mutating it and writes:

```text
state/actual/snapshot.json
```

Snapshots contain organization-sensitive data such as members, invitations,
teams, repositories, and related metadata. Keep them in a private control
repository and do not publish them unintentionally.

Compare desired state with that snapshot offline:

```bash
octostate audit diff \
  --config-dir ./config \
  --state-dir ./state
```

If a command fails or you stop it with Ctrl-C, remove the token before
continuing. When finished, remove the token from the environment:

```bash
unset OCTOSTATE_GITHUB_TOKEN
```

The audit path can run after an intentional apply or independently to inspect
current drift. `audit diff` requires an existing snapshot from `audit pull`.
If apply was skipped, drift is expected whenever the live organization differs
from the desired state.

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
