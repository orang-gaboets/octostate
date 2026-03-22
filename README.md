# repo-builder

`repo-builder` is a GitHub organization operations CLI. It provides commands to manage and query common GitHub resources such as:

- repositories (create from template, edit, delete)
- topics (add, replace, list)
- teams (create, edit, get, delete, members list/add/remove, repo permissions list/add/remove)
- organization utilities (list repos/members/teams, get org by name)
- users (lookup by ID / username)

This repository is the **engine** (CLI + reusable automation building blocks).  
**Planned next:** a separate **control repo** will hold configuration + approvals (GitOps), and call into this engine via GitHub Actions.

## Status / Roadmap

- **v0 (current):** API-primitive commands (direct GitHub operations) ✅
- **v1 (in progress):** GitOps commands + audit:
  - ✅ `repo-builder config validate`
  - ✅ `repo-builder config sync-from-live --mode bootstrap`
  - ✅ `repo-builder config plan`
  - ✅ `repo-builder audit pull`
  - ✅ `repo-builder config apply`
  - ✅ `repo-builder audit diff`

## Installation

```
go install github.com/orang-gaboets/repo-builder/cmd/repo-builder@latest
```

## Development Setup

1. Clone the repository
    ```bash
    git clone https://github.com/orang-gaboets/repo-builder.git
    ```
2. Change to the project directory
    ```bash
    cd repo-builder
    ```
3. Install Go
    - Make sure you have Go installed on your system. You can download it from [the official Go website](https://golang.org/dl/).
    
    - If you have Go installed, you can check the version:
        ```bash
        go version
        ```
    
    Ensure that your Go version is 1.24 or higher.
\
4. Install dependencies
    ```bash
    go mod tidy
    ```
5. Install pre-commit
    ```bash
    pip install pre-commit
    ```
6. Install pre-commit hooks
    ```bash
    pre-commit install
    ```
7. Build the project (optional)
    ```bash
    go build -o bin/repo-builder ./cmd/repo-builder
    ```



## Usage

### How to run `repo-builder` commands:

Option 1: If you have installed the package globally:
```bash
repo-builder <command> [flags]
```

Option 2: If you are using the built binary:
```bash
./bin/repo-builder <command> [flags]
```

Option 3: If you want to run it directly using `go run`:
```bash
go run ./cmd/repo-builder <command> [flags]
```

This README uses canonical command names (for example `organization`,
`create-from-template`, `delete-by-slug`, `get-by-slug`). Common aliases such as
`org`, `repo create`, `team delete`, and `team get` also work.

Use `--verbose` (or `-v`) to enable diagnostic logs on stderr while keeping command results on stdout.

Command results are written to stdout as JSON:
- Query/list/get commands return resource payloads (object/array JSON).
- Mutating commands return an operation envelope with `status`, `message`, and `data` fields.

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

## Developer Commands

Common local development commands:

```bash
# Build
go build -o bin/repo-builder ./cmd/repo-builder

# Run help / smoke check
go run ./cmd/repo-builder --help

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

## Releases

This repository uses [`release-please`](https://github.com/googleapis/release-please)
and the
[`release-please-action`](https://github.com/googleapis/release-please-action)
to automate release PRs, changelog updates, Git tags, and GitHub Releases from
Conventional Commits on `main`.

Version updates for this project are performed by `release-please` using a
GitHub App installation token rather than a user-owned personal access token.

- Use Conventional Commit subjects on the commits that land on `main`.
- If you use squash merge, put the Conventional Commit prefix in the PR title.
- `fix:` produces a patch release, `feat:` produces a minor release, and
  `!` or `BREAKING CHANGE:` produces a major release.
- The workflow prefers a GitHub App installation token created with
  [`actions/create-github-app-token`](https://github.com/actions/create-github-app-token).
  If the app credentials are not configured, it falls back to `GITHUB_TOKEN`.
- In repository settings, enable "Allow GitHub Actions to create and approve
  pull requests" so the workflow can open release PRs.

Bootstrap the current CLI baseline as `v0.1.0` once before relying on automated
version bumps:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

After `v0.1.0` exists, future releasable commits on `main` will cause
`release-please` to open or update a release PR automatically.

### Auto-merge

This repository also uses
`.github/workflows/automerge-release-please.yml` to enable auto-merge for
`release-please` PRs created by the configured GitHub App.

- The workflow only targets same-repository PRs into `main`.
- The workflow only targets release branches created by `release-please`
  (`release-please--branches--*`).
- Human-authored PRs are intentionally ignored.

### Authentication

All commands require GitHub authentication. You must supply exactly one of the following:

- `--token` – personal access token (PAT).
- `--app-id`, `--installation-id`, and `--app-key-path` – GitHub App ID, installation ID, and path to the App's private key.

Providing both methods or neither results in an error.

### Safety Flags

Mutating commands support `--dry-run` to preview the requested action without
calling GitHub mutation APIs. Dry-run previews are input-based and may not
include remote-derived final state (for example topic merge results).

Destructive delete commands require explicit confirmation with `--yes`:
- `repo-builder repo delete`
- `repo-builder team delete-by-slug`

You can use `--dry-run` instead of `--yes` to preview a delete operation.

### Config (GitOps)

#### Validate desired-state configuration

```bash
repo-builder config validate --config-dir ./config
```

##### Flags
- `--config-dir` (required): Path to a directory containing `organization.yaml`

Behavior:
- Loads `<config-dir>/organization.yaml`
- Uses strict YAML decoding (unknown fields are rejected)
- Runs semantic validation (duplicates, enum checks, references, parent cycles, invite identity rules)
- Prints a JSON validation report to stdout
- Does not call GitHub APIs (fully offline)

Invite rules:
- Each invite must declare exactly one of `username`, `email`, or `user_id`
- Declaring more than one identity field is invalid
- Declaring none of the identity fields is invalid
- Declared `username` values must be valid GitHub usernames
- Declared `email` values must be valid email addresses
- Declared `user_id` values must be greater than zero
- Declared empty or whitespace-only `username` / `email` values are rejected
- Explicit `null` is rejected for `username`, `email`, and `user_id`

Repository field rules:
- `visibility` and `topics` are exact-reconcile fields
- `template.owner` and `template.name` remain create-time inputs for repository creation
- `description`, `homepage`, `allow_forking`, `archived`, and `is_template` are presence-aware optional fields
- If one of those optional repository fields is omitted, GitOps leaves it unmanaged
- Explicit empty strings for `description` or `homepage` clear those fields
- Boolean repository fields are only managed when explicitly set to `true` or `false`
- Explicit `null` is rejected for `description`, `homepage`, `allow_forking`, `archived`, and `is_template`
- `allow_forking` is still ignored for private repositories

Exit codes:
- `0`: valid configuration
- `2`: configuration loaded, but semantic validation failed
- `1`: load/decode/runtime failure (for example missing file or malformed YAML)

Example `organization.yaml`:

```yaml
organization: orang-gaboets
invites:
  - username: octocat
    role: direct_member
    team_slugs:
      - platform

repositories:
  - name: repo-builder
    visibility: private
    template:
      owner: orang-gaboets
      name: repo-template

teams:
  - slug: platform
    name: Platform
    privacy: closed
    members:
      - username: alice
        role: maintainer
    repositories:
      - name: repo-builder
        permission: push
```

#### Bootstrap desired-state config from live GitHub state

```bash
repo-builder config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config --token <token>
repo-builder config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config --token <token> --write
```

##### Flags
- `--mode` (required): Sync mode to run (currently only `bootstrap`)
- `--org` (required): GitHub organization to read from live state
- `--config-dir` (required): Path to the config directory containing or receiving `organization.yaml`
- `--write`: Write the generated `organization.yaml` into `--config-dir` instead of printing YAML to stdout
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Collects live GitHub organization state required for bootstrap generation
- Builds a canonical `organization.yaml` proposal from:
  - repositories
  - teams
  - team memberships
  - team repository permissions
- Prints the generated YAML to stdout by default
- Validates the generated config before printing or writing it
- With `--write`, writes `<config-dir>/organization.yaml`

Bootstrap rules:
- Pending invites are excluded by default
- Stable repository settings are emitted as an explicit baseline, including presence-aware optional repository fields
- `allow_forking` is omitted for private repositories
- Bootstrap fails if it detects direct organization members outside teams, because current GitOps config cannot represent them safely

Write behavior:
- `--write` fails if `<config-dir>/organization.yaml` already exists
- Writes are atomic: the file is written through a temp file and then renamed into place
- Existing-target checks happen before GitHub authentication and live collection

Example print-to-stdout use:

```bash
go run ./cmd/repo-builder config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config --token "$GITHUB_TOKEN"
```

Example write use:

```bash
go run ./cmd/repo-builder config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config --token "$GITHUB_TOKEN" --write
```

#### Preview reconciliation plan

```bash
repo-builder config plan --config-dir ./config --token <token>
```

##### Flags
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Loads `<config-dir>/organization.yaml`
- Runs semantic validation before contacting GitHub
- Collects current GitHub actual state using the GitOps collector layer
- Builds a deterministic, read-only reconciliation plan
- Prints the JSON plan report to stdout
- Does not mutate GitHub state
- Repository optional fields are only diffed when they are explicitly declared in config

Plan report fields:
- `organization`
- `summary`
- `actions`

Action behavior:
- Executable actions represent supported changes that a later `config apply`
  command can carry out, such as `create` and `update`
- Non-executable actions represent live drift that is detected but not yet
  automatically reconciled, such as `delete` and `remove`
- Action ordering is deterministic so CI output and PR comments stay stable

Example use:

```bash
go run ./cmd/repo-builder config plan --config-dir ./config --token "$GITHUB_TOKEN"
```

#### Apply reconciliation changes

```bash
repo-builder config apply --config-dir ./config --token <token> --dry-run
repo-builder config apply --config-dir ./config --token <token>
```

##### Flags
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--dry-run`: Build the live plan and print the executable/skipped actions without mutating GitHub
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Loads `<config-dir>/organization.yaml`
- Runs semantic validation before contacting GitHub
- Collects current GitHub actual state using the GitOps collector layer
- Builds the deterministic reconciliation plan used by `config apply`
- `--dry-run` prints the executable actions and skipped live drift without performing writes
- Live apply executes only supported executable `create` / `update` actions
- Unsupported live drift (`delete` / `remove`) is reported back as skipped drift and is not executed
- Repository creation currently requires `template.owner` and `template.name`
- Omitted optional repository fields are left unmanaged during apply
- Explicit empty `description` / `homepage` values are applied as clears
- Explicit boolean repository values are only applied when declared in config

Dry-run output fields:
- `status`
- `message`
- `data.organization`
- `data.plan_summary`
- `data.executable_actions`
- `data.skipped_actions`

Live apply output fields:
- `status`
- `message`
- `data.organization`
- `data.plan_summary`
- `data.executed_actions`
- `data.skipped_actions`

Exit codes:
- `0`: apply or dry-run completed successfully
- `2`: configuration loaded, but semantic validation failed
- `1`: load/auth/collection/planning/apply failure

Example dry-run:

```bash
go run ./cmd/repo-builder config apply --config-dir ./config --token "$GITHUB_TOKEN" --dry-run
```

Example live apply:

```bash
go run ./cmd/repo-builder config apply --config-dir ./config --token "$GITHUB_TOKEN"
```

### Audit

#### Pull actual-state snapshot

```bash
repo-builder audit pull --config-dir ./config --state-dir ./state --token <token>
```

##### Flags
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--state-dir` (required): Path to the state directory where the actual-state snapshot will be written
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Loads `<config-dir>/organization.yaml` to determine the target organization
- Collects current GitHub actual state using the GitOps collector layer
- Writes a stable JSON snapshot to:
  - `<state-dir>/actual/snapshot.json`
- Prints a structured success result to stdout
- Does not mutate GitHub state (read-only)

Snapshot fields:
- `pulled_at`
- `organization`
- `resolved_invite_user_ids_by_username`
- `members`
- `pending_invitations`
- `repositories`
- `teams`
- `team_members`
- `team_repo_permissions`

Example success output:

```json
{
  "status": "success",
  "message": "wrote actual-state snapshot",
  "data": {
    "path": "state/actual/snapshot.json",
    "organization": "orang-gaboets",
    "pulled_at": "2026-03-10T01:30:00Z"
  }
}
```

This snapshot feeds offline GitOps workflows such as `audit diff` and can also
support later reconciliation planning.

#### Diff desired state against the stored snapshot

```bash
repo-builder audit diff --config-dir ./config --state-dir ./state
repo-builder audit diff --config-dir ./config --state-dir ./state --fail-on-drift
```

##### Flags
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--state-dir` (required): Path to the state directory containing `actual/snapshot.json`
- `--fail-on-drift`: Exit with code `2` when any drift is detected

Behavior:
- Loads `<config-dir>/organization.yaml`
- Runs semantic validation before building the offline diff
- Loads the stored snapshot from:
  - `<state-dir>/actual/snapshot.json`
- Builds a deterministic offline drift report without calling GitHub APIs
- Prints the JSON drift report to stdout
- Uses the latest `audit pull` snapshot as its source of actual state

Drift report fields:
- `organization`
- `snapshot_pulled_at`
- `summary`
- `actions`

Drift behavior:
- Uses the same resource ordering and action schema as `config plan`
- Reports `create` / `update` drift for desired state that is missing or changed
- Reports `delete` / `remove` drift for unsupported extra snapshot state
- Does not mutate GitHub state

Exit codes:
- `0`: no drift, or drift detected without `--fail-on-drift`
- `2`: drift detected and `--fail-on-drift` is set
- `1`: load/decode/validation/runtime failure

Example offline diff:

```bash
go run ./cmd/repo-builder audit diff --config-dir ./config --state-dir ./state
```

Example CI-style drift gate:

```bash
go run ./cmd/repo-builder audit diff --config-dir ./config --state-dir ./state --fail-on-drift
```

### Organizations

#### Get Organization Details by Name

```bash
repo-builder organization get-by-name --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name

#### List Organization Repositories

```bash
repo-builder organization list-repos --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> [--type <all|public|private|forks|sources|member>]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--type` (optional): Type of repositories to list (default is "all")

#### List Organization Members

```bash
repo-builder organization list-members --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> [--role <all|admin|member>]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--role` (optional): Role of members to list (default is "all")

#### List Organization Teams

```bash
repo-builder organization list-teams --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name

#### Invite a User to an Organization

```bash
repo-builder organization invite --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> (--id <user-id> | --username <username>) [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--id` (required unless `--username` is provided): GitHub user ID to invite
- `--username` (required unless `--id` is provided): GitHub username to invite
- `--dry-run` (optional): Preview the invitation request without creating it (username lookups are skipped in dry-run mode)

### Repo

#### Get a Repository

```bash
repo-builder repo get --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name

#### Create a New Repository Based on a Template

```bash
repo-builder repo create-from-template --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> [--template-org <template-org>] --template-name <template-name> --org <org> --name <repo-name> [--desc <description>] [--topics <t1,t2>] [--private true|false] [--include-all-branches true|false] [--dry-run]
```

##### Flags

- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--template-org` (optional): Organisation that owns the template repository (defaults to `--org`)
- `--template-name` (required): Name of the template repository
- `--name` (required): New repository name
- `--desc` (optional): Repository description
- `--topics` (optional): Comma-separated list of repository topics
- `--private` (optional): Create a private repository (default is public)
- `--include-all-branches` (optional): Include all branches from the template repository (default is false)
- `--dry-run` (optional): Preview repository creation without creating it

#### Delete a Repository

```bash
repo-builder repo delete --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> (--yes | --dry-run)
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name
- `--yes` (required unless `--dry-run` is set): Confirm the destructive delete operation
- `--dry-run` (optional): Preview repository deletion without deleting it

#### Edit Repository Settings

```bash
repo-builder repo edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> [--desc <description>] [--homepage <homepage-url>] [--private true|false] [--is-template true|false] [--archived true|false] [--allow-forking true|false] [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name
- `--desc` (optional): Repository description
- `--homepage` (optional): Repository homepage URL
- `--private` (optional): Set repository to private/public
- `--is-template` (optional): Set or unset repository as a template
- `--archived` (optional): Archive/unarchive the repository
- `--allow-forking` (optional): Allow/disallow private forking of the repository
- `--dry-run` (optional): Preview repository edits without updating the repository

### Topic

#### Add Topics to a Repository

```bash
repo-builder topic add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --topics <t1,t2> [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name
- `--topics` (required): Comma-separated list of topics to add
- `--dry-run` (optional): Preview topic additions without updating the repository

#### List All Topics of a Repository

```bash
repo-builder topic list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name

#### Replace All Topics of a Repository

```bash
repo-builder topic replace --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --topics <t1,t2> [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name
- `--topics` (required): Comma-separated list of topics to set
- `--dry-run` (optional): Preview topic replacement without updating the repository

### Team

#### Create a New Team

```bash
repo-builder team create --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <team-name> [--desc <description>] [--secret true|false] [--parent <parent-team-slug>] [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Team name
- `--desc` (optional): Team description
- `--secret` (optional): Create a secret team (default is false)
- `--parent` (optional): Parent team slug (if creating a child team)
- `--dry-run` (optional): Preview team creation without creating the team

#### Edit a Team

```bash
repo-builder team edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> [--name <new-team-name>] [--desc <description>] [--secret true|false] [--parent <parent-team-slug> | --clear-parent] [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)
- `--name` (optional): New team name
- `--desc` (optional): New team description (pass an empty string to clear)
- `--secret` (optional): Set privacy to secret (`true`) or visible (`false`) when provided
- `--parent` (optional): Parent team slug to assign
- `--clear-parent` (optional): Remove the parent team relationship
- `--dry-run` (optional): Preview team edits without updating the team

#### Delete a Team

```bash
repo-builder team delete-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-name> (--yes | --dry-run)
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)
- `--yes` (required unless `--dry-run` is set): Confirm the destructive delete operation
- `--dry-run` (optional): Preview team deletion without deleting the team

#### Get Team by Slug

```bash
repo-builder team get-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-name>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)

#### List Team Members

```bash
repo-builder team members list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> [--role <all|member|maintainer>]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)
- `--role` (optional): Team member role filter (`all`, `member`, or `maintainer`; default is `all`)

#### Add Team Member

```bash
repo-builder team members add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --username <username> [--role <member|maintainer>] [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)
- `--username` (required): GitHub username to add/update in the team
- `--role` (optional): Team membership role (`member` or `maintainer`; default is `member`)
- `--dry-run` (optional): Preview the membership change without calling GitHub

#### Remove Team Member

```bash
repo-builder team members remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --username <username> [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)
- `--username` (required): GitHub username to remove from the team
- `--dry-run` (optional): Preview the membership removal without calling GitHub

#### List Team Repository Permissions

```bash
repo-builder team repo permissions list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)

#### Add Team Repository Permission

```bash
repo-builder team repo permissions add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --repo <repo-name> [--repo-org <repo-org>] [--permission <pull|push|admin|maintain|triage>] [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)
- `--repo` (required): Repository name
- `--repo-org` (optional): Owner organization of the repository (defaults to `--org`)
- `--permission` (optional): Permission to grant (`pull`, `push`, `admin`, `maintain`, `triage`; default is `pull`)
- `--dry-run` (optional): Preview the permission change without calling GitHub

#### Remove Team Repository Permission

```bash
repo-builder team repo permissions remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --repo <repo-name> [--repo-org <repo-org>] [--dry-run]
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)
- `--repo` (required): Repository name
- `--repo-org` (optional): Owner organization of the repository (defaults to `--org`)
- `--dry-run` (optional): Preview the permission removal without calling GitHub

### User

#### Get User Details by ID

```bash
repo-builder user get-by-id --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --id <user-id>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--id` (required): User ID

#### Get User Details by Username

```bash
repo-builder user get-by-username --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --username <username>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--username` (required): GitHub username

## Testing
To run the full test suite:

```bash
go test ./...
```

To generate a coverage report:

```bash
go test ./... -cover -coverprofile=coverage.out
```
## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## GitOps (In Progress)

This repository is the **engine** (CLI + reusable automation building blocks).  
The GitOps workflow will live in a separate **control repository** per organization (e.g. `<org>-control`), which will:

- store the **desired state** in `config/`
- require **PR review/approval** for changes
- apply changes to GitHub using this engine (via GitHub Actions + GitHub App auth)
- continuously snapshot reality into `state/` and report drift

### Control repo layout (v1 target)

```text
config/
  organization.yaml # desired organization state (invites, repositories, teams)
state/
  actual/           # generated snapshots from GitHub (never hand-edit)
  diff/             # drift reports (optional)
  events/           # append-only change log (optional)
```

### GitOps command status (engine)

Implemented:
- `repo-builder config validate` — validate `organization.yaml` schema + invariants offline
- `repo-builder config sync-from-live --mode bootstrap` — generate a canonical bootstrap `organization.yaml` from live GitHub state
- `repo-builder config plan` — preview deterministic reconciliation actions from desired vs live state
- `repo-builder audit pull` — snapshot GitHub into `state/actual/snapshot.json`
- `repo-builder config apply` — reconcile GitHub to match config/ for supported create/update actions
- `repo-builder audit diff` — compare desired config against the stored snapshot offline

Planned:
- bounded-concurrency collection for GitOps collectors
