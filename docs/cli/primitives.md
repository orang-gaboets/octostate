# Primitive Commands

Live invocations of these commands operate directly on GitHub resources
instead of going through the desired-state file (`config/organization.yaml` in
the common layout).

Live primitive operations require GitHub authentication. Supply exactly one of:
- `--token`
- `--app-id`, `--installation-id`, and `--app-key-path`

The command examples in this page intentionally show GitHub App auth directly so
that the non-token path stays visible in the reference.

This reference uses canonical command names. Common aliases such as `org`,
`repo create`, `team delete`, and `team get` also work.

Mutating commands support `--dry-run` where noted. Destructive delete commands
require explicit confirmation with `--yes` unless `--dry-run` is used.

## Proposal mode

The repository, topic, team, team membership, and team repository permission
mutation commands documented below also support
`--to-config <organization.yaml>`. Proposal mode updates the existing local
configuration instead of calling GitHub, so GitHub authentication is not
required for these operations.

Proposal mode requires an existing regular file (not a directory or symbolic
link) whose `organization:` value matches `--org` case-insensitively. The file
is validated before and after the requested mutation and is replaced
atomically. Semantic no-ops return `changed: false` and leave the file bytes
unchanged. `--to-config` and `--dry-run` are mutually exclusive.

Team proposals follow the desired-state schema rules: `team create` derives
the team slug from the normalized team name and rejects a slug that collides
with an existing team; `team edit` rejects name changes that would change the
team slug; and any supplied parent team must already exist in desired state.

Team membership proposals require the target team to exist in desired state.
`team members add` also requires the username to already be declared in
top-level `members:`, and updates the role in place when the member is already
on the team. `team members remove` drops only the targeted membership and
leaves the top-level `members:` entry in place; removing a member who is not on
the team is a no-op.

Team repository permission proposals also require the target team to exist in
desired state. `team repo permissions add` updates the permission in place when
the team already has an entry for the repository, and `remove` drops only the
targeted entry; a repository the team has no entry for is a no-op. `--repo-org`
is stored only when it differs from the organization.

The repository itself does not need to be declared under top-level
`repositories:` — `config validate` does not cross-check permission entries
against declared repositories, so this is valid proposal state. Be aware of what
that means downstream: `config plan` marks a team repository permission
executable only once the repository exists in live state or is created earlier
in the same plan, and `config apply` skips actions that are not executable. A
permission proposed against a repository that is neither declared nor live is
therefore valid, reviewable config that stays pending until the repository
exists.

## Organization

### `octostate organization get-by-name`

```bash
octostate organization get-by-name --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name

### `octostate organization list-repos`

```bash
octostate organization list-repos --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> [--type <all|public|private|forks|sources|member>]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--type` (optional): Type of repositories to list (default is `all`)

### `octostate organization list-members`

```bash
octostate organization list-members --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> [--role <all|admin|member>]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--role` (optional): Role of members to list (default is `all`)

### `octostate organization list-teams`

```bash
octostate organization list-teams --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name

### `octostate organization list-invitations`

```bash
octostate organization list-invitations --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name

### `octostate organization invite`

```bash
octostate organization invite --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> (--id <user-id> | --username <username>) [--dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--id` (required unless `--username` is provided): GitHub user ID to invite
- `--username` (required unless `--id` is provided): GitHub username to invite
- `--dry-run` (optional): Preview the invitation request without creating it (username lookups are skipped in dry-run mode)

## Repository

### `octostate repo get`

```bash
octostate repo get --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name

### `octostate repo create-from-template`

```bash
octostate repo create-from-template --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> [--template-org <template-org>] --template-name <template-name> --org <org> --name <repo-name> [--desc <description>] [--topics <t1,t2>] [--private true|false] [--include-all-branches true|false] [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--template-org` (optional): Organization that owns the template repository (defaults to `--org`)
- `--template-name` (required): Name of the template repository
- `--name` (required): New repository name
- `--desc` (optional): Repository description
- `--topics` (optional): Comma-separated list of repository topics
- `--private` (optional): Create a private repository (default is public)
- `--include-all-branches` (optional): Include all branches from the template repository (default is false)
- `--to-config` (optional): Add the repository proposal to an existing local organization config
- `--dry-run` (optional): Preview repository creation without creating it

### `octostate repo delete`

```bash
octostate repo delete --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> (--yes | --dry-run)
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name
- `--yes` (required unless `--dry-run` is set): Confirm the destructive delete operation
- `--dry-run` (optional): Preview repository deletion without deleting it

### `octostate repo edit`

```bash
octostate repo edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> [--desc <description>] [--homepage <homepage-url>] [--private true|false] [--is-template true|false] [--archived true|false] [--allow-forking true|false] [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name
- `--desc` (optional): Repository description
- `--homepage` (optional): Repository homepage URL
- `--private` (optional): Set repository to private/public
- `--is-template` (optional): Set or unset repository as a template
- `--archived` (optional): Archive/unarchive the repository
- `--allow-forking` (optional): Allow/disallow private forking of the repository
- `--to-config` (optional): Apply the repository proposal to an existing local organization config
- `--dry-run` (optional): Preview repository edits without updating the repository

## Topic

### `octostate topic add`

```bash
octostate topic add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --topics <t1,t2> [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name
- `--topics` (required): Comma-separated list of topics to add
- `--to-config` (optional): Apply the topic proposal to an existing local organization config
- `--dry-run` (optional): Preview topic additions without updating the repository

### `octostate topic list`

```bash
octostate topic list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name

### `octostate topic replace`

```bash
octostate topic replace --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --topics <t1,t2> [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name
- `--topics` (required): Comma-separated list of topics to set
- `--to-config` (optional): Apply the topic replacement to an existing local organization config
- `--dry-run` (optional): Preview topic replacement without updating the repository

## Team

### `octostate team create`

```bash
octostate team create --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <team-name> [--desc <description>] [--secret true|false] [--parent <parent-team-slug>] [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Team name
- `--desc` (optional): Team description
- `--secret` (optional): Create a secret team (default is false)
- `--parent` (optional): Parent team slug (if creating a child team); the parent must already exist in desired state when using `--to-config`
- `--to-config` (optional): Add the team proposal to an existing local organization config instead of creating the team in GitHub; the slug is derived from the normalized team name and must not collide with an existing team
- `--dry-run` (optional): Preview team creation without creating the team

### `octostate team edit`

```bash
octostate team edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> [--name <new-team-name>] [--desc <description>] [--secret true|false] [--parent <parent-team-slug> | --clear-parent] [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--name` (optional): New team name
- `--desc` (optional): New team description (pass an empty string to clear)
- `--secret` (optional): Set privacy to secret (`true`) or closed (`false`) when provided
- `--parent` (optional): Parent team slug to assign
- `--clear-parent` (optional): Remove the parent team relationship
- `--to-config` (optional): Apply the team edit proposal to an existing local organization config instead of GitHub; name edits that would change the normalized team slug are rejected, and a supplied parent must already exist in desired state
- `--dry-run` (optional): Preview team edits without updating the team

### `octostate team delete-by-slug`

```bash
octostate team delete-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> (--yes | --dry-run)
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--yes` (required unless `--dry-run` is set): Confirm the destructive delete operation
- `--dry-run` (optional): Preview team deletion without deleting the team

### `octostate team get-by-slug`

```bash
octostate team get-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)

### `octostate team members list`

```bash
octostate team members list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> [--role <all|member|maintainer>]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--role` (optional): Team member role filter (`all`, `member`, or `maintainer`; default is `all`)

### `octostate team members add`

```bash
octostate team members add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --username <username> [--role <member|maintainer>] [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--username` (required): GitHub username to add/update in the team
- `--role` (optional): Team membership role (`member` or `maintainer`; default is `member`)
- `--to-config` (optional): Apply the membership proposal to an existing local organization config instead of GitHub; the team must exist in desired state and the username must already be declared in top-level `members:`
- `--dry-run` (optional): Preview the membership change without calling GitHub

### `octostate team members remove`

```bash
octostate team members remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --username <username> [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--username` (required): GitHub username to remove from the team
- `--to-config` (optional): Remove the membership from an existing local organization config instead of GitHub; the top-level `members:` entry is preserved
- `--dry-run` (optional): Preview the membership removal without calling GitHub

### `octostate team repo permissions list`

```bash
octostate team repo permissions list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)

### `octostate team repo permissions add`

```bash
octostate team repo permissions add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --repo <repo-name> [--repo-org <repo-org>] [--permission <pull|push|admin|maintain|triage>] [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--repo` (required): Repository name
- `--repo-org` (optional): Owner organization of the repository (defaults to `--org`)
- `--permission` (optional): Permission to grant (`pull`, `push`, `admin`, `maintain`, `triage`; default is `pull`)
- `--to-config` (optional): Apply the permission proposal to an existing local organization config instead of GitHub; the team must exist in desired state and an existing entry for the repository is updated in place
- `--dry-run` (optional): Preview the permission change without calling GitHub

### `octostate team repo permissions remove`

```bash
octostate team repo permissions remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --repo <repo-name> [--repo-org <repo-org>] [--to-config <organization.yaml> | --dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--repo` (required): Repository name
- `--repo-org` (optional): Owner organization of the repository (defaults to `--org`)
- `--to-config` (optional): Remove the permission entry from an existing local organization config instead of GitHub; an entry that does not exist is a no-op
- `--dry-run` (optional): Preview the permission removal without calling GitHub

## User

### `octostate user get-by-id`

```bash
octostate user get-by-id --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --id <user-id>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--id` (required): User ID

### `octostate user get-by-username`

```bash
octostate user get-by-username --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --username <username>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--username` (required): GitHub username
