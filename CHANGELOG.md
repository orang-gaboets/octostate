# Changelog

All notable changes to this project will be documented in this file.

The release process is managed by `release-please`, which updates this changelog
when release pull requests are created and merged.

## [0.1.0](https://github.com/orang-gaboets/octostate/compare/v1.2.0...v0.1.0) (2026-08-28)


### ⚠ BREAKING CHANGES

* validate v1.0.0 readiness ([#108](https://github.com/orang-gaboets/octostate/issues/108))

### Features

* add offline GitOps config validate command and single-file organization.yaml schema ([#48](https://github.com/orang-gaboets/octostate/issues/48)) ([e76484a](https://github.com/orang-gaboets/octostate/commit/e76484ac8d3ff26c279bd82932270e55698721da))
* **apply:** pre-resolve invite usernames before sequential execution ([#101](https://github.com/orang-gaboets/octostate/issues/101)) ([79d1ec1](https://github.com/orang-gaboets/octostate/commit/79d1ec112b97bf3d4af40b4787aa3f2745cb84e5))
* **audit:** add actual-state snapshot pull command ([#51](https://github.com/orang-gaboets/octostate/issues/51)) ([6f60366](https://github.com/orang-gaboets/octostate/commit/6f60366570c5c21cb5da01f893fe95dff94f74f6))
* **audit:** add offline GitOps drift diff command ([#64](https://github.com/orang-gaboets/octostate/issues/64)) ([24518dd](https://github.com/orang-gaboets/octostate/commit/24518dd328317b47b441a79543af68176c673f48))
* **audit:** parallelize invite username resolution ([#97](https://github.com/orang-gaboets/octostate/issues/97)) ([f0f8536](https://github.com/orang-gaboets/octostate/commit/f0f85367a47fb7b9076cbdff0bd9f68df729d8bc))
* **cli:** add config-only proposals for repository and topic mutations ([#195](https://github.com/orang-gaboets/octostate/issues/195)) ([2f82471](https://github.com/orang-gaboets/octostate/commit/2f82471453d82439b84138fa709dcc4a0aedf3bb))
* **cli:** add shared config-only mutation helper ([#191](https://github.com/orang-gaboets/octostate/issues/191)) ([375b286](https://github.com/orang-gaboets/octostate/commit/375b2865d814f282582c4f906f4b65f02c1340a3))
* **config:** add apply preflight check mode ([#131](https://github.com/orang-gaboets/octostate/issues/131)) ([c07c97c](https://github.com/orang-gaboets/octostate/commit/c07c97c5245d6add6c0646cbb271fb175f55fe0e))
* **config:** add GitOps reconciliation planning command ([#54](https://github.com/orang-gaboets/octostate/issues/54)) ([bbdb431](https://github.com/orang-gaboets/octostate/commit/bbdb431ea331f305e76bb6a4ddc26b877b64a19d))
* **config:** add read-only apply preflight probes ([#134](https://github.com/orang-gaboets/octostate/issues/134)) ([352aba9](https://github.com/orang-gaboets/octostate/commit/352aba9b7de524fd232bd5961ed853e4ae591597))
* **config:** add sync-from-live adopt mode ([#82](https://github.com/orang-gaboets/octostate/issues/82)) ([db70738](https://github.com/orang-gaboets/octostate/commit/db70738f81dd3d78d6b8b93f0818fb4c47b8b585))
* **config:** add sync-from-live bootstrap mode ([#74](https://github.com/orang-gaboets/octostate/issues/74)) ([7c5a266](https://github.com/orang-gaboets/octostate/commit/7c5a266f6a9ee1860dc453721d7815d53eb9d179))
* **config:** add sync-from-live materialize mode ([#84](https://github.com/orang-gaboets/octostate/issues/84)) ([b28aad2](https://github.com/orang-gaboets/octostate/commit/b28aad27b78b1b8a699ad31788c57de0f2f91dc0))
* **config:** split config plan output into executable and skipped actions ([#89](https://github.com/orang-gaboets/octostate/issues/89)) ([ad1e1aa](https://github.com/orang-gaboets/octostate/commit/ad1e1aaea4461ac39cc478c95353f63c33385e0f))
* **config:** validate repository topic constraints ([#198](https://github.com/orang-gaboets/octostate/issues/198)) ([6c6a6d3](https://github.com/orang-gaboets/octostate/commit/6c6a6d3f1b41a58c0136d84fd5aeacce1528eacc))
* **delete:** add config-only repository and team deletion proposals ([#206](https://github.com/orang-gaboets/octostate/issues/206)) ([21f9a1e](https://github.com/orang-gaboets/octostate/commit/21f9a1efdb082293a5c07f14d858e5f6910b88af))
* **diff:** parallelize offline audit diff build phases ([#99](https://github.com/orang-gaboets/octostate/issues/99)) ([78a799c](https://github.com/orang-gaboets/octostate/commit/78a799cf60c736f032e870b37be0010e2beafdd2))
* **gitops:** add bounded concurrency to collectors and plan phases ([#92](https://github.com/orang-gaboets/octostate/issues/92)) ([5c08dbb](https://github.com/orang-gaboets/octostate/commit/5c08dbb10a0b728b63cb22b7e6b6874668939daf))
* **gitops:** add first-class organization membership ([#80](https://github.com/orang-gaboets/octostate/issues/80)) ([ed97c63](https://github.com/orang-gaboets/octostate/commit/ed97c6374327b4e47317fe589a5447c1b38f2bcd))
* **gitops:** add reconciliation apply executor and config apply ([#62](https://github.com/orang-gaboets/octostate/issues/62)) ([fefdce0](https://github.com/orang-gaboets/octostate/commit/fefdce0ac01523d9f19890266f014d945a2089d3))
* **gitops:** make repository reconciliation fields presence-aware ([#67](https://github.com/orang-gaboets/octostate/issues/67)) ([91002b3](https://github.com/orang-gaboets/octostate/commit/91002b3ccfa3a1d42a13330f8c5af659f9e48b31))
* **organization:** add config-only organization invite proposals ([#202](https://github.com/orang-gaboets/octostate/issues/202)) ([ec574ab](https://github.com/orang-gaboets/octostate/commit/ec574ab916505debc207037e30b5d65e1bdb7067))
* **organization:** add pending invitation list command ([#87](https://github.com/orang-gaboets/octostate/issues/87)) ([4bcba62](https://github.com/orang-gaboets/octostate/commit/4bcba62d89be7315ef330de171312fae8ebedc05))
* **team:** add config-only team create and edit proposals ([#197](https://github.com/orang-gaboets/octostate/issues/197)) ([8f63612](https://github.com/orang-gaboets/octostate/commit/8f63612b042c036e97b1d337da518770e31657b1))
* **team:** add config-only team membership proposals ([#199](https://github.com/orang-gaboets/octostate/issues/199)) ([b1838ed](https://github.com/orang-gaboets/octostate/commit/b1838ed646c5e073b76fe9a46506821fa4878fbf))
* **team:** add config-only team repository permission proposals ([#200](https://github.com/orang-gaboets/octostate/issues/200)) ([5665fb5](https://github.com/orang-gaboets/octostate/commit/5665fb51a7bb251d08f0a0dfe5204dce475efc78))
* validate v1.0.0 readiness ([#108](https://github.com/orang-gaboets/octostate/issues/108)) ([71242ad](https://github.com/orang-gaboets/octostate/commit/71242adf3e8a8884571d3896f6555748a8df0da7))


### Bug Fixes

* **ci:** gate release-please auto-merge on publisher approval ([#125](https://github.com/orang-gaboets/octostate/issues/125)) ([3e42cec](https://github.com/orang-gaboets/octostate/commit/3e42cecd22f2e9dfe81d75318dbb06111ca8dccd))
* **ci:** merge release-please PRs after checks pass ([#120](https://github.com/orang-gaboets/octostate/issues/120)) ([af4a054](https://github.com/orang-gaboets/octostate/commit/af4a054b5e9a60cc61e9f212d9166795d58ee104))
* **ci:** require configured release and lifecycle labels ([#252](https://github.com/orang-gaboets/octostate/issues/252)) ([acea7cc](https://github.com/orang-gaboets/octostate/commit/acea7cca13132d5650206cef7aac450260f78193))
* **ci:** serialize all `release-please` workflow runs ([#57](https://github.com/orang-gaboets/octostate/issues/57)) ([0b0cd35](https://github.com/orang-gaboets/octostate/commit/0b0cd3552cf2a47b13092911b80d6ebd46c5d572))
* **cli:** align team members mutation output with operation envelopes ([#215](https://github.com/orang-gaboets/octostate/issues/215)) ([cc2ff81](https://github.com/orang-gaboets/octostate/commit/cc2ff814ab9923429e0997c6abbdea8b1edc21f7))
* **config:** classify config runtime failures by phase ([#144](https://github.com/orang-gaboets/octostate/issues/144)) ([1092193](https://github.com/orang-gaboets/octostate/commit/10921934e813c4a71ba3b45bf96b49a6cfdc817f))
* **config:** enforce organization-local repository ownership ([#213](https://github.com/orang-gaboets/octostate/issues/213)) ([0502605](https://github.com/orang-gaboets/octostate/commit/05026055648df7f727c95b2e0acdc172d2b10b2f))
* **config:** normalize programmatic desired state before reconciliation ([#235](https://github.com/orang-gaboets/octostate/issues/235)) ([5dcea99](https://github.com/orang-gaboets/octostate/commit/5dcea99e75878ad8bcef63b48e3329614c3dec7d))
* **config:** reject duplicate invite identities during validation ([#209](https://github.com/orang-gaboets/octostate/issues/209)) ([c052b37](https://github.com/orang-gaboets/octostate/commit/c052b3775b1b775224d5395397013d2d4120135f))
* **deps:** resolve Go security alerts ([#117](https://github.com/orang-gaboets/octostate/issues/117)) ([f26022a](https://github.com/orang-gaboets/octostate/commit/f26022a772536a827830296c41f20d49f3f0fdfe))
* **diff:** gate team repository permission availability ([#227](https://github.com/orang-gaboets/octostate/issues/227)) ([7710ed2](https://github.com/orang-gaboets/octostate/commit/7710ed20855bfa52319a314e25c5224016f0bad6))
* **gitops:** preserve template state across same-plan applies ([#143](https://github.com/orang-gaboets/octostate/issues/143)) ([7f3d8b1](https://github.com/orang-gaboets/octostate/commit/7f3d8b19ba1568170ad0515d0ef6bcaca2da10c2))
* **plan:** gate team repo permissions on repository availability ([#142](https://github.com/orang-gaboets/octostate/issues/142)) ([67ef3b1](https://github.com/orang-gaboets/octostate/commit/67ef3b19ae9e7c8fbd1545d71104e852255a145f))
* **plan:** honor managed repository template dependencies ([#216](https://github.com/orang-gaboets/octostate/issues/216)) ([5523c6f](https://github.com/orang-gaboets/octostate/commit/5523c6fc0c1cf1b106ee42479426e8abe748b768))
* propagate PAT GitHub client construction errors ([#168](https://github.com/orang-gaboets/octostate/issues/168)) ([5f1050c](https://github.com/orang-gaboets/octostate/commit/5f1050c06a7a1ea6e8a44bee50c4180180ac8743))
* scope GitHub App token to the current repository ([#36](https://github.com/orang-gaboets/octostate/issues/36)) ([4e86afd](https://github.com/orang-gaboets/octostate/commit/4e86afde7cd2202bcf399e144a190d7e69e6546a))
* **team:** align proposal casing and document role downgrade ([#212](https://github.com/orang-gaboets/octostate/issues/212)) ([090c75d](https://github.com/orang-gaboets/octostate/commit/090c75d295fa9e31202688dec2f1a267cedee308))


### Miscellaneous Chores

* bootstrap first automated release ([#35](https://github.com/orang-gaboets/octostate/issues/35)) ([4fead29](https://github.com/orang-gaboets/octostate/commit/4fead296ec998c117be3919a8ab0f75f462fe867))

## [1.2.0](https://github.com/orang-gaboets/octostate/compare/v1.1.1...v1.2.0) (2026-08-27)


### Features

* **cli:** add config-only proposals for repository and topic mutations ([#195](https://github.com/orang-gaboets/octostate/issues/195)) ([2f82471](https://github.com/orang-gaboets/octostate/commit/2f82471453d82439b84138fa709dcc4a0aedf3bb))
* **cli:** add shared config-only mutation helper ([#191](https://github.com/orang-gaboets/octostate/issues/191)) ([375b286](https://github.com/orang-gaboets/octostate/commit/375b2865d814f282582c4f906f4b65f02c1340a3))
* **config:** validate repository topic constraints ([#198](https://github.com/orang-gaboets/octostate/issues/198)) ([6c6a6d3](https://github.com/orang-gaboets/octostate/commit/6c6a6d3f1b41a58c0136d84fd5aeacce1528eacc))
* **delete:** add config-only repository and team deletion proposals ([#206](https://github.com/orang-gaboets/octostate/issues/206)) ([21f9a1e](https://github.com/orang-gaboets/octostate/commit/21f9a1efdb082293a5c07f14d858e5f6910b88af))
* **organization:** add config-only organization invite proposals ([#202](https://github.com/orang-gaboets/octostate/issues/202)) ([ec574ab](https://github.com/orang-gaboets/octostate/commit/ec574ab916505debc207037e30b5d65e1bdb7067))
* **team:** add config-only team create and edit proposals ([#197](https://github.com/orang-gaboets/octostate/issues/197)) ([8f63612](https://github.com/orang-gaboets/octostate/commit/8f63612b042c036e97b1d337da518770e31657b1))
* **team:** add config-only team membership proposals ([#199](https://github.com/orang-gaboets/octostate/issues/199)) ([b1838ed](https://github.com/orang-gaboets/octostate/commit/b1838ed646c5e073b76fe9a46506821fa4878fbf))
* **team:** add config-only team repository permission proposals ([#200](https://github.com/orang-gaboets/octostate/issues/200)) ([5665fb5](https://github.com/orang-gaboets/octostate/commit/5665fb51a7bb251d08f0a0dfe5204dce475efc78))


### Bug Fixes

* **cli:** align team members mutation output with operation envelopes ([#215](https://github.com/orang-gaboets/octostate/issues/215)) ([cc2ff81](https://github.com/orang-gaboets/octostate/commit/cc2ff814ab9923429e0997c6abbdea8b1edc21f7))
* **config:** enforce organization-local repository ownership ([#213](https://github.com/orang-gaboets/octostate/issues/213)) ([0502605](https://github.com/orang-gaboets/octostate/commit/05026055648df7f727c95b2e0acdc172d2b10b2f))
* **config:** normalize programmatic desired state before reconciliation ([#235](https://github.com/orang-gaboets/octostate/issues/235)) ([5dcea99](https://github.com/orang-gaboets/octostate/commit/5dcea99e75878ad8bcef63b48e3329614c3dec7d))
* **config:** reject duplicate invite identities during validation ([#209](https://github.com/orang-gaboets/octostate/issues/209)) ([c052b37](https://github.com/orang-gaboets/octostate/commit/c052b3775b1b775224d5395397013d2d4120135f))
* **diff:** gate team repository permission availability ([#227](https://github.com/orang-gaboets/octostate/issues/227)) ([7710ed2](https://github.com/orang-gaboets/octostate/commit/7710ed20855bfa52319a314e25c5224016f0bad6))
* **plan:** honor managed repository template dependencies ([#216](https://github.com/orang-gaboets/octostate/issues/216)) ([5523c6f](https://github.com/orang-gaboets/octostate/commit/5523c6fc0c1cf1b106ee42479426e8abe748b768))
* **team:** align proposal casing and document role downgrade ([#212](https://github.com/orang-gaboets/octostate/issues/212)) ([090c75d](https://github.com/orang-gaboets/octostate/commit/090c75d295fa9e31202688dec2f1a267cedee308))

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
