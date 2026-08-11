# Changelog

All notable changes to this project will be documented in this file.

The release process is managed by `release-please`, which updates this changelog
when release pull requests are created and merged.

## [1.1.1](https://github.com/orang-gaboets/octostate/compare/v1.1.0...v1.1.1) (2026-07-29)


### Bug Fixes

* propagate PAT GitHub client construction errors ([#168](https://github.com/orang-gaboets/octostate/issues/168)) ([5f1050c](https://github.com/orang-gaboets/octostate/commit/5f1050c06a7a1ea6e8a44bee50c4180180ac8743))

## [1.1.0](https://github.com/orang-gaboets/octostate/compare/v1.0.2...v1.1.0) (2026-06-24)


### Features

* **config:** add apply preflight check mode ([#131](https://github.com/orang-gaboets/octostate/issues/131)) ([c07c97c](https://github.com/orang-gaboets/octostate/commit/c07c97c5245d6add6c0646cbb271fb175f55fe0e))
* **config:** add read-only apply preflight probes ([#134](https://github.com/orang-gaboets/octostate/issues/134)) ([352aba9](https://github.com/orang-gaboets/octostate/commit/352aba9b7de524fd232bd5961ed853e4ae591597))


### Bug Fixes

* **config:** classify config runtime failures by phase ([#144](https://github.com/orang-gaboets/octostate/issues/144)) ([1092193](https://github.com/orang-gaboets/octostate/commit/10921934e813c4a71ba3b45bf96b49a6cfdc817f))
* **gitops:** preserve template state across same-plan applies ([#143](https://github.com/orang-gaboets/octostate/issues/143)) ([7f3d8b1](https://github.com/orang-gaboets/octostate/commit/7f3d8b19ba1568170ad0515d0ef6bcaca2da10c2))
* **plan:** gate team repo permissions on repository availability ([#142](https://github.com/orang-gaboets/octostate/issues/142)) ([67ef3b1](https://github.com/orang-gaboets/octostate/commit/67ef3b19ae9e7c8fbd1545d71104e852255a145f))

## [1.0.2](https://github.com/orang-gaboets/octostate/compare/v1.0.1...v1.0.2) (2026-06-07)


### Bug Fixes

* **ci:** gate release-please auto-merge on publisher approval ([#125](https://github.com/orang-gaboets/octostate/issues/125)) ([3e42cec](https://github.com/orang-gaboets/octostate/commit/3e42cecd22f2e9dfe81d75318dbb06111ca8dccd))

## [1.0.1](https://github.com/orang-gaboets/octostate/compare/v1.0.0...v1.0.1) (2026-05-31)


### Bug Fixes

* **ci:** merge release-please PRs after checks pass ([#120](https://github.com/orang-gaboets/octostate/issues/120)) ([af4a054](https://github.com/orang-gaboets/octostate/commit/af4a054b5e9a60cc61e9f212d9166795d58ee104))
* **deps:** resolve Go security alerts ([#117](https://github.com/orang-gaboets/octostate/issues/117)) ([f26022a](https://github.com/orang-gaboets/octostate/commit/f26022a772536a827830296c41f20d49f3f0fdfe))

## [1.0.0](https://github.com/orang-gaboets/octostate/compare/v0.17.0...v1.0.0) (2026-05-22)


### ⚠ BREAKING CHANGES

* validate v1.0.0 readiness ([#108](https://github.com/orang-gaboets/octostate/issues/108))

### Features

* validate v1.0.0 readiness ([#108](https://github.com/orang-gaboets/octostate/issues/108)) ([71242ad](https://github.com/orang-gaboets/octostate/commit/71242adf3e8a8884571d3896f6555748a8df0da7))

## [0.17.0](https://github.com/orang-gaboets/octostate/compare/v0.16.0...v0.17.0) (2026-04-02)


### Features

* **apply:** pre-resolve invite usernames before sequential execution ([#101](https://github.com/orang-gaboets/octostate/issues/101)) ([79d1ec1](https://github.com/orang-gaboets/octostate/commit/79d1ec112b97bf3d4af40b4787aa3f2745cb84e5))

## [0.16.0](https://github.com/orang-gaboets/octostate/compare/v0.15.0...v0.16.0) (2026-04-02)


### Features

* **diff:** parallelize offline audit diff build phases ([#99](https://github.com/orang-gaboets/octostate/issues/99)) ([78a799c](https://github.com/orang-gaboets/octostate/commit/78a799cf60c736f032e870b37be0010e2beafdd2))

## [0.15.0](https://github.com/orang-gaboets/octostate/compare/v0.14.0...v0.15.0) (2026-04-01)


### Features

* **audit:** parallelize invite username resolution ([#97](https://github.com/orang-gaboets/octostate/issues/97)) ([f0f8536](https://github.com/orang-gaboets/octostate/commit/f0f85367a47fb7b9076cbdff0bd9f68df729d8bc))

## [0.14.0](https://github.com/orang-gaboets/octostate/compare/v0.13.0...v0.14.0) (2026-04-01)


### Features

* **gitops:** add bounded concurrency to collectors and plan phases ([#92](https://github.com/orang-gaboets/octostate/issues/92)) ([5c08dbb](https://github.com/orang-gaboets/octostate/commit/5c08dbb10a0b728b63cb22b7e6b6874668939daf))

## [0.13.0](https://github.com/orang-gaboets/octostate/compare/v0.12.0...v0.13.0) (2026-04-01)


### Features

* **config:** split config plan output into executable and skipped actions ([#89](https://github.com/orang-gaboets/octostate/issues/89)) ([ad1e1aa](https://github.com/orang-gaboets/octostate/commit/ad1e1aaea4461ac39cc478c95353f63c33385e0f))

## [0.12.0](https://github.com/orang-gaboets/octostate/compare/v0.11.0...v0.12.0) (2026-03-31)


### Features

* **organization:** add pending invitation list command ([#87](https://github.com/orang-gaboets/octostate/issues/87)) ([4bcba62](https://github.com/orang-gaboets/octostate/commit/4bcba62d89be7315ef330de171312fae8ebedc05))

## [0.11.0](https://github.com/orang-gaboets/octostate/compare/v0.10.0...v0.11.0) (2026-03-30)


### Features

* **config:** add sync-from-live materialize mode ([#84](https://github.com/orang-gaboets/octostate/issues/84)) ([b28aad2](https://github.com/orang-gaboets/octostate/commit/b28aad27b78b1b8a699ad31788c57de0f2f91dc0))

## [0.10.0](https://github.com/orang-gaboets/octostate/compare/v0.9.0...v0.10.0) (2026-03-29)


### Features

* **config:** add sync-from-live adopt mode ([#82](https://github.com/orang-gaboets/octostate/issues/82)) ([db70738](https://github.com/orang-gaboets/octostate/commit/db70738f81dd3d78d6b8b93f0818fb4c47b8b585))

## [0.9.0](https://github.com/orang-gaboets/octostate/compare/v0.8.0...v0.9.0) (2026-03-24)


### Features

* **gitops:** add first-class organization membership ([#80](https://github.com/orang-gaboets/octostate/issues/80)) ([ed97c63](https://github.com/orang-gaboets/octostate/commit/ed97c6374327b4e47317fe589a5447c1b38f2bcd))

## [0.8.0](https://github.com/orang-gaboets/octostate/compare/v0.7.0...v0.8.0) (2026-03-23)


### Features

* **config:** add sync-from-live bootstrap mode ([#74](https://github.com/orang-gaboets/octostate/issues/74)) ([7c5a266](https://github.com/orang-gaboets/octostate/commit/7c5a266f6a9ee1860dc453721d7815d53eb9d179))

## [0.7.0](https://github.com/orang-gaboets/octostate/compare/v0.6.0...v0.7.0) (2026-03-15)


### Features

* **gitops:** make repository reconciliation fields presence-aware ([#67](https://github.com/orang-gaboets/octostate/issues/67)) ([91002b3](https://github.com/orang-gaboets/octostate/commit/91002b3ccfa3a1d42a13330f8c5af659f9e48b31))

## [0.6.0](https://github.com/orang-gaboets/octostate/compare/v0.5.0...v0.6.0) (2026-03-14)


### Features

* **audit:** add offline GitOps drift diff command ([#64](https://github.com/orang-gaboets/octostate/issues/64)) ([24518dd](https://github.com/orang-gaboets/octostate/commit/24518dd328317b47b441a79543af68176c673f48))

## [0.5.0](https://github.com/orang-gaboets/octostate/compare/v0.4.1...v0.5.0) (2026-03-13)


### Features

* **gitops:** add reconciliation apply executor and config apply ([#62](https://github.com/orang-gaboets/octostate/issues/62)) ([fefdce0](https://github.com/orang-gaboets/octostate/commit/fefdce0ac01523d9f19890266f014d945a2089d3))

## [0.4.1](https://github.com/orang-gaboets/octostate/compare/v0.4.0...v0.4.1) (2026-03-13)


### Bug Fixes

* **ci:** serialize all `release-please` workflow runs ([#57](https://github.com/orang-gaboets/octostate/issues/57)) ([0b0cd35](https://github.com/orang-gaboets/octostate/commit/0b0cd3552cf2a47b13092911b80d6ebd46c5d572))

## [0.4.0](https://github.com/orang-gaboets/octostate/compare/v0.3.0...v0.4.0) (2026-03-13)


### Features

* **config:** add GitOps reconciliation planning command ([#54](https://github.com/orang-gaboets/octostate/issues/54)) ([bbdb431](https://github.com/orang-gaboets/octostate/commit/bbdb431ea331f305e76bb6a4ddc26b877b64a19d))

## [0.3.0](https://github.com/orang-gaboets/octostate/compare/v0.2.0...v0.3.0) (2026-03-10)


### Features

* **audit:** add actual-state snapshot pull command ([#51](https://github.com/orang-gaboets/octostate/issues/51)) ([6f60366](https://github.com/orang-gaboets/octostate/commit/6f60366570c5c21cb5da01f893fe95dff94f74f6))

## [0.2.0](https://github.com/orang-gaboets/octostate/compare/v0.1.0...v0.2.0) (2026-03-06)


### Features

* add offline GitOps config validate command and single-file organization.yaml schema ([#48](https://github.com/orang-gaboets/octostate/issues/48)) ([e76484a](https://github.com/orang-gaboets/octostate/commit/e76484ac8d3ff26c279bd82932270e55698721da))

## 0.1.0 (2026-03-03)

Initial release of `octostate`.

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
