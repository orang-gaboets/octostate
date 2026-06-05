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

The release-please GitHub App installation also needs to be configured with
`Members: read` so the release approval workflow can verify the approver team.
It also needs `Issues: write` so unauthorized approval attempts can leave a PR
comment from the app bot.

For a major release or first stable release, keep the human validation checklist
and supporting evidence in [`v1.0.0-readiness.md`](v1.0.0-readiness.md) before
intentionally merging a releasable breaking change onto `main`.

## Release PR Merge

This repository also uses `.github/workflows/automerge-release-please.yml` to
merge `release-please` PRs after an explicit approval label has been applied by
an authorized maintainer.

- The approval label is `release: ready`
- Only active members of `@orang-gaboets/octostate-publishers` may apply that label to approve a release
- The workflow only targets same-repository PRs into `main`
- The workflow only targets release branches created by `release-please` (`release-please--branches--*`)
- The workflow verifies the PR author is the configured GitHub App bot
- The workflow verifies the label actor against the `octostate-publishers` team
- Unauthorized `release: ready` labels are removed automatically
- Unauthorized approval attempts leave a PR comment from the release-please app bot
- If `release-please` updates the PR head after approval, the stale `release: ready` label is removed and must be re-applied
- The workflow waits for the `CI` and `CodeQL` release checks to complete before merging
- The workflow uses the configured GitHub App token to merge the release PR directly
- Human-authored PRs are intentionally ignored

The configured GitHub App must be able to:

- bypass the `main` branch ruleset for pull requests
- write issues so it can leave PR comments for unauthorized approval attempts
- write pull requests so it can merge the release PR and remove labels

Keep the release-please app in the `main-protection` ruleset bypass list before
relying on this workflow; otherwise the direct merge can still fail with
`REVIEW_REQUIRED`.

## Release Approval Recovery

If `release: ready` is applied by someone who is not in
`@orang-gaboets/octostate-publishers`, the workflow removes the label and fails
before merge. It also leaves a PR comment explaining why the approval was
rejected and what an authorized publisher should do next.

The workflow does not leave comments when removing stale approvals after
`release-please` updates the PR, to avoid noisy release PR timelines.

If the release-please app cannot verify team membership, confirm that the app
installation has `Members: read` and that the `octostate-publishers` team
exists.
