# octostate

`octostate` is a GitHub organization operations CLI and GitOps engine.

It provides two main layers:
- **API primitives** for direct GitHub operations on organizations,
  repositories, topics, teams, and users
- **GitOps commands** for validating desired state, building plans, applying
  supported changes, pulling snapshots, and diffing drift

This repository is the **engine** (CLI + reusable automation building blocks).
A separate admin or control repository should own the organization-specific
approval workflow and automation policy around that engine.

## Current Scope

Shipped command groups:
- `config validate`
- `config sync-from-live --mode bootstrap`
- `config sync-from-live --mode adopt`
- `config sync-from-live --mode materialize`
- `config plan`
- `config apply`
- `audit pull`
- `audit diff`
- primitive `organization`, `repo`, `topic`, `team`, and `user` commands

## Installation

```bash
go install github.com/orang-gaboets/octostate/cmd/octostate@latest
```

## Authentication

All commands that talk to GitHub require exactly one auth method:
- `--token`
- `--app-id`, `--installation-id`, and `--app-key-path`

Most examples use `$GITHUB_TOKEN` for brevity, but every live command can also
use GitHub App authentication with `--app-id`, `--installation-id`, and
`--app-key-path`.

`octostate config validate` and `octostate audit diff` are offline once
their required files are present.

## CLI Basics

This README and the docs use canonical command names (for example
`organization`, `create-from-template`, `delete-by-slug`, and `get-by-slug`).
Common aliases such as `org`, `repo create`, `team delete`, and `team get`
also work.

Use `--verbose` (or `-v`) to enable diagnostic logs on stderr while keeping
command results on stdout.

Most command results are written to stdout as JSON:
- query/list/get commands return resource payloads
- mutating commands return an operation envelope with `status`, `message`, and
  `data`
- documented exceptions may use a different stdout format; for example,
  `config sync-from-live` outputs YAML by default unless `--write` is used

Example mutating output:

```json
{
  "status": "success",
  "message": "Created repository acme/new-repo from template acme/template",
  "data": {
    "owner": "acme",
    "name": "new-repo"
  }
}
```

Errors and diagnostics (including `--verbose` logs) are written to stderr.

## GitOps Quickstart

If you are using `octostate` as a GitOps engine, the normal flow is:

1. Start with `config/organization.yaml`
2. Validate it offline
3. Preview the live plan
4. Run `config apply --check` as a read-only preflight before approval
5. Apply supported changes after approval
6. Pull a snapshot and use offline diff where needed

Example flow:

```bash
# Validate desired state offline
octostate config validate --config-dir ./config

# Preview live reconciliation
octostate config plan --config-dir ./config --token "$GITHUB_TOKEN"

# Run read-only apply preflight checks before approval
octostate config apply --config-dir ./config --token "$GITHUB_TOKEN" --check

# Apply supported create/update actions
octostate config apply --config-dir ./config --token "$GITHUB_TOKEN"

# Refresh the stored actual-state snapshot
octostate audit pull --config-dir ./config --state-dir ./state --token "$GITHUB_TOKEN"

# Compare desired state against the stored snapshot offline
octostate audit diff --config-dir ./config --state-dir ./state --fail-on-drift
```

`config apply --check` is a best-effort preflight. It helps catch supported
apply-path problems before approval, but it is not a transactional dry-run and
cannot guarantee that a later live apply will succeed.

For the full command behavior and workflow details, see the
[GitOps overview](docs/gitops/overview.md), the
[Control-repo integration](docs/gitops/control-repo-integration.md), and the
[GitOps architecture](docs/gitops/architecture.md).

If you are starting from existing live GitHub state, `config sync-from-live`
can generate or update `config/organization.yaml` for bootstrap, adopt, or
materialize flows.

## Engine vs Control Repo

This repository documents the engine contract:
- desired-state input: `config/organization.yaml`
- live planning and apply behavior
- actual-state snapshot shape and offline diff behavior
- deterministic output and CLI contracts

The detailed admin or control-repo workflow should live in the control repo,
not here. That includes approval policy, branching strategy, notifications, and
workflow orchestration around when `validate`, `plan`, `apply`, `pull`, and
`diff` run.

## Docs

### CLI Reference
- [Config commands](docs/cli/config.md)
- [Audit commands](docs/cli/audit.md)
- [Primitive commands](docs/cli/primitives.md)

### GitOps
- [GitOps overview](docs/gitops/overview.md)
- [GitOps architecture](docs/gitops/architecture.md)
- [Control-repo integration](docs/gitops/control-repo-integration.md)

### Maintainers
- [Development](docs/maintainers/development.md)
- [Releases](docs/maintainers/releases.md)
- [v1.0.0 readiness](docs/maintainers/v1.0.0-readiness.md)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE)
file for details.
