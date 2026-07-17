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

   The module includes a `toolchain go1.25.11` directive, so Go commands will
   automatically prefer the patched 1.25.11 toolchain when toolchain switching
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
go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
actionlint
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

The `govulncheck` hook is manual so you can run it on demand without slowing
every commit.

The race-detector check stays scoped to `pkg/gitops/...` because that tree
contains the bounded-concurrency collector, planner, and apply packages. That
keeps the check focused on the code most likely to hide shared-memory races
while avoiding a slower full-repository `-race` pass on every PR.

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
