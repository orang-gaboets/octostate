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

## Repositories

Each repository entry requires:

- `name`
- `visibility`

Repository owner handling:

- `owner` is optional
- when omitted, it defaults to the top-level `organization`

Supported visibility values:

- `public`
- `private`

`internal` visibility is currently rejected by validation and is not supported
by apply yet.

Repository template fields are create-time inputs:

- `template.owner`
- `template.name`
- `template.include_all_branches`

`template.owner` and `template.name` are required together when creating a
repository from a template.

Repository reconciliation semantics:

- `visibility` and `topics` are exact-reconcile fields
- `description`, `homepage`, `allow_forking`, `archived`, and `is_template`
  are presence-aware optional fields
- omitted optional fields are left unmanaged
- explicit empty strings for `description` or `homepage` clear those fields
- explicit boolean values manage the boolean fields
- explicit `null` is rejected for the presence-aware optional fields
- `allow_forking` is ignored for private repositories

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

Supported repository permissions:

- `pull`
- `triage`
- `push`
- `maintain`
- `admin`

## Reconciliation Notes

- presence-aware repository fields only reconcile when they are explicitly
  declared in desired config
- repository template fields are create-time only
- team repository permissions are managed per team, but the underlying
  repository still needs to exist or be created earlier in the same plan
- `config apply --check` uses the same normalized plan ordering as `config
  apply`, so same-plan repository template updates are visible to later same
  plan creates
