# Contributor showcase

The `Contributors` section of [`README.md`](../../README.md) is generated. This
guide covers how it works, how to correct it, and what it deliberately does not
do.

## What it is

`cmd/contributors` reads the repository's GitHub contributor list, applies the
overrides in [`.github/contributors.yml`](../../.github/contributors.yml), and
rewrites the region between these markers:

```html
<!-- contributors:start -->
<!-- contributors:end -->
```

Everything outside the markers is left untouched. If either marker is missing,
duplicated, or out of order, the tool fails rather than guessing where the
section belongs.

## Running it

```bash
go run ./cmd/contributors
```

Flags: `-repository` (default `orang-gaboets/octostate`), `-readme` (default
`README.md`), and `-config` (default `.github/contributors.yml`).

`GITHUB_TOKEN` is optional. The contributor endpoint returns public data, so the
token only raises the API rate limit and needs no privileged scope.

Running against an unchanged repository is a no-op: the tool compares the
rendered result with the file and writes nothing when they match, so it never
produces an empty commit.

## Automation

[`.github/workflows/contributors.yml`](../../.github/workflows/contributors.yml)
runs weekly and on manual dispatch. When the showcase changes it pushes
`chore/contributor-showcase` and opens a pull request, reusing the existing pull
request if one is already open.

The workflow is read-only by default. Only the update job requests
`contents: write` and `pull-requests: write` — enough to push a branch and open
a pull request, and nothing more. There is no third-party action, GitHub App, or
external service in this path: it uses the repository's own Go tool plus `git`
and `gh`, so no additional trust is delegated.

One consequence worth knowing: pull requests created with `GITHUB_TOKEN` do not
trigger workflow runs. That is acceptable here because the change is confined to
generated Markdown, but close and reopen the pull request if you want CI to run
on it.

## Fixing attribution

Edit [`.github/contributors.yml`](../../.github/contributors.yml). Do not
hand-edit the generated markup — the next run overwrites it.

```yaml
exclude:
  - login-to-hide

include:
  - login: someone
    name: Someone Example
```

- `exclude` removes a login from the showcase. Matching is case-insensitive.
- `include` adds someone automatic discovery misses — a contributor whose work
  did not land as a commit authored by their account, or a meaningful non-code
  contribution. `name` is optional and becomes the accessible label.
- `exclude` wins over `include`.

An unknown key is an error rather than a silent no-op, so a misspelling cannot
quietly disable an override you intended.

Bot and service accounts are excluded automatically, both by GitHub's account
type and by the `[bot]` login suffix, so they do not need `exclude` entries.

## Design constraints

**It is recognition, not a leaderboard.** Ordering is alphabetical by login.
Nothing scores or ranks contributors — not commits, lines changed, pull
requests, issues, or tenure. The GitHub API returns a contribution count and the
code deliberately ignores it.

**Only public identity is used.** An entry needs a login; the profile and avatar
URLs are derived from it. No contributor email address or other non-public
metadata is read or written. Deriving the avatar URL rather than storing the
API's also keeps output stable across API responses.

Avatars are rendered at a fixed 100px with `alt` text set to the display name,
falling back to the login.

## Tests

`internal/contributors` covers ordering independence from input order and
contribution count, bot exclusion, both override paths and their precedence,
marker handling including malformed markers, HTML escaping of contributor text,
and idempotency.

```bash
go test ./internal/contributors/ ./cmd/contributors/
```
