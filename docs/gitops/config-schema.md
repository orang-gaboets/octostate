# `config/organization.yaml` Schema Reference

> Canonical reference for the desired-state file used by `octostate config
> validate`, `config plan`, `config apply`, and `config sync-from-live`.

`config/organization.yaml` is the declarative desired state for a GitHub
organization. It can describe:

- the organization itself
- durable organization members
- pending invitations
- repositories
- teams
- team memberships
- team repository permissions

## Top-Level Fields

- `organization` - required string
- `members` - optional list of durable organization members
- `invites` - optional list of transitional pending invites
- `repositories` - optional list of repositories to manage
- `teams` - optional list of teams to manage

## Members

Each top-level member entry requires:

- `username`
- `role`

Supported member roles:

- `admin`
- `member`

## Invites

Each invite must declare exactly one identity field:

- `username`
- `email`
- `user_id`

Other invite fields are optional:

- `role`
- `team_slugs`

Supported invite roles:

- `admin`
- `direct_member`
- `billing_manager`

Invite team assignments go in `team_slugs`. Each listed slug must already
match a declared team.

Validation rules:

- `username` values must be valid GitHub usernames
- `email` values must be valid email addresses
- `user_id` values must be greater than zero
- empty or whitespace-only `username` / `email` values are rejected
- explicit `null` is rejected for `username`, `email`, and `user_id`
- a `username` invite that duplicates a declared top-level member is rejected
- two invites must not declare the same identity

Duplicate invite identities are compared within a single identity kind, after
trimming:

- `username` against `username`, case-insensitively, matching how GitHub
  usernames are compared everywhere else in the schema
- `email` against `email`, case-insensitively over the whole address
- `user_id` against `user_id`, by numeric value

The later duplicate is reported and names the first declaration, so the error
is stable regardless of how many duplicates a file contains.

Comparison never spans identity kinds. A `username` invite, an `email` invite,
and a `user_id` invite are always independent, even when they refer to the same
GitHub account, because establishing that relationship would require a live
identity lookup that offline validation must not perform.

Case-insensitive email comparison is an intentional octostate desired-state
rule, not an assumption about how mail systems route messages. SMTP still
permits a case-sensitive local-part, so octostate defines equivalence for
itself: two invite emails that differ only by case are one desired-state
identity, handled deterministically and consistently with how every other
collection in this schema establishes identity.

An invite that does not declare exactly one identity is reported by the
identity rules above and takes no part in duplicate detection, so a malformed
entry never masks or manufactures a duplicate.

## Repositories

Each repository entry requires:

- `name`
- `visibility`

Repository owner handling:

- `owner` is optional
- when omitted, it defaults to the top-level `organization`
- managed repository owners stay within the declared top-level organization
- explicit owners are compared after trimming surrounding whitespace and
  case-folding, so `Acme`, ` acme `, and `ACME` all match the same
  organization
- an explicit cross-organization managed owner is rejected with
  `repository_owner_scope`
- this boundary applies only to managed repository ownership in
  `repositories[].owner`; external `template.owner` references are separate
  create-time inputs and may still point at another organization

Supported visibility values:

- `public`
- `private`
- `internal`

`internal` is available for GitHub Enterprise organizations/accounts that
support internal repositories. GitHub may reject it during live preflight or
apply for unsupported targets. Template-based creation does not support
`internal`; declare an ordinary repository creation or use a supported target.

Repository template fields are create-time inputs and also define managed
same-organization dependency edges for repositories that are missing from live
state:

- `template.owner`
- `template.name`
- `template.include_all_branches`

`template.owner` and `template.name` are required together when creating a
repository from a template. Omitting the template selects ordinary
organization-repository creation. A reference to another missing repository is a
managed dependency only when the source is also a top-level desired repository
in the configured organization. External, cross-organization, or live-only
references remain reference-only and are checked during apply preflight
instead.

`config apply` creates repositories ordinarily when no template is declared and
uses the template path when both template fields are present.

Repository reconciliation semantics:

- `visibility` and `topics` are exact-reconcile fields
- `description`, `homepage`, `allow_forking`, `archived`, and `is_template`
  are presence-aware optional fields
- omitted optional fields are left unmanaged
- explicit empty strings for `description` or `homepage` clear those fields
- explicit boolean values manage the boolean fields
- explicit `null` is rejected for presence-aware fields by semantic validation,
  except for `is_template`, where it means unmanaged/preserve-live-state
- `allow_forking` is managed only for private repositories when explicitly
  present; omitted private values remain unmanaged and public/internal values
  are ignored

Repository topics:

- omitted or empty `topics` means no topics
- leading and trailing whitespace is trimmed
- whitespace-only values fail validation
- valid topics contain only ASCII lowercase letters, numbers, and hyphens
- each topic is at most 50 characters
- each repository has at most 20 distinct non-empty normalized topics
- duplicate entries do not consume topic-limit capacity
- validation does not lowercase, reorder, or deduplicate config values
- invalid state is rejected before plan, apply, or proposal writes, and before
  `sync-from-live --write` writes generated config

These constraints follow the [GitHub repository topic constraints](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/classifying-your-repository-with-topics).

Examples:

```yaml
# Valid
organization: octostate
repositories:
  - name: octostate
    visibility: public
    topics: [go, gitops-tools, octostate]

# Rejected by validation as written; values are not rewritten.
organization: octostate
repositories:
  - name: octostate
    visibility: public
    topics: [Go, go_lang, go/lang, "go lang"]
```

## Teams

Each team entry requires:

- `slug`
- `name`

Common team fields:

- `description` - managed as a normal string
- `privacy` - should be one of the supported GitHub privacy values when the
  team is being managed
- `parent_slug` - optional parent team reference
- `members` - optional list of team memberships
- `repositories` - optional list of team repository permissions

When both are declared, `slug` must match the normalized form of `name` used
by validation.

Supported team member roles:

- `member`
- `maintainer`

Supported team privacy values:

- `closed`
- `secret`

Team members must also exist in top-level `members`.

## Team Repository Permissions

Team repository permission entries live under each team’s `repositories`
section.

Each entry requires:

- `name`
- `permission`

Repository owner handling:

- `owner` is optional
- when omitted, it defaults to the top-level `organization`
- managed team repository permission targets stay within the declared
  top-level organization
- explicit owners are compared after trimming surrounding whitespace and
  case-folding, matching the same normalization used for top-level
  repositories
- an explicit cross-organization managed owner is rejected with
  `repository_owner_scope`
- same-organization team targets may be declared here even when the same
  repository is omitted from the top-level `repositories:` collection; this
  grants team access without making the repository a top-level managed
  repository

Supported repository permissions:

- `pull`
- `triage`
- `push`
- `maintain`
- `admin`

## Ownership Boundary and Migration

Managed repository ownership in this schema is organization-local:

- `repositories[].owner` and `teams[].repositories[].owner` default to the
  top-level `organization` when omitted
- omitted owners and explicit same-organization owners are equivalent after
  trimming and case-folding
- explicit cross-organization managed owners are invalid and are reported as
  `repository_owner_scope`
- when the top-level `organization` is missing or blank, owner-scope
  diagnostics are suppressed and the missing-organization rule remains
  authoritative
- `template.owner` is not part of this managed-owner boundary and does not need
  to match the top-level `organization`

If you have existing config that relied on previously accepted explicit
cross-organization managed owners:

1. Run `octostate config validate --config-dir ./config` to surface each
   `repository_owner_scope` violation before plan, apply, diff, or
   `sync-from-live --write` continues.
2. For managed repositories or team repository permission targets that belong
   to the same organization, either remove the redundant `owner` field or
   normalize it to the top-level organization.
3. For intentionally cross-organization managed targets, remove them from
   managed desired state or move them to a desired config whose top-level
   `organization` matches the managed owner.
4. For external template sources, keep `template.owner` as written; no
   migration is required unless the template itself changed.

## Reconciliation Notes

- presence-aware repository fields only reconcile when they are explicitly
  declared in desired config
- repository template fields are create-time only
- for dependency resolution, an existing source uses an explicitly managed
  `is_template` value or its live value when `is_template` is omitted/null; a
  new source is usable only when `is_template: true`
- managed repository actions use deterministic dependency-safe topological
  ordering; unavailable sources propagate diagnostics transitively and cycles
  report a stable `template dependency cycle: ...` path
- team repository permissions reuse repository availability; external,
  cross-organization, live-only, and otherwise non-managed template references
  remain apply-preflight concerns
- the public plan JSON contains no dependency field
- `config apply --check` uses the same dependency-safe order as `config apply`,
  continues through remaining executable actions, and aggregates preflight
  failures deterministically; resource-specific dependency handling may defer
  some checks, so failures are not universally reported in plan order
