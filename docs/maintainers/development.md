# Development

## Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/orang-gaboets/octostate.git
   cd octostate
   ```

2. Install Go 1.26.11 or later in the 1.26 series, or Go 1.27.4 or later:

   ```bash
   go version
   ```

   The module's `go 1.26.11` directive implies the `go1.26.11` toolchain.
   Go commands automatically select it when toolchain switching is enabled.

3. Install module dependencies:

   ```bash
   go mod tidy
   ```

4. Optionally install `pre-commit` for the recommended local hook workflow.
   It is not required to build, test, or contribute; CI remains authoritative
   for required checks:

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

`go.mod` remains the source of truth for the selected toolchain, and
this development guide is the designated duplicate maintainer reference for
that value. The remediation workflow updates only `go.mod` and this file's
setup section when it proposes a patch-line toolchain bump; it must leave the
`go` language-version directive unchanged. When no explicit `toolchain`
directive exists, the helper reads the implicit version from `go` and adds a
`toolchain` directive for the newer patch.

The remediation workflow pins the vulnerability scanner to
`golang.org/x/vuln/cmd/govulncheck@v1.5.0` for both the initial and
post-update structured scans. Its write-capable token step uses
`actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1`
with only `contents: write` and `pull-requests: write`. Organization-level
Actions configuration for the maintenance App stays external to this
repository. The workflow declares the protected `go-toolchain-remediation`
environment with deployment record creation disabled because the environment
is used for credential scoping and access control rather than application
deployment. It expects the organization Client ID variable
`GO_TOOLCHAIN_REMEDIATION_APP_CLIENT_ID` plus the
`GO_TOOLCHAIN_REMEDIATION_APP_PRIVATE_KEY` secret in that environment. Configure
the environment's branch policy to allow `main` only, and remove any old
organization-level copy of the private-key secret after the environment secret
is populated. This documentation does not claim that the environment, App
installation, secret values, or branch ruleset configuration have already been
verified. The maintenance App must not be a `main` branch or ruleset bypass
actor.

The remediation workflow sources `.github/scripts/git-remote-auth.sh` for its
three authenticated branch-existence checks. The helper supplies the App token
through Git's environment-backed configuration, keeping it out of the `git`
process argument list and limiting the credential to each command invocation.

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
go test ./internal/filereplace ./internal/contributors ./pkg/gitops/config ./pkg/gitops/snapshot ./cmd/octostate/internal/configproposal ./cmd/octostate/config
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

## Documentation Assets

Update `docs/assets/diagrams/gitops-flow.mmd` first, then regenerate the
committed SVG from the repository root:

```bash
npx --yes --package @mermaid-js/mermaid-cli@11.12.0 mmdc \
  -i docs/assets/diagrams/gitops-flow.mmd \
  -o docs/assets/diagrams/gitops-flow.svg \
  --backgroundColor transparent
```

## Testing

Run the full test suite:

```bash
go test ./...
```

### Generated-output serialization memory observations

The generated YAML and snapshot JSON writers were measured on Go 1.26.11,
macOS 26.6.2 (build 25G83), Darwin arm64. The same host was recorded as an
Apple M4 Pro with 24 GiB of memory in the Issue #349 benchmark. The base was
`b7f53e9f9beefa3f3e0ac7d30544eacb7722ffdc`; the final implementation was
`f4c46554fc7b51a20924a908b6001e267c64f301`.

Each case used one warm-up and five measured pairs of fresh serial processes,
with base/final order alternated by pair. The model-only process constructed
the same fixture and seeded snapshot destination without calling the writer.
Paired RSS is API-process peak RSS minus its matching model-only peak RSS;
sampled heap is the maximum API-process heap above retained heap after setup
and GC; `TotalAlloc` and elapsed time cover the API call. Values below are
medians with the full five-run range in brackets. The fixtures reproduce the
published repository counts and exact output sizes from #349, but the original
#349 harness and complete record layouts were not available in this checkout;
these are deterministic local reconstructions, not a byte-for-byte rerun of
that unavailable harness.

| Fixture | Variant | Expected output | Model peak RSS | API peak RSS | Paired RSS increase | Sampled heap increase | `TotalAlloc` | Elapsed |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| YAML, 20,000 repositories | Base | 4,754,668 B | 19.31 [19.20–19.62] MiB | 505.59 [428.09–505.84] MiB | 486.25 [408.69–486.39] MiB | 360.28 [360.27–360.28] MiB | 708.66 [708.65–708.66] MiB | 163.74 [162.78–165.45] ms |
| YAML, 20,000 repositories | Final | 4,754,668 B | 19.33 [19.22–19.45] MiB | 54.50 [47.45–55.55] MiB | 35.23 [28.11–36.22] MiB | 32.95 [24.30–33.13] MiB | 435.80 [435.80–435.80] MiB | 113.14 [111.12–113.85] ms |
| JSON, 256 repositories, accepted | Base | 134,217,727 B | 140.80 [140.75–140.92] MiB | 657.78 [656.55–657.84] MiB | 516.92 [515.75–517.09] MiB | 514.45 [514.44–514.45] MiB | 773.56 [773.55–773.56] MiB | 860.52 [849.84–880.45] ms |
| JSON, 256 repositories, accepted | Final | 134,217,727 B | 140.78 [140.72–140.83] MiB | 141.41 [141.38–141.50] MiB | 0.62 [0.55–0.77] MiB | 0.39 [0.39–0.39] MiB | 0.39 [0.39–0.39] MiB | 347.42 [346.57–352.45] ms |
| JSON, 256 repositories, rejected | Base | 134,217,729 B | 140.86 [140.70–140.91] MiB | 656.70 [528.42–656.89] MiB | 516.00 [387.59–516.03] MiB | 383.92 [383.91–383.92] MiB | 643.03 [643.02–643.03] MiB | 423.02 [419.86–427.30] ms |
| JSON, 256 repositories, rejected | Final | 134,217,729 B | 140.78 [140.73–140.89] MiB | 141.45 [141.39–141.59] MiB | 0.67 [0.56–0.81] MiB | 0.40 [0.40–0.40] MiB | 0.40 [0.40–0.40] MiB | 342.25 [337.78–342.95] ms |

The near-limit JSON case succeeded and the over-limit case returned the
existing 128 MiB error; both implementations preserved the destination on
rejection. Separate final diagnostics also accepted the 16 MiB-minus-one-byte
YAML output and rejected the YAML byte, node, and oversized-scalar cases plus
the JSON object-entry and oversized-scalar cases. The JSON rejection cases
preserved the seeded destination. The fixture generator, raw process
measurements, and aggregation data remain in the ignored local `out/` directory
and are not part of the repository.

### Structured-input memory observations

The public `config.LoadFile` and `snapshot.ReadActual` paths were measured on
Go 1.26.10, macOS arm64, Apple M4 Pro using the previously reported adversarial
fixture shapes plus the empty-block-scalar YAML variant. Each fixture was
padded with valid whitespace to its documented byte limit; peak RSS includes
the Go runtime and input buffer. All cases were rejected during YAML
source-complexity or JSON array-element preflight, before typed decoding. Peak
RSS was read in-process with `getrusage`.

| Fixture shape | Input size | Rejection | Peak RSS |
| --- | ---: | --- | ---: |
| YAML with 400,000 member entries | 16 MiB | 1.1 million source-complexity units | 46.6 MiB |
| YAML with one repository and a large topics flow sequence | 16 MiB | 1.1 million source-complexity units | 46.5 MiB |
| YAML with one team and a large members flow sequence | 16 MiB | 1.1 million source-complexity units | 46.4 MiB |
| YAML with an empty block scalar before sibling topic entries | 16 MiB | 1.1 million source-complexity units | 46.5 MiB |
| JSON with one repository and a large topics array | 128 MiB | 1 million total array elements | 284.8 MiB |
| JSON with an array of empty member objects | 128 MiB | 1 million total array elements | 280.9 MiB |

These are observations for this Go and host version, not portable upper-bound
guarantees. Re-measure after changing input budgets, parser implementations,
or supported Go versions.

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
