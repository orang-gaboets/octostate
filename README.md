# repo-builder

`repo-builder` is a command-line tool for creating new GitHub repositories from a template repository. It is intended for bootstrapping projects within an organisation.

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

## Testing
To run tests, use the following command:

```bash
go test ./... -cover -coverprofile=coverage.out -tags=unit
```
## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.