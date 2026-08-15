# Development

## Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/orang-gaboets/octostate.git
   cd octostate
   ```

2. Install Go 1.25.0 or higher:

   ```bash
   go version
   ```

   The module includes a `toolchain go1.25.13` directive, so Go commands will
   automatically prefer the patched 1.25.13 toolchain when toolchain switching
   is enabled.

3. Install module dependencies:

   ```bash
   go mod tidy
   ```

4. Install `pre-commit`:

   ```bash
   pip install pre-commit
   pre-commit install
   ```

5. Build the project if you want a local binary:

   ```bash
   go build -o bin/octostate ./cmd/octostate
   ```

## Common Commands

Use these examples when working from source during development. User-facing
CLI docs prefer the installed `octostate` binary.

```bash
# Build
go build -o bin/octostate ./cmd/octostate

# Run help / smoke check
go run ./cmd/octostate --help

# Format (check / write)
gofmt -l .
gofmt -w <files>

# Static checks
go vet ./...
golangci-lint run --timeout=5m
go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
go test -race ./pkg/gitops/...

# Tests
go test ./...
go test ./... -cover -coverprofile=coverage.out

# Pre-commit hooks
pre-commit install
pre-commit run --all-files
pre-commit run govulncheck --hook-stage manual --all-files
```

The `actionlint` hook runs automatically for workflow files in the local
pre-commit setup. The first `go run ...actionlint...` invocation downloads the
tool if it is not already cached.

The `govulncheck` hook is manual so you can run it on demand without slowing
every commit.

The dedicated `.github/workflows/govulncheck.yml` workflow runs on pull
requests, pushes to `main`, a daily schedule, and manual
dispatch. It is detection-only: it queries the normal Go vulnerability
database and fails visibly when reachable vulnerabilities are found. Handle
remediation separately through the normal issue and pull-request workflow; the
monitoring workflow does not modify dependencies or the Go toolchain.

The separate `.github/workflows/go-toolchain-remediation.yml` workflow is the
only repository automation that may propose Go toolchain remediation. It does
not change #223's detection-only contract. The remediation workflow runs only
on its own daily schedule and `workflow_dispatch`, and the remediation job
itself exits unless `github.ref` is exactly `refs/heads/main`. It checks out
trusted `main`, fetches `origin/main`, and switches to the exact detached
`origin/main` commit before it runs the classifier or mints any write-capable
token.

`go.mod` remains the source of truth for the pinned toolchain directive, and
this development guide is the designated duplicate maintainer reference for
that value. The remediation workflow updates only `go.mod` and this file's
setup section when it proposes a patch-line toolchain bump; it must leave the
`go` language-version directive unchanged.

The remediation workflow pins the vulnerability scanner to
`golang.org/x/vuln/cmd/govulncheck@v1.5.0` for both the initial and
post-update structured scans. Its write-capable token step uses
`actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1`
with only `contents: write` and `pull-requests: write`. Organization-level
Actions configuration for that maintenance App stays external to this
repository. The workflow expects the organization variable
`GO_TOOLCHAIN_REMEDIATION_APP_ID` and organization secret
`GO_TOOLCHAIN_REMEDIATION_APP_PRIVATE_KEY`, scoped to repositories approved to
run this workflow. This documentation does not claim that the App installation,
secret values, or any branch ruleset configuration have already been verified.

When the classifier finds an eligible same-minor Go patch upgrade, the
remediation workflow proposes a draft PR from a deterministic branch named
`ci/go-toolchain-X.Y.Z`. The generated commit and PR title are both
`ci: bump Go toolchain to X.Y.Z`. Duplicate detection recognizes the stable PR
marker `<!-- octostate-go-toolchain-remediation:v1 -->` or the deterministic
`ci/go-toolchain-` branch prefix. Exact duplicates additionally require the
expected repository, `main` base, App bot, head, and current/target metadata.
The pinned-base metadata comment is recorded for audit/recovery context only; it
is not used as the duplicate-match predicate. A matching existing PR causes a
no-op exit; a different recognized remediation PR, an unexpected branch
collision, changed `main`, invalid classifier output, failed validation, or any
GitHub API/authentication problem fails closed without rewriting existing
remediation work.

If the workflow creates the remote branch but cannot open the draft PR, it
fails and leaves orphan-branch recovery to a maintainer: confirm whether a
matching draft PR already exists, then either create the draft PR manually
with the same marker or delete the orphan branch after verifying that no PR is
attached to it. The automation never force-pushes, approves, marks ready,
merges, deletes, or rewrites remediation branches or PRs; human review starts
only after the draft PR exists.

The race-detector check stays scoped to `pkg/gitops/...` because that tree
contains the bounded-concurrency collector, planner, and apply packages. That
keeps the check focused on the code most likely to hide shared-memory races
while avoiding a slower full-repository `-race` pass on every PR.

For config replacement changes, run the targeted Windows-oriented coverage
used by CI:

```bash
go test ./cmd/octostate/internal/filereplace ./cmd/octostate/internal/configproposal ./cmd/octostate/config
```

GitHub Actions runs that check on `windows-latest`.

## Optional Graphify

Graphify is an optional local workflow for repo exploration, not a required
development dependency.

Install the CLI first if needed:

```bash
uv tool install graphifyy
```

The PyPI distribution is `graphifyy`; it installs the `graphify` CLI.

Recommended extraction flow:

```bash
graphify extract . --code-only
```

That writes the local graph into `graphify-out/`, which stays untracked. From
there, use `graphify query`, `graphify path`, or `graphify affected` when you
need architecture, dependency, or impact analysis.

## Testing

Run the full test suite:

```bash
go test ./...
```

Generate a coverage report:

```bash
go test ./... -cover -coverprofile=coverage.out
```

## Command Output Conventions

When developing or reviewing commands, remember:
- diagnostic logs belong on stderr
- command results belong on stdout, with JSON as the default for most commands
- documented exceptions may use a different stdout format; for example, `octostate config sync-from-live` outputs YAML by default unless `--write` is used
- query/list/get commands return resource payloads
- mutating commands return an operation envelope with `status`, `message`, and `data`
