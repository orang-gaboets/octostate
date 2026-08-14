# Scheduled govulncheck Monitoring Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use a focused execution workflow for this small, coupled change. Steps use checkbox syntax for tracking.

**Goal:** Add read-only daily and manually triggerable govulncheck monitoring for the latest `main` branch while preserving the existing PR/main vulnerability gate and avoiding duplicate scans.

**Architecture:** Move the existing govulncheck job out of the broad CI workflow into a dedicated `.github/workflows/govulncheck.yml`. The dedicated workflow owns pull-request-to-main, main-push, daily schedule, and `workflow_dispatch` triggers while retaining the existing job/check name. It uses the repository's pinned Go toolchain through `actions/setup-go@v7` with `go-version-file: go.mod` and the explicitly pinned `golang.org/x/vuln/cmd/govulncheck@v1.5.0` command. Maintainer documentation describes detection-only behavior and separate remediation.

**Tech Stack:** GitHub Actions YAML, `actions/checkout@v7`, `actions/setup-go@v7`, Go 1.25.13 from `go.mod`, govulncheck v1.5.0, actionlint v1.7.12.

## Global Constraints

- Issue owner: #222; related context: #148 (existing govulncheck capability), #220 and #221 (toolchain remediation).
- Intended branch: `ci/scheduled-govulncheck`; actual base: `ci/go-toolchain-1.25.13` at `6f1d9c7`.
- Daily schedule: `03:24 UTC` (`24 3 * * *`), matching the repository's existing CodeQL schedule minute.
- Workflow permissions: top-level `contents: read`; no write permissions or mutation steps.
- Do not modify `go.mod`, `go.sum`, application/GitOps code, CodeQL/Dependabot schedules, branch protection, or dependency/toolchain versions.
- Do not use `continue-on-error`, suppression, automatic remediation, commits, branches, pull requests, or pushes in the workflow.

---

### Task 1: Extract govulncheck into a dedicated monitoring workflow

**Files:**
- Create: `.github/workflows/govulncheck.yml`
- Modify: `.github/workflows/ci.yml` — remove only the existing `govulncheck` job; leave all other jobs and triggers unchanged.

**Implementation:**

- Set workflow name to `Go vulnerability monitoring`.
- Configure `pull_request` for `main`, `push` for `main`, `schedule` with `cron: "24 3 * * *"`, and `workflow_dispatch`.
- Set `permissions: contents: read`.
- Define one `ubuntu-latest` job named `Go vulnerability scan`, with no event-suppressing `if` guard.
- Use `actions/checkout@v7`.
- Use `actions/setup-go@v7` with `go-version-file: go.mod` and `cache: true`.
- Run `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...` as a normal failing step.
- Add comments explaining that scheduled/manual runs detect newly disclosed vulnerabilities and do not remediate them.
- Preserve the job name so moving the job does not intentionally change the existing status-check identity.
- Remove the old job from `ci.yml` so PR and main-push scans run exactly once; do not change lint, actionlint, build, GitOps race, or Windows config jobs.

### Task 2: Document the monitoring and remediation boundary

**Files:**
- Modify: `docs/maintainers/development.md` in the existing govulncheck/development workflow section.

**Implementation:**

- Retain the existing local govulncheck and manual pre-commit commands.
- Add a maintainer note stating that `.github/workflows/govulncheck.yml` runs on main-targeting pull requests, pushes to main, a daily schedule, and manual dispatch.
- State that the workflow is detection-only, reads the normal Go vulnerability database, fails visibly on reachable findings, and requires separate issue/PR remediation.
- Do not document automatic upgrades, generated remediation PRs, finding suppression, or frozen vulnerability data.

### Task 3: Verify workflow correctness and unchanged repository behavior

**Checks:**

- Run `go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12` and require all workflow files to pass.
- Run `git diff --check` and require no whitespace errors.
- Inspect the workflow diff for the four triggers, `contents: read`, the pinned `@v1.5.0` command, `go-version-file: go.mod`, absence of write/mutation steps, and exactly one workflow definition of the PR/main govulncheck job.
- Run `go test ./...`, `go vet ./...`, `golangci-lint run --timeout=5m`, `go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...`, `go test -race ./pkg/gitops/...`, and the documented config-replacement package test command. The Windows-specific acceptance remains covered by the unchanged GitHub Actions `windows-latest` job.
- Confirm the CI diff leaves all non-govulncheck jobs and their behavior unchanged.
- After authorized publication, verify GitHub Actions reports the new workflow for PR, push, schedule/manual configuration and that the existing required vulnerability check remains visible; no remote mutation is part of this plan.

**Commit:** One cohesive implementation commit containing the workflow extraction and maintainer documentation, for example `ci: schedule govulncheck monitoring`. The plan artifact is committed separately before implementation.

## Requirement-to-Test Matrix

| Requirement | Change | Evidence |
| --- | --- | --- |
| PRs targeting `main` and pushes to `main` continue scanning | Dedicated workflow triggers; old CI job removed | Workflow inspection and GitHub Actions run |
| Daily and manual monitoring | `schedule` and `workflow_dispatch` triggers | YAML validation and trigger inspection |
| Pinned Go and govulncheck versions | `go-version-file: go.mod`; `@v1.5.0` | Workflow inspection and scan output |
| Read-only, detection-only behavior | `contents: read`; no mutation steps or suppression | Workflow inspection and failing-step semantics |
| No duplicate scans; other CI unchanged | Move only the govulncheck job | Diff review and actionlint |
| Maintainer remediation boundary documented | Development guide note | Documentation review |

## Plan Review

Two independent first-pass reviews were completed before approval:

- Requirements/acceptance: no material findings.
- Design/YAGNI/risk: no material findings.

Evidence limits recorded by both reviewers: branch-protection settings cannot be verified locally, and manual dispatch relies on selecting the intended ref. Neither limitation changes the approved scope.
