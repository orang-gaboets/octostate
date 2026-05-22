# Releases

This repository uses [`release-please`](https://github.com/googleapis/release-please)
and the [`release-please-action`](https://github.com/googleapis/release-please-action)
to automate release PRs, changelog updates, Git tags, and GitHub Releases from
Conventional Commits on `main`.

Version updates for this project are performed by `release-please`,
preferably using a GitHub App installation token rather than a
user-owned personal access token. If app credentials are not configured,
the workflow falls back to `GITHUB_TOKEN`. Release state is anchored
explicitly in:
- `release-please-config.json`
- `.release-please-manifest.json`

## Release Workflow Guidelines

- Use Conventional Commit subjects on the commits that land on `main`
- If you use squash merge, put the Conventional Commit prefix in the PR title
- `fix:` produces a patch release, `feat:` produces a minor release, and `!` or `BREAKING CHANGE:` produces a major release
- The workflow prefers a GitHub App installation token created with [`actions/create-github-app-token`](https://github.com/actions/create-github-app-token)
- If the app credentials are not configured, the workflow falls back to `GITHUB_TOKEN`
- In repository settings, enable **Allow GitHub Actions to create and approve pull requests** so the workflow can open release PRs
- Do not manually re-bootstrap historical releases in normal operation; update the manifest or config intentionally if release state ever needs repair

With the manifest baseline in place, future releasable commits on `main` will
cause `release-please` to open or update a release PR automatically from the
current anchored version.

For a major release or first stable release, keep the human validation checklist
and supporting evidence in [`v1.0.0-readiness.md`](v1.0.0-readiness.md) before
intentionally merging a releasable breaking change onto `main`.

## Auto-merge

This repository also uses `.github/workflows/automerge-release-please.yml` to
enable auto-merge for `release-please` PRs created by the configured GitHub
App.

- The workflow only targets same-repository PRs into `main`
- The workflow only targets release branches created by `release-please` (`release-please--branches--*`)
- Human-authored PRs are intentionally ignored
