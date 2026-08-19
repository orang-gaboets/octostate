# Octostate documentation

Octostate is a GitHub organization operations CLI and GitOps engine. This
index points from common reader goals to the existing authoritative
documentation.

## New to Octostate?

- [Getting started](getting-started.md) — install Octostate, validate desired
  state, inspect a live plan, run preflight, and understand the optional apply
  and audit paths.
- [Project README](../README.md) — concise project overview and core mental
  model.

## Using the CLI

- [Config commands](cli/config.md) — validate, sync, plan, and apply desired
  state.
- [Audit commands](cli/audit.md) — pull actual-state snapshots and diff them
  offline.
- [Primitive commands](cli/primitives.md) — direct organization, repository,
  topic, team, and user operations.

## Understanding GitOps

- [GitOps overview](gitops/overview.md) — desired state, live planning,
  preflight, apply boundaries, snapshots, and offline diffing.
- [Config schema](gitops/config-schema.md) — the field-by-field
  `config/organization.yaml` contract.
- [GitOps architecture](gitops/architecture.md) — package ownership and
  command data flow.

## Building control-repository automation

- [Control-repository integration](gitops/control-repo-integration.md) — the
  boundary between the engine and organization-specific workflow orchestration.

## Developing or maintaining Octostate

- [Development](maintainers/development.md) — local setup, checks, and testing.
- [Releases](maintainers/releases.md) — release automation and maintainer
  procedures.
- [Release readiness](maintainers/release-readiness.md) — release validation
  evidence and readiness checks.

## Reporting a vulnerability

See the repository’s [security policy](../SECURITY.md) for responsible
disclosure instructions.
