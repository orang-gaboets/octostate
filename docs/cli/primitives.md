# Primitive Commands

These commands operate directly on GitHub resources instead of going through
`organization.yaml`.

All primitive commands require GitHub authentication. Supply exactly one of:
- `--token`
- `--app-id`, `--installation-id`, and `--app-key-path`

The command examples in this page intentionally show GitHub App auth directly so
that the non-token path stays visible in the reference.

This reference uses canonical command names. Common aliases such as `org`,
`repo create`, `team delete`, and `team get` also work.

Mutating commands support `--dry-run` where noted. Destructive delete commands
require explicit confirmation with `--yes` unless `--dry-run` is used.

## Organization

### `repo-builder organization get-by-name`

```bash
repo-builder organization get-by-name --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name

### `repo-builder organization list-repos`

```bash
repo-builder organization list-repos --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> [--type <all|public|private|forks|sources|member>]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--type` (optional): Type of repositories to list (default is `all`)

### `repo-builder organization list-members`

```bash
repo-builder organization list-members --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> [--role <all|admin|member>]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--role` (optional): Role of members to list (default is `all`)

### `repo-builder organization list-teams`

```bash
repo-builder organization list-teams --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name

### `repo-builder organization list-invitations`

```bash
repo-builder organization list-invitations --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name

### `repo-builder organization invite`

```bash
repo-builder organization invite --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> (--id <user-id> | --username <username>) [--dry-run]
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

### `repo-builder repo get`

```bash
repo-builder repo get --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name

### `repo-builder repo create-from-template`

```bash
repo-builder repo create-from-template --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> [--template-org <template-org>] --template-name <template-name> --org <org> --name <repo-name> [--desc <description>] [--topics <t1,t2>] [--private true|false] [--include-all-branches true|false] [--dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--template-org` (optional): Organisation that owns the template repository (defaults to `--org`)
- `--template-name` (required): Name of the template repository
- `--name` (required): New repository name
- `--desc` (optional): Repository description
- `--topics` (optional): Comma-separated list of repository topics
- `--private` (optional): Create a private repository (default is public)
- `--include-all-branches` (optional): Include all branches from the template repository (default is false)
- `--dry-run` (optional): Preview repository creation without creating it

### `repo-builder repo delete`

```bash
repo-builder repo delete --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> (--yes | --dry-run)
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

### `repo-builder repo edit`

```bash
repo-builder repo edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> [--desc <description>] [--homepage <homepage-url>] [--private true|false] [--is-template true|false] [--archived true|false] [--allow-forking true|false] [--dry-run]
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
- `--dry-run` (optional): Preview repository edits without updating the repository

## Topic

### `repo-builder topic add`

```bash
repo-builder topic add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --topics <t1,t2> [--dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name
- `--topics` (required): Comma-separated list of topics to add
- `--dry-run` (optional): Preview topic additions without updating the repository

### `repo-builder topic list`

```bash
repo-builder topic list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name

### `repo-builder topic replace`

```bash
repo-builder topic replace --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --topics <t1,t2> [--dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--name` (required): Repository name
- `--topics` (required): Comma-separated list of topics to set
- `--dry-run` (optional): Preview topic replacement without updating the repository

## Team

### `repo-builder team create`

```bash
repo-builder team create --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <team-name> [--desc <description>] [--secret true|false] [--parent <parent-team-slug>] [--dry-run]
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
- `--parent` (optional): Parent team slug (if creating a child team)
- `--dry-run` (optional): Preview team creation without creating the team

### `repo-builder team edit`

```bash
repo-builder team edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> [--name <new-team-name>] [--desc <description>] [--secret true|false] [--parent <parent-team-slug> | --clear-parent] [--dry-run]
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
- `--secret` (optional): Set privacy to secret (`true`) or visible (`false`) when provided
- `--parent` (optional): Parent team slug to assign
- `--clear-parent` (optional): Remove the parent team relationship
- `--dry-run` (optional): Preview team edits without updating the team

### `repo-builder team delete-by-slug`

```bash
repo-builder team delete-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> (--yes | --dry-run)
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

### `repo-builder team get-by-slug`

```bash
repo-builder team get-by-slug --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)

### `repo-builder team members list`

```bash
repo-builder team members list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> [--role <all|member|maintainer>]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--role` (optional): Team member role filter (`all`, `member`, or `maintainer`; default is `all`)

### `repo-builder team members add`

```bash
repo-builder team members add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --username <username> [--role <member|maintainer>] [--dry-run]
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
- `--dry-run` (optional): Preview the membership change without calling GitHub

### `repo-builder team members remove`

```bash
repo-builder team members remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --username <username> [--dry-run]
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)
- `--username` (required): GitHub username to remove from the team
- `--dry-run` (optional): Preview the membership removal without calling GitHub

### `repo-builder team repo permissions list`

```bash
repo-builder team repo permissions list --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organization name
- `--slug` (required): Team slug (URL-friendly name)

### `repo-builder team repo permissions add`

```bash
repo-builder team repo permissions add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --repo <repo-name> [--repo-org <repo-org>] [--permission <pull|push|admin|maintain|triage>] [--dry-run]
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
- `--dry-run` (optional): Preview the permission change without calling GitHub

### `repo-builder team repo permissions remove`

```bash
repo-builder team repo permissions remove --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-slug> --repo <repo-name> [--repo-org <repo-org>] [--dry-run]
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
- `--dry-run` (optional): Preview the permission removal without calling GitHub

## User

### `repo-builder user get-by-id`

```bash
repo-builder user get-by-id --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --id <user-id>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--id` (required): User ID

### `repo-builder user get-by-username`

```bash
repo-builder user get-by-username --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --username <username>
```

Flags:
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--username` (required): GitHub username
