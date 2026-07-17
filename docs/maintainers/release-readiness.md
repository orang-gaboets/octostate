# Release Readiness

This document defines the future-facing release validation policy for
`octostate`.

Historical evidence for the original `v1.0.0` stable release lives in
[`v1.0.0-readiness.md`](./v1.0.0-readiness.md). Keep that file as historical
evidence only; use this guide for future release validation.

## Release Tiers

### Patch releases: `vX.Y.Z`

Most patch releases should use a lightweight checklist:

- CI passes
- CodeQL and security checks pass
- the `release-please` PR and changelog look correct
- targeted tests exist for the affected change
- the affected command or path is smoke-tested when appropriate

A full readiness pass is usually not required for a normal patch release.

### Risky patch releases

If a patch release touches a high-risk area, it should get targeted extra
validation focused on the affected path instead of replaying the full
readiness checklist.

High-risk areas include:

- `config apply`
- `config apply --check`
- `config plan`
- `audit pull`
- `audit diff`
- `sync-from-live`
- authentication
- release automation
- security-sensitive behavior

### Minor releases: `vX.Y.0`

Meaningful minor releases should receive the standard readiness pass below when
they introduce or materially change behavior, especially GitOps behavior, CLI
behavior, apply/check semantics, authentication behavior, or release
automation.

For documentation-only or very small minor releases, a lighter validation pass
is acceptable, but that decision should be explicit.

### Standard readiness

The standard readiness pass should include:

- CI passes
- CodeQL and security checks pass
- the `release-please` PR and changelog look correct
- CLI smoke checks cover the affected command or path
- affected offline GitOps checks pass
- targeted live sandbox checks pass when the change touches live behavior
- release automation checks pass
- evidence is recorded in the relevant PR or maintainer doc

### Major releases: `vX.0.0`

Major releases should always receive a full readiness pass.

The full readiness pass for a major release should include everything in
standard readiness, plus:

- repository health checks
- full offline GitOps coverage
- full live sandbox GitOps coverage
- `sync-from-live` checks
- migration or breaking-change notes, when applicable
- complete evidence recording

## Missed Readiness After Release

If a meaningful release was already published without a readiness pass, do not
create retroactive readiness evidence.

Instead, record a post-release verification note in the relevant PR, issue, or
maintainer doc.

If extra confidence is needed for `v1.1.0`, run a small post-release
verification and record it as post-release verification, not pre-release
readiness.

## Evidence To Record

For larger releases, record the following fields:

- date
- operator
- release version or candidate
- commit SHA
- Go version
- commands run
- sandbox org or resources used
- result summary
- follow-up issues
