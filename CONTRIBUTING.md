# Contributing to Octostate

Thanks for your interest in Octostate. Bug fixes, features, tests,
documentation, refactoring, performance work, and CI or maintenance
improvements are all welcome.

Contributions are reviewed on their merits. Maintainers may request changes, or
decide that a proposal does not fit the project's direction or scope, so it is
worth agreeing on the shape of a change before investing much time in it.

This guide is the entry point for contributors. It deliberately stays short and
links to the authoritative documents rather than repeating them. The
[development guide](docs/maintainers/development.md) remains the source of
truth for setup, commands, and validation.

## Start with an issue

Before starting work you expect to turn into a pull request, find an existing
[issue](https://github.com/orang-gaboets/octostate/issues) or raise one. An
issue establishes the problem, settles the scope, avoids duplicated effort, and
gives reviewers the context they need later.

This matters most for:

- new features;
- CLI or API changes;
- configuration-schema changes;
- behavior changes that affect compatibility;
- substantial refactors; and
- security-sensitive changes.

This is a recommendation, not a gate. A typo fix or an obviously correct small
correction does not need an issue first. Use judgement proportionate to the
change.

## Workflow

1. Find or raise the relevant issue.
2. Create a branch, or fork the repository if you do not have write access.
3. Make a focused, coherent change.
4. Add or update tests and documentation where the change calls for it.
5. Run the validation relevant to what you changed.
6. Open a pull request and link the issue.
7. Describe the change, the validation you ran, and any compatibility or
   security impact.
8. Respond to review feedback.

### Branch names

Descriptive branch names are strongly recommended. Nothing enforces them, and
this guide does not add such enforcement. Common prefixes:

```text
feat/<short-description>
fix/<short-description>
docs/<short-description>
test/<short-description>
refactor/<short-description>
ci/<short-description>
chore/<short-description>
```

### Scope

A pull request should be one coherent change that can be reviewed as a whole.

One pull request **may** resolve several issues when they are closely related,
share the same implementation context, are more naturally done together, and
still read as one change. There is no one-issue-per-pull-request rule.

What to avoid is the opposite: bundling unrelated fixes, features, refactors,
or cleanup into the same pull request because they happened to be in progress
at the same time. Optimize for reviewability, not for a low pull-request count.

### Linking issues

Use a closing keyword when the pull request fully resolves an issue, and a
plain reference when it does not:

```text
Closes #123
Related to #456
```

A pull request that coherently resolves several issues may use more than one
`Closes` line.

## Commit and pull-request titles

Commits that land on `main` must stay compatible with the repository's
Conventional Commit and release-please workflow. Because pull requests are
commonly squash-merged, **the pull-request title is usually the subject that
lands**, so it is the one to get right:

```text
feat: add ...
fix: correct ...
docs: document ...
test: cover ...
refactor: simplify ...
perf: improve ...
ci: update ...
chore: maintain ...
```

You do not need to read the full Conventional Commits specification. Matching
the shape above is enough. Local commits may follow the same convention.

### Do not hand-manage release state

Release-please owns versioning, changelog entries, tags, and releases. A normal
pull request should not bump the version, edit release manifests, or write
changelog entries by hand. Exceptional release-repair work follows the
[release documentation](docs/maintainers/releases.md) instead of this guide.

## Development setup

You need Go — [`go.mod`](go.mod) declares the language version — and a
checkout of the repository.

```bash
go build ./...
go test ./...
```

The [development guide](docs/maintainers/development.md) is authoritative for
toolchain selection, the full command set, and validation details.

Contributing does not require maintainer-only credentials. GitHub App keys,
environment secrets, and organization secrets are used by release and
maintenance automation, not by normal development.

### Pre-commit

The repository ships a pre-commit configuration and it is a convenient local
workflow, but installing it is not a prerequisite for contributing. CI remains
authoritative for required checks.

## Validation

Run what your change warrants. CI runs the full set regardless; the point of
running locally is a faster loop, not duplicating CI.

| You changed | Also run |
| --- | --- |
| Go code | formatting, `go test ./...`, `go vet ./...`, `golangci-lint run` |
| GitHub Actions workflows | `actionlint` |
| GitOps concurrency or shared state | the scoped race-detector suite |
| Documentation only | check links, paths, commands, config examples, and behavioral claims |
| Dependencies or security-relevant code | the relevant security and dependency checks |

Exact commands live in the [development guide](docs/maintainers/development.md).

A prose-only documentation change does not need the full Go test suite unless
it is coupled to behavior that needs verifying. Equally, a one-line dependency
bump does not need every security scanner run by hand when CI already covers
it.

### Tests

Behavior changes and bug fixes should normally come with tests, and a bug fix
should add regression coverage wherever that is practical. If a meaningful test
genuinely is not practical, say so in the pull request and explain why.

Changes that cannot affect executable behavior — a typo in prose, for
instance — do not need new tests.

### Documentation

A user-facing behavior change should update the authoritative documentation in
the same pull request. This applies to CLI behavior and output, the
desired-state and configuration schema, GitOps semantics, authentication
behavior, automation contracts, and anything affecting compatibility.

Try not to merge a behavior change that knowingly leaves the documented
contract stale.

### Compatibility

If your change means users may have to adjust configuration, scripts,
automation, command parsing, or another integration, say so explicitly in the
pull-request description. You do not need to edit release notes or changelog
state yourself.

## Project invariants

A few expectations are durable and worth preserving:

- command results belong on stdout;
- diagnostics and errors belong on stderr;
- machine-readable output stays deterministic — normalize and sort before
  printing or writing;
- this repository is the engine layer, and organization-specific policy belongs
  in the control repository; and
- tests and authoritative documentation stay aligned with behavior.

[AGENTS.md](AGENTS.md), the
[GitOps architecture](docs/gitops/architecture.md), and the
[control-repository integration guide](docs/gitops/control-repo-integration.md)
carry the full detail.

## AI-assisted contributions

Using AI assistance is fine and does not change the standard a contribution is
held to.

Whoever opens the pull request is accountable for it: understanding the change,
reviewing generated code and prose, verifying correctness, running the
validation, checking for unintended edits, complying with licensing and
repository policy, and being able to explain and maintain the work through
review. An AI-assisted pull request is expected to meet the same correctness,
testing, documentation, and security bar as any other.

## Security

**Do not report suspected vulnerabilities in public issues or pull requests.**
Use the private process in [SECURITY.md](SECURITY.md).

Never commit or expose tokens, GitHub App private keys, repository or
organization secrets, credentials, private environment values, unredacted
sensitive logs, or other non-public information. Examples and tests should use
placeholders, fixtures, mocks, or redacted data.

## Pull-request expectations

Give reviewers enough to review efficiently:

- what changed and why;
- the issue or issues it relates to;
- the validation and tests you ran;
- documentation updated, where applicable;
- compatibility impact, if any; and
- security considerations, if relevant.

Focused, readable diffs get reviewed faster.

## Review

Pull requests are reviewed before merge. Reviewers may request changes, CI and
required checks must pass, and unresolved feedback should be addressed or
answered. Maintainers are ultimately responsible for deciding whether a change
fits the project.

Thanks for contributing.
