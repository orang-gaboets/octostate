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
    go build -o repo-builder ./cmd/repo-builder
    ```



## Usage

### Create a New Repository Based on a Template

Option 1 (installed packaged):
```bash
repo-builder repo create-from-template --token <token> --org <org> --template-org <template-org> --template-name <template-name> --name <repo-name> [--desc <description>] [--topics <t1,t2>] [--private true|false] [--include-all-branches true|false]
```

Option 2 (using build):
```bash
./repo-builder repo create-from-template --token <token> --org <org> --template-org <template-org> --template-name <template-name> --name <repo-name> [--desc <description>] [--topics <t1,t2>] [--private true|false] [--include-all-branches true|false]
```

Option 3 (using `go run`):
```bash
go run ./cmd/repo-builder repo create-from-template --token <token> --org <org> --template-org <template-org> --template-name <template-name> --name <repo-name> [--desc <description>] [--topics <t1,t2>] [--private true|false] [--include-all-branches true|false]
```

#### Flags

- `--token` (required): GitHub personal access token
- `--org` (required): GitHub organisation name
- `--template-org` (required): Organisation that owns the template repository
- `--template-name` (required): Name of the template repository
- `--name` (required): New repository name
- `--desc` (optional): Repository description
- `--topics` (optional): Comma-separated list of repository topics
- `--private` (optional): Create a private repository (default is public)
- `--include-all-branches` (optional): Include all branches from the template repository (default is false)

### Edit Repository Settings

Option 1 (installed packaged):
```bash
repo-builder repo edit --token <token> --org <org> --name <repo-name> [--desc <description>] [--homepage <homepage>] [--private true|false] [--is-template true|false] [--archived true|false] [--allow-forking true|false]
```

Option 2 (using build):
```bash
./repo-builder repo edit --token <token> --org <org> --name <repo-name> [--desc <description>] [--homepage <homepage>] [--private true|false] [--is-template true|false] [--archived true|false] [--allow-forking true|false]
```

Option 3 (using `go run`):
```bash
go run ./cmd/repo-builder repo edit --token <token> --org <org> --name <repo-name> [--desc <description>] [--homepage <homepage>] [--private true|false] [--is-template true|false] [--archived true|false] [--allow-forking true|false]
```

#### Flags
- `--token` (required): GitHub personal access token
- `--org` (required): GitHub organisation name
- `--name` (required): Repository name
- `--desc` (optional): Repository description
- `--homepage` (optional): Repository homepage URL
- `--private` (optional): Set repository to private/public
- `--is-template` (optional): Set or unset repository as a template
- `--archived` (optional): Archive/unarchive the repository
- `--allow-forking` (optional): Allow/disallow private forking of the repository



## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.