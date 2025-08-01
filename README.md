# repo-builder

`repo-builder` is a command-line tool for creating new GitHub repositories from a template repository. It is intended for bootstrapping projects within an organisation.

## Installation

```
go install github.com/yourorg/repo-builder/cmd/repo-builder@latest
```

## Usage

```
repo-builder --token <token> --org <org> --template <template> --name <repo-name> [--desc <description>] [--topics <t1,t2>] [--private true|false]
```

### Flags

- `--token` (required): GitHub personal access token
- `--org` (required): GitHub organisation name
- `--template` (required): Template repository name
- `--name` (required): New repository name
- `--desc` (optional): Repository description
- `--topics` (optional): Comma-separated list of repository topics
- `--private` (optional): Create a private repository (default is public)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.