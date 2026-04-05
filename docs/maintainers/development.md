# Development

## Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/orang-gaboets/octostate.git
   cd octostate
   ```

2. Install Go 1.24 or higher:

   ```bash
   go version
   ```

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

# Tests
go test ./...
go test ./... -cover -coverprofile=coverage.out

# Pre-commit hooks
pre-commit install
pre-commit run --all-files
```

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
