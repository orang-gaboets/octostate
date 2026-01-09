# repo-builder

`repo-builder` is a GitHub organization operations CLI. It provides commands to manage and query common GitHub resources such as:

- repositories (create from template, edit, delete)
- topics (add, replace, list)
- teams (create, get, delete)
- organization utilities (list repos/members/teams, get org by name)
- users (lookup by ID / username)

This repository is the **engine** (CLI + reusable automation building blocks).  
**Planned next:** a separate **control repo** will hold configuration + approvals (GitOps), and call into this engine via GitHub Actions.

## Status / Roadmap

- **v0 (current):** API-primitive commands (direct GitHub operations) ✅
- **v1 (planned):** GitOps commands + audit:
  - `repo-builder config validate`
  - `repo-builder plan`
  - `repo-builder apply`
  - `repo-builder audit pull`
  - `repo-builder audit diff`

## Installation

```
go install github.com/yourorg/repo-builder/cmd/repo-builder@latest
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
    
    Ensure that your Go version is at least 1.18 or higher.
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

### Authentication

All commands require GitHub authentication. You must supply exactly one of the following:

- `--token` – personal access token (PAT).
- `--app-id`, `--installation-id`, and `--app-key-path` – GitHub App ID, installation ID, and path to the App's private key.

Providing both methods or neither results in an error.

### Organizations

#### Get Organization Details by Name

```bash
repo-builder org get-by-name --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name

#### List Organization Repositories

```bash
repo-builder org list-repos --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> [--type <all|public|private|forks|sources|member>]
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
repo-builder org list-members --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> [--role <all|admin|member>]
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
repo-builder org list-teams --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name

### Repo

#### Create a New Repository Based on a Template

```bash
repo-builder repo create --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --template-org <template-org> --template-name <template-name> --org <org> --name <repo-name> [--desc <description>] [--topics <t1,t2>] [--private true|false] [--include-all-branches true|false]
```

##### Flags

- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--template-org` (required): Organisation that owns the template repository
- `--template-name` (required): Name of the template repository
- `--name` (required): New repository name
- `--desc` (optional): Repository description
- `--topics` (optional): Comma-separated list of repository topics
- `--private` (optional): Create a private repository (default is public)
- `--include-all-branches` (optional): Include all branches from the template repository (default is false)

#### Delete a Repository

```bash
repo-builder repo delete --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name

#### Edit Repository Settings

```bash
repo-builder repo edit --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> [--desc <description>] [--homepage <homepage-url>] [--private true|false] [--is-template true|false] [--archived true|false] [--allow-forking true|false]
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

### Topic

#### Add Topics to a Repository

```bash
repo-builder topic add --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --topics <t1,t2>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name
- `--topics` (required): Comma-separated list of topics to add

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
repo-builder topic replace --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <repo-name> --topics <t1,t2>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name
- `--topics` (required): Comma-separated list of topics to set

### Team

#### Create a New Team

```bash
repo-builder team create --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --name <team-name> [--desc <description>] [--secret true|false] [--parent <parent-team-slug>]
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

#### Delete a Team

```bash
repo-builder team delete --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-name>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)

#### Get Team by Slug

```bash
repo-builder team get --app-id <app-id> --installation-id <installation-id> --app-key-path <path-to-app-key> --org <org> --slug <team-name>
```

##### Flags
- `--token`: GitHub personal access token (required if using PAT authentication)
- `--app-id`: GitHub App ID (required if using GitHub App authentication)
- `--installation-id`: GitHub App installation ID (required if using GitHub App authentication)
- `--app-key-path`: Path to the GitHub App's private key file (required if using GitHub App authentication)
- `--org` (required): GitHub organisation name
- `--slug` (required): Team slug (URL-friendly name)

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
To run tests, use the following command:

```bash
go test ./... -cover -coverprofile=coverage.out -tags=unit
```
## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## GitOps (Planned)

This repository is the **engine** (CLI + reusable automation building blocks).  
The GitOps workflow will live in a separate **control repository** per organization (e.g. `<org>-control`), which will:

- store the **desired state** in `config/`
- require **PR review/approval** for changes
- apply changes to GitHub using this engine (via GitHub Actions + GitHub App auth)
- continuously snapshot reality into `state/` and report drift

### Control repo layout (proposed)

```text
config/
  repos.yaml        # desired repositories (template, settings, topics, etc.)
  teams.yaml        # desired teams, members, repo permissions
  policies.yaml     # org-wide defaults / rules (optional)
state/
  actual/           # generated snapshots from GitHub (never hand-edit)
  diff/             # drift reports (optional)
  events/           # append-only change log (optional)
```

### Planned GitOps commands (engine)

These commands will be implemented in this repo and invoked from the control repo workflows:
- `repo-builder config validate` — validate config schema + invariants
- `repo-builder plan` — show what changes would be made
- `repo-builder apply` — reconcile GitHub to match config/ (idempotent)
- `repo-builder audit pull` — snapshot GitHub into state/actual/
- `repo-builder audit diff` — detect drift / policy violations (optionally fail CI)
