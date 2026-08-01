# AGENTS.md

`octostate` is the GitHub organization operations CLI and GitOps engine.
This repository is the engine layer only. The separate control repository owns
organization-specific approval policy, branching, notifications, and workflow
orchestration around the engine.

## Repo Map

- `README.md`: project purpose, engine boundary, and high-level command model
- `cmd/octostate/`: CLI entry point, command wiring, and authentication
- `pkg/github/`: GitHub API clients and resource services
- `pkg/gitops/`: desired-state loading, planning, collection, and apply logic
- `.github/workflows/`: CI, security, and release automation
- `docs/gitops/overview.md`: GitOps flow and responsibility split
- `docs/gitops/architecture.md`: package layout and command flow
- `docs/gitops/control-repo-integration.md`: engine/control-repo boundary
- `docs/maintainers/development.md`: local development and test commands
- `docs/maintainers/releases.md`: release automation and maintainer rules
- `docs/maintainers/v1.0.0-readiness.md`: stable-release readiness evidence

## Durable Invariants

- Command results belong on stdout.
- Diagnostic logs and errors belong on stderr.
- JSON is the default machine-readable output unless a documented command
  explicitly uses another stdout format.
- Output should stay deterministic: normalize, sort, and stabilize results
  before printing or writing snapshots.
- Keep changes within the engine scope here; control-repo policy belongs in the
  control repository and the docs linked above.

## Validation by Change Type

Use [docs/maintainers/development.md](docs/maintainers/development.md) as the
baseline validation guide.

- Go code: run formatting, `go test ./...`, `go vet ./...`, and
  `golangci-lint run`.
- GitHub Actions changes: also run `actionlint`.
- GitOps concurrency or shared-state changes: also run the scoped race-detector
  suite from the maintainer guide.
- Documentation-only changes: verify links, file paths, and documented commands
  against the maintainer guide.

## Optional Graphify Workflow

Graphify is optional. Use it when architecture exploration, dependency tracing,
impact analysis, or whole-branch review would help.

Small, local changes do not require Graphify. Treat Graphify results as
navigation and analysis aids; verify important findings against the current
source, tests, docs, and Git diff before acting on them.

If Graphify is unavailable, fall back to normal repository inspection and the
existing Go and Git tooling.

Generated Graphify output is local working data only. Do not commit anything
under `graphify-out/`.

## Authoritative Guidance

Use the linked docs for the full contract instead of repeating policy here.
When changing behavior, keep this file short and update the authoritative docs
if the contract itself changes.
