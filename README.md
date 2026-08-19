# octostate

[![CI](https://github.com/orang-gaboets/octostate/actions/workflows/ci.yml/badge.svg)](https://github.com/orang-gaboets/octostate/actions/workflows/ci.yml)
[![Latest release](https://img.shields.io/github/v/release/orang-gaboets/octostate?sort=semver)](https://github.com/orang-gaboets/octostate/releases)
[![Go version](https://img.shields.io/github/go-mod/go-version/orang-gaboets/octostate)](https://github.com/orang-gaboets/octostate/blob/main/go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Manage a GitHub organization as version-controlled desired state.

`octostate` is the GitHub organization operations CLI and GitOps engine that
validates desired state, previews reconciliation, preflights supported changes,
applies approved updates, and detects drift.

This repository is the engine: a CLI and reusable automation building blocks.
A separate admin or control repository should own organization-specific
approval workflow, branching policy, notifications, and automation around the
engine.

## Why Octostate?

- Version-controlled organization state that can be reviewed like code
- Deterministic reconciliation plans for repeatable automation
- Read-only preflight before supported mutations
- Snapshot-based drift detection and offline diffing
- JSON-first output for CI and control-repository workflows

## GitOps at a glance

```text
desired state
    ↓
validate
    ↓
plan
    ↓
apply --check
    ↓
review / approval
    ↓
apply
    ↓
audit
```

A minimal desired-state file looks like this:

```yaml
organization: example-org
members:
  - username: alice
    role: member
```

The values are fictional; replace them before running commands that contact
GitHub.

The complete newcomer walkthrough, including a fuller example, is in
[Getting started](docs/getting-started.md).

## Install

Go 1.25.0 or newer is required. The module declares its language version in
`go.mod` and may select the patched toolchain automatically when toolchain
switching is enabled.

For a local installation:

```bash
go install github.com/orang-gaboets/octostate/cmd/octostate@latest
```

Automation and control repositories should use an explicitly selected release
instead of `@latest` so their behavior is reproducible.

## Authentication

Commands that contact GitHub require exactly one authentication method:

- `--token`
- `--app-id`, `--installation-id`, and `--app-key-path`

Examples use `$GITHUB_TOKEN` as a placeholder. Do not put tokens, private keys,
or installation secrets in configuration files or documentation.

`config validate` and `audit diff` are offline once their required files exist.
The other GitOps commands read live GitHub state, and `config apply` can mutate
supported resources.

## GitOps quick start

Create `config/organization.yaml`, then run the read-only path:

```bash
# Validate desired state without contacting GitHub
octostate config validate --config-dir ./config

# Preview the live reconciliation plan
octostate config plan --config-dir ./config --token "$GITHUB_TOKEN"

# Run best-effort, non-mutating apply preflight
octostate config apply --config-dir ./config --token "$GITHUB_TOKEN" --check
```

Review the plan and preflight result before intentionally applying supported
create/update actions:

```bash
octostate config apply --config-dir ./config --token "$GITHUB_TOKEN"
```

`config apply --check` is best-effort preflight, not a transactional GitHub dry
run, and cannot guarantee that a later apply will succeed. Unsupported
destructive drift, such as `delete` and `remove` actions, is reported rather
than silently executed.

For the complete first-time workflow, including snapshots and offline drift
checks, see [Getting started](docs/getting-started.md).

## Engine and control-repository boundary

This repository documents the engine contract:

- desired state in `config/organization.yaml`;
- live planning and supported apply behavior;
- actual-state snapshots and offline diffing; and
- deterministic command output.

The control repository should own approval policy, branch strategy,
notifications, and workflow orchestration around `config validate`,
`config plan`, `config apply`, `audit pull`, and `audit diff`.

## Documentation

Start with the [documentation index](docs/README.md) or the
[getting-started guide](docs/getting-started.md).

### CLI reference

- [Config commands](docs/cli/config.md)
- [Audit commands](docs/cli/audit.md)
- [Primitive commands](docs/cli/primitives.md)

### GitOps

- [Config schema](docs/gitops/config-schema.md)
- [GitOps overview](docs/gitops/overview.md)
- [GitOps architecture](docs/gitops/architecture.md)
- [Control-repository integration](docs/gitops/control-repo-integration.md)

### Maintainers

- [Development](docs/maintainers/development.md)
- [Releases](docs/maintainers/releases.md)
- [Release readiness](docs/maintainers/release-readiness.md)

### Security

- [Security policy](SECURITY.md)

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for
details.
