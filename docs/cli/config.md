# Config Commands

The `config` command group drives the live GitOps workflow against an
`organization.yaml` desired-state file, typically `config/organization.yaml`.

Use these commands when you want to validate desired state, generate or update
that desired state from live GitHub, preview reconciliation, or apply the
supported portion of the plan.

For the canonical field-by-field schema, see
[Config schema](../gitops/config-schema.md).

Authentication rules:
- `octostate config validate` is fully offline and does not require GitHub auth.
- The other `config` commands require exactly one auth method:
  - `OCTOSTATE_GITHUB_TOKEN` (preferred for PAT authentication)
  - `--token` (supported for compatibility)
  - `--app-id`, `--installation-id`, and `--app-key-path`

Set `OCTOSTATE_GITHUB_TOKEN` before the PAT-authenticated examples below. The
same live commands also support GitHub App authentication with `--app-id`,
`--installation-id`, and `--app-key-path`.

When `--token` is supplied, including as `--token=`, it takes precedence over
`OCTOSTATE_GITHUB_TOKEN`. Octostate intentionally ignores `GH_TOKEN` and
`GITHUB_TOKEN`.

## Command Comparison

Use these commands at different points in the GitOps workflow:

| Command | Live GitHub access | Mutates GitHub | Primary use |
| --- | --- | --- | --- |
| `octostate config validate` | No | No | Validate `organization.yaml` before any live calls |
| `octostate config sync-from-live` | Yes | No | Bootstrap, adopt, or materialize desired state from live GitHub; `--write` can save `organization.yaml` locally |
| `octostate config plan` | Yes | No | Build the live reconciliation preview for review |
| `octostate config apply --dry-run` | Yes | No | Show the executable/skipped apply view without preflight probes or writes |
| `octostate config apply --check` | Yes | No | Validate the supported apply path with read-only probes before review or merge |
| `octostate config apply` | Yes | Yes | Execute supported create/update actions after approval |

## `octostate config validate`

Validate desired-state configuration.

```bash
octostate config validate --config-dir ./config
```

Flags:
- `--config-dir` (required): Path to a directory containing `organization.yaml`

Behavior:
- Loads `<config-dir>/organization.yaml`
- Uses strict YAML decoding (unknown fields are rejected)
- Runs semantic validation (duplicates, enum checks, references, parent cycles, invite identity rules)
- Prints a JSON validation report to stdout
- Does not call GitHub APIs (fully offline)

For the canonical schema details behind these validation rules, including
invite identities, repository field semantics, and supported enum values, see
[Config schema](../gitops/config-schema.md).

Exit codes:
- `0`: valid configuration
- `2`: configuration loaded, but semantic validation failed
- `1`: load/decode/runtime failure (for example missing file or malformed YAML)

Example `organization.yaml`:

```yaml
organization: orang-gaboets
members:
  - username: alice
    role: member

invites:
  - username: octocat
    role: direct_member
    team_slugs:
      - platform

repositories:
  - name: octostate
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
      - name: octostate
        permission: push
```

## `octostate config sync-from-live`

Build or update desired state from live GitHub.

### Bootstrap desired-state config from live GitHub state

```bash
export OCTOSTATE_GITHUB_TOKEN="<token>"
octostate config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config
octostate config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config --write
```

Flags:
- `--mode` (required): Sync mode to run (`bootstrap`, `adopt`, or `materialize`)
- `--org` (required): GitHub organization to read from live state
- `--config-dir` (required): Path to the config directory containing or receiving `organization.yaml`
- `--write`: Write the generated `organization.yaml` into `--config-dir` instead of printing YAML to stdout
- `--token`: Optional explicit GitHub personal access token; prefer `OCTOSTATE_GITHUB_TOKEN` for PAT authentication
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Collects live GitHub organization state required for bootstrap generation
- Builds a canonical `organization.yaml` proposal from:
  - organization members
  - repositories
  - teams
  - team memberships
  - team repository permissions
- Prints the generated YAML to stdout by default
- Validates the generated config before printing or writing it
- With `--write`, writes `<config-dir>/organization.yaml`

Bootstrap rules:
- Pending invites are excluded by default
- Top-level `members:` are emitted for collected durable organization membership
- Stable repository settings are emitted as an explicit baseline, including presence-aware optional repository fields
- `allow_forking` is omitted for private repositories
- Direct organization members outside teams are represented through top-level `members:`

Write behavior:
- `--write` fails if `<config-dir>/organization.yaml` already exists, including symlinks and dangling symlinks
- Writes are atomic: the file is written to a temp file, hard-linked into place at `<config-dir>/organization.yaml`, and then the temp file is removed
- Existing-target checks happen before GitHub authentication and live collection

Example print-to-stdout use:

```bash
octostate config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config
```

Example write use:

```bash
octostate config sync-from-live --mode bootstrap --org orang-gaboets --config-dir ./config --write
```

### Adopt supported live state into an existing desired config

```bash
octostate config sync-from-live --mode adopt --org orang-gaboets --config-dir ./config
octostate config sync-from-live --mode adopt --org orang-gaboets --config-dir ./config --write
```

Behavior:
- Loads and validates the existing `<config-dir>/organization.yaml` before contacting GitHub
- Collects live GitHub organization state required for adoption generation
- Merges supported live state back into config for:
  - top-level `members`
  - repositories
  - teams
  - team memberships
  - team repository permissions
- Preserves existing invites that are still transitional; removes invites already satisfied by live org membership
- Preserves existing config-only declarations; `adopt` does not auto-remove config that is missing from live state
- Prints the adopted YAML to stdout by default
- Validates the merged config before printing or writing it
- With `--write`, atomically replaces `<config-dir>/organization.yaml`
- `--write` requires `<config-dir>/organization.yaml` to directly reference an existing regular file; symbolic links and other non-regular targets are rejected before GitHub authentication or live collection

Adopt rules:
- Pending invites are excluded by default
- Top-level durable org membership is adopted into `members:`
- Presence-aware repository fields are only updated from live when they are already explicitly managed in config
- Newly adopted repositories leave presence-aware repository fields unmanaged; add those fields manually to `organization.yaml` if you want them explicit today
- Existing config order is preserved where possible; newly adopted entries append deterministically

Example print-to-stdout use:

```bash
octostate config sync-from-live --mode adopt --org orang-gaboets --config-dir ./config
```

Example write use:

```bash
octostate config sync-from-live --mode adopt --org orang-gaboets --config-dir ./config --write
```

### Materialize unmanaged repository fields in an existing desired config

```bash
octostate config sync-from-live --mode materialize --org orang-gaboets --config-dir ./config
octostate config sync-from-live --mode materialize --org orang-gaboets --config-dir ./config --write
```

Behavior:
- Loads and validates the existing `<config-dir>/organization.yaml` before contacting GitHub
- Collects only the live GitHub organization identity and repositories required for materialization
- Fills currently unmanaged optional repository fields from live state for already-declared repositories only
- Preserves `organization`, `members`, `invites`, `teams`, repository order, and already-managed repository fields
- Does not adopt live-only repositories or remove config-only declarations
- Prints the materialized YAML to stdout by default
- Validates the merged config before printing or writing it
- With `--write`, atomically replaces `<config-dir>/organization.yaml`
- `--write` requires `<config-dir>/organization.yaml` to directly reference an existing regular file; symbolic links and other non-regular targets are rejected before GitHub authentication or live collection

Materialize rules:
- Only these repository fields are materialized:
  - `description`
  - `homepage`
  - `allow_forking`
  - `archived`
  - `is_template`
- Empty live string values become explicit managed empty-string clears
- Boolean live values, including `false`, become explicit managed booleans
- `allow_forking` is not materialized for desired private repositories
- If a repository is not yet declared in config, adopt it first and then materialize optional fields afterward

Example print-to-stdout use:

```bash
octostate config sync-from-live --mode materialize --org orang-gaboets --config-dir ./config
```

Example write use:

```bash
octostate config sync-from-live --mode materialize --org orang-gaboets --config-dir ./config --write
```

## `octostate config plan`

Preview the live reconciliation plan.

```bash
octostate config plan --config-dir ./config
```

Flags:
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--token`: Optional explicit GitHub personal access token; prefer `OCTOSTATE_GITHUB_TOKEN` for PAT authentication
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Loads `<config-dir>/organization.yaml`
- Runs semantic validation before contacting GitHub
- Collects current GitHub actual state using the bounded-concurrency GitOps collector layer
- Builds a deterministic, read-only reconciliation plan
- Prints a Terraform-style split JSON preview to stdout
- Does not mutate GitHub state
- Repository optional fields are only diffed when they are explicitly declared in config

Plan preview fields:
- `organization`
- `plan_summary`
- `executable_actions`
- `skipped_actions`

Action behavior:
- `executable_actions` contains the supported changes that a later `config apply` command can carry out, such as `create` and `update`
- `skipped_actions` contains unsupported live drift that is detected but not automatically reconciled, such as `delete` and `remove`
- Managed same-organization repository dependencies are emitted in deterministic dependency-safe topological order, using normalized repository identity to break ready-action ties, so a required source create or `is_template: true` enabling update precedes its consumers
- Existing sources use their explicit final `is_template` value, or retain live state when the field is omitted or null; new sources are usable only with `is_template: true`. Unavailable sources propagate diagnostics transitively, and cycles report stable `template dependency cycle: ...` messages
- External, cross-organization, live-only, and other non-managed template references are not managed plan dependencies and remain apply-preflight concerns
- Team repository permission create/update actions reuse repository availability: they are executable only when the target exists or is available earlier in the dependency-safe plan; otherwise they remain in `skipped_actions`. This does not relax the organization-only ownership rule for team repository permissions
- Dependency metadata is internal; the public plan JSON has no dependency field
- Both arrays keep deterministic action ordering so CI output and PR comments stay stable

Example use:

```bash
octostate config plan --config-dir ./config
```

## `octostate config apply`

Apply supported reconciliation changes.

```bash
octostate config apply --config-dir ./config --check
octostate config apply --config-dir ./config --dry-run
octostate config apply --config-dir ./config
```

Flags:
- `--config-dir` (required): Path to a directory containing `organization.yaml`
- `--check`: Run apply preflight validation without mutating GitHub
- `--dry-run`: Build the live plan and print the executable/skipped actions without mutating GitHub
- `--token`: Optional explicit GitHub personal access token; prefer `OCTOSTATE_GITHUB_TOKEN` for PAT authentication
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)

Behavior:
- Loads `<config-dir>/organization.yaml`
- Runs semantic validation before contacting GitHub
- Collects current GitHub actual state using the bounded-concurrency GitOps collector layer
- Builds the deterministic reconciliation plan used by `config apply`
- `--check` runs apply preflight validation against the collected actual state without mutating GitHub
- `--check` uses read-only GitHub probes for supported apply targets, including template repositories, repository update targets, team update targets, username-based invites, invitation team slugs, and team repository permission targets
- `--check` inherits the same repository dependency gate for team repository permission targets, so a permission is only preflighted when its repository already exists or is created earlier in the same plan
- `--check` consumes the same dependency-safe action order as `config apply`, so final same-plan template state and newly created repositories are visible to later create preflight checks
- `--check` also uses normalized repository keys, so mixed-case same-plan repository references resolve to the same planned repository during preflight
- `--check` is a best-effort preflight: it continues through remaining executable actions and aggregates preflight failures deterministically. Repository-action failures are processed in plan order, while resource-specific dependency handling may defer other checks. It validates the supported apply executor inputs plus live read probes, but it is not a guaranteed GitHub transaction dry-run
- `--dry-run` prints the same split executable/skipped view as `config plan` without performing writes or read-only preflight probes
- `--check` does not pre-resolve top-level `members:` usernames, team-member changes, or email invites before apply; `user_id` invites can still trigger a login lookup during planning when they are matched against desired members or pending invitations, so `--check` and `--dry-run` can still fail before the live apply phase
- `--check` may still miss GitHub-side failures caused by permission changes, organization policy, rate limits, races after collection, live state changes after preflight, or other GitHub-side validation that only occurs during write-time execution
- `--check` and `--dry-run` are mutually exclusive
- Live apply executes only supported executable `create` / `update` actions
- Unsupported live drift (`delete` / `remove`) is reported back as skipped drift and is not executed
- Repository creation currently requires `template.owner` and `template.name`
- Omitted optional repository fields are left unmanaged during apply
- Explicit empty `description` / `homepage` values are applied as clears
- Explicit boolean repository values are only applied when declared in config

Check output fields:
- `status`
- `message`
- `data.organization`
- `data.plan_summary`
- `data.checked_actions`
- `data.skipped_actions`

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
- `0`: apply, check, or dry-run completed successfully
- `2`: configuration semantic validation failed, or mutually exclusive apply flags were used
- `1`: load/auth/collection/planning/check/apply failure

Example check:

```bash
octostate config apply --config-dir ./config --check
```

Example dry-run:

```bash
octostate config apply --config-dir ./config --dry-run
```

Example live apply:

```bash
octostate config apply --config-dir ./config
```
