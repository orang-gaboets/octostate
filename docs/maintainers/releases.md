# Releases

This repository uses [`release-please`](https://github.com/googleapis/release-please)
and the [`release-please-action`](https://github.com/googleapis/release-please-action)
to automate release PRs, changelog updates, Git tags, and GitHub Releases from
Conventional Commits on `main`.

Version updates for this project are performed by `release-please`,
preferably using a GitHub App installation token rather than a
user-owned personal access token. If app credentials are not configured,
the workflow falls back to `GITHUB_TOKEN`. Release configuration and version
state are tracked in:
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
- Do not keep `last-release-sha` configured after a good release PR has merged; it is a repair override, not normal steady-state configuration

With the manifest baseline in place, future releasable commits on `main` will
cause `release-please` to open or update a release PR automatically from the
current anchored version.

The release-please GitHub App installation also needs these permissions for the
release approval workflow:

- `Contents: write` so the app bot can update repository contents through PR
  merge operations
- `Pull requests: write` so the app bot can merge release PRs
- `Issues: write` so the app bot can remove approval labels and leave PR
  comments
- `Checks: read` and `Commit statuses: read` so the app bot can wait for
  required release checks before merging
- `Members: read` so the app bot can verify the approver team

Unlike the release-please workflow above, the release approval auto-merge
workflow requires this GitHub App token and fails closed if the token cannot be
created.

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

The `release: ready` label belongs to this repository's approval gate. The
`autorelease:*` labels belong to `release-please` and should not be manually
edited during normal release flow:

- `autorelease: pending`
- `autorelease: tagged`
- `autorelease: snapshot`
- `autorelease: published`
- `autorelease: triggered`

Only close generated release PRs or delete `release-please--branches--*`
branches during an intentional release-state repair. In normal operation,
release-please manages that generated PR branch itself.

If the approval label ever changes, set the repository Actions variable
`RELEASE_READY_LABEL` to the new label name so runtime checks and workflow
concurrency cancellation use the same value.

The configured GitHub App must be able to:

- bypass the `main` branch ruleset for pull requests
- read checks and commit statuses so it can wait for required release checks
- read organization members so it can verify the release approver team
- write contents and pull requests so it can merge release PRs
- write issues so it can remove approval labels and leave PR comments

Keep the release-please app in the `main-protection` ruleset bypass list before
relying on this workflow; otherwise the direct merge can still fail with
`REVIEW_REQUIRED`.

## Release Approval Recovery

If `release: ready` is applied by someone who is not in
[@orang-gaboets/octostate-publishers](https://github.com/orgs/orang-gaboets/teams/octostate-publishers),
the workflow removes the label and fails before merge. It also leaves a PR
comment explaining why the approval was rejected and what an authorized
publisher should do next.

The workflow does not leave comments when removing stale approvals after
`release-please` updates the PR, to avoid noisy release PR timelines.

If the release-please app cannot verify team membership, confirm that the app
installation has `Members: read` and that the
[`octostate-publishers`](https://github.com/orgs/orang-gaboets/teams/octostate-publishers)
team exists. Configuration or GitHub API failures fail closed without removing
the `release: ready` label, so maintainers can retry after fixing the setup.

## Release State Repair

If a release PR merges but the expected tag or GitHub Release is missing, repair
the release state before merging another release PR:

- Confirm `CHANGELOG.md` and `.release-please-manifest.json` already contain the
  intended version
- Create the missing GitHub Release and tag at the merged release PR commit,
  using the matching changelog section as release notes
- Remove or update any temporary `last-release-sha` override so future
  release-please runs use the latest valid release baseline
- Close any generated release PR that was created from stale release state
- Delete the generated `release-please--branches--*` branch after closing the
  stale release PR
- Re-run the `Release Please` workflow manually and confirm it does not recreate
  the stale release PR
