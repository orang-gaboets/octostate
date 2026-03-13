# Changelog

All notable changes to this project will be documented in this file.

The release process is managed by `release-please`, which updates this changelog
when release pull requests are created and merged.

## [0.5.0](https://github.com/orang-gaboets/repo-builder/compare/v0.4.1...v0.5.0) (2026-03-13)


### Features

* **gitops:** add reconciliation apply executor and config apply ([#62](https://github.com/orang-gaboets/repo-builder/issues/62)) ([fefdce0](https://github.com/orang-gaboets/repo-builder/commit/fefdce0ac01523d9f19890266f014d945a2089d3))

## [0.4.1](https://github.com/orang-gaboets/repo-builder/compare/v0.4.0...v0.4.1) (2026-03-13)


### Bug Fixes

* **ci:** serialize all `release-please` workflow runs ([#57](https://github.com/orang-gaboets/repo-builder/issues/57)) ([0b0cd35](https://github.com/orang-gaboets/repo-builder/commit/0b0cd3552cf2a47b13092911b80d6ebd46c5d572))

## [0.4.0](https://github.com/orang-gaboets/repo-builder/compare/v0.3.0...v0.4.0) (2026-03-13)


### Features

* **config:** add GitOps reconciliation planning command ([#54](https://github.com/orang-gaboets/repo-builder/issues/54)) ([bbdb431](https://github.com/orang-gaboets/repo-builder/commit/bbdb431ea331f305e76bb6a4ddc26b877b64a19d))

## [0.3.0](https://github.com/orang-gaboets/repo-builder/compare/v0.2.0...v0.3.0) (2026-03-10)


### Features

* **audit:** add actual-state snapshot pull command ([#51](https://github.com/orang-gaboets/repo-builder/issues/51)) ([6f60366](https://github.com/orang-gaboets/repo-builder/commit/6f60366570c5c21cb5da01f893fe95dff94f74f6))

## [0.2.0](https://github.com/orang-gaboets/repo-builder/compare/v0.1.0...v0.2.0) (2026-03-06)


### Features

* add offline GitOps config validate command and single-file organization.yaml schema ([#48](https://github.com/orang-gaboets/repo-builder/issues/48)) ([e76484a](https://github.com/orang-gaboets/repo-builder/commit/e76484ac8d3ff26c279bd82932270e55698721da))

## 0.1.0 (2026-03-03)

Initial release of `repo-builder`.

### Highlights

- GitHub organization operations CLI for repositories, teams, topics,
  organizations, and users
- Script-friendly JSON output for read, list, and get commands
- Structured operation results for mutating commands
- Safety controls including `--dry-run` previews and explicit `--yes`
  confirmation for destructive deletes
- Support for both personal access token and GitHub App authentication
- CI, linting, and release automation setup

### Included Commands

- `organization`: `get-by-name`, `list-repos`, `list-members`, `list-teams`,
  `invite`
- `repo`: `create-from-template`, `get`, `edit`, `delete`
- `team`: `create`, `get-by-slug`, `edit`, `delete-by-slug`, `members list`,
  `members add`, `members remove`, `repo permissions list`, `repo permissions add`,
  `repo permissions remove`
- `topic`: `list`, `add`, `replace`
- `user`: `get-by-id`, `get-by-username`

### Notes

This release establishes the API-primitive CLI baseline. Configuration-driven
GitOps commands such as `config validate`, `plan`, and `apply` are planned for
follow-up releases.
