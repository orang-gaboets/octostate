# Go API and Consumer Guide

Octostate is both a command-line tool and a reusable GitOps engine. This guide
explains how Go developers should consume the module, which package surfaces are
recommended, and how compatibility is handled for the stable `v1` module.

## Choose the right consumption path

Octostate has three different Go and repository workflows. They should not be
treated as interchangeable:

| Goal | Use | Result |
| --- | --- | --- |
| Install the CLI from source | `go install github.com/orang-gaboets/octostate/cmd/octostate@v<version>` | Builds and installs the `octostate` executable |
| Embed Octostate in another Go program | `go get github.com/orang-gaboets/octostate@v<version>` | Adds or updates the Octostate module dependency |
| Develop Octostate itself | `git clone https://github.com/orang-gaboets/octostate.git` | Obtains the repository for contributor development |

### Use Octostate as a CLI

For releases with prebuilt archives, follow the installation instructions in
the [project README](../README.md). When a Go-native installation from source
is appropriate, use the version-qualified command:

```bash
go install github.com/orang-gaboets/octostate/cmd/octostate@v<version>
```

This installs the CLI command. It does not add Octostate as a library
dependency to the current Go module. Automation should pin an explicit release
version rather than using `@latest`.

### Embed Octostate in another Go program

Add the module as a dependency:

```bash
go get github.com/orang-gaboets/octostate@v<version>
```

Then import only packages identified below as appropriate for external use.
`go get` manages a dependency in the consuming module; it is not the command
installation path for `octostate`.

### Develop Octostate itself

Clone the repository and follow the [development guide](maintainers/development.md)
for setup, testing, and contributor checks. Cloning obtains the development
source; it does not install a released CLI or add a dependency to another
module.

## What Go makes importable

Go applies a language-enforced import boundary to directories named `internal`.
An external module cannot import an `internal` package unless it is within the
allowed parent tree. This rule is enforced by the Go toolchain.

The `pkg` directory has no special meaning to the Go language. It is a
repository convention. An exported identifier in a package outside an
`internal` directory can generally be imported by another module, whether or
not Octostate recommends that package as a stable consumer surface.

Package placement alone therefore does not create or remove a compatibility
promise. Existing externally importable APIs must be considered before they
are changed, even when their package is implementation-oriented or has not yet
been documented as a recommended API.

## Current package surface

The current module was audited by package family. The classifications below
describe intended use; they do not move or hide any existing package.

| Current packages | Visibility | Support status and intended role |
| --- | --- | --- |
| `pkg/gitops/config`, `pkg/gitops/state`, `pkg/gitops/plan`, `pkg/gitops/diff`, `pkg/gitops/snapshot` | Externally importable | Supported reusable API: recommended building blocks for loading desired state, representing state, planning, and offline analysis |
| `pkg/github` (including `client`, `logging`, and resource services); `pkg/gitops/collector`, `pkg/gitops/apply`, `pkg/gitops/syncfromlive` | Externally importable | Implementation-oriented and not the primary stable SDK surface; existing exported APIs remain compatibility-sensitive |
| Non-`internal` packages under `cmd/**`, including `cmd/octostate/*` | Importable according to normal Go package rules, except executable `main` packages | CLI-only implementation surface; install `cmd/octostate` rather than importing command packages |
| `internal/**`, `cmd/octostate/internal/**`, `pkg/gitops/internal/**` | Non-importable from external modules | Implementation details protected by Go's `internal` import rule |

The supported reusable API list is intentionally focused. A package being
implementation-oriented does not make its existing exported API safe to break
casually; it means only that new consumers should prefer the documented
supported packages when possible. If a package should eventually move behind
an `internal` boundary, track that migration separately and choose a compatible
transition rather than moving it as unrelated cleanup.

## Compatibility policy for the `v1` module

Octostate uses one Go module and one release version stream for both its CLI and
its Go packages. Go API compatibility is therefore a release concern alongside
CLI compatibility; there is no separate SDK version or package release stream.

For the current `v1` module, the normal release boundaries are:

| Release | Go API expectation |
| --- | --- |
| Patch release, such as `v1.3.1` | Preserve source compatibility; limit changes to compatible fixes, documentation, and internal implementation updates. |
| Minor release, such as `v1.4.0` | Preserve source compatibility; additive APIs and deprecations are allowed. |
| Major release, such as `v2.0.0` | May introduce source-incompatible changes. The Go module path becomes `github.com/orang-gaboets/octostate/v2`, and migration notes must describe the break. |

An exceptional source-incompatible change within `v1` requires an explicit
maintainer decision, a tracked compatibility rationale, and release notes that
explain the impact. It is not the normal alternative to a major release.

### Existing importable APIs

For an existing `v1` package outside an `internal` directory, an exported
identifier is compatibility-sensitive. Source-incompatible changes include
removing an exported identifier, changing an exported function or method
signature, or changing an exported type in a way that breaks consumers.

Such changes must be evaluated against the project's semantic-versioning and
release policy. They must not be introduced casually in a patch or minor
release merely because the package was previously described as implementation-
oriented or because its API was undocumented.

If the project decides that an importable package should no longer be public,
the package migration must be tracked separately and handled at an appropriate
compatibility boundary. This document does not authorize moving or removing
existing packages.

### Supported reusable packages

For the supported reusable API:

The compatibility contract also covers externally importable types, interfaces,
constants, variables, or other API elements that form part of the exported
contract of a supported package, even when they are declared in an otherwise
implementation-oriented package.

- preserve source compatibility within the documented `v1` contract;
- prefer additive changes where practical;
- deprecate obsolete APIs before removal where feasible;
- document a supported replacement and migration steps; and
- reserve source-incompatible removal for a major-version boundary or another
  explicitly justified compatibility boundary.

New implementation-only code should use an appropriate `internal` package so
that it does not become externally importable accidentally.

### Deprecation and removal

Use a Go `Deprecated:` documentation comment when an exported API is obsolete.
The comment should identify the supported replacement when one exists. Release
or compatibility notes should describe the migration and the planned removal
boundary.

When feasible, allow at least one subsequent minor release after deprecation
before removing an API. A security, correctness, or other exceptional case may
require a different schedule, but the exception and its compatibility impact
should be stated in the relevant release notes. Deprecated APIs do not need to
be retained permanently.

This policy makes #259 actionable: `pkg/github/client.New` is a deprecated,
source-compatible constructor retained for existing consumers. It should remain
until the project selects an appropriate removal boundary and completes the
required migration communication. This documentation work does not remove it;
consumers needing construction errors should use `NewPAT`, as recommended by the
`New` deprecation comment.

## Module downloads and source archives

Normal Go module management may place downloaded module source, documentation,
and tests in the consumer's shared module cache. That cache behavior is an
implementation detail of Go dependency management, not a reason to redesign
Octostate.

This policy does not require pruning GitHub source archives, adding
`.gitattributes export-ignore` rules, splitting the repository into multiple
modules, or copying individual packages into consumer repositories. Those
would require separate design and compatibility decisions.
