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

## Compatibility Notes for Releases

When a release includes a concrete upgrade adaptation for existing
configuration, scripts, or consuming integrations, keep the detailed guidance
in a versioned compatibility document under `docs/maintainers/`. For `v1.2.0`,
the canonical example is
[`v1.2.0-compatibility.md`](v1.2.0-compatibility.md).

Only create one of these documents when an upgrading user must change released
configuration, a released script, or a consuming integration. Ordinary bug
fixes, internal implementation changes, and other user-invisible adjustments do
not need compatibility notes.

`release-please` remains the source of truth for generated changelog content,
but its commit-subject summaries are not enough to carry migration guidance on
their own. The release PR and published GitHub Release for a qualifying release
must each include a direct pointer to the versioned compatibility document so
the detailed notes are reachable from both release surfaces.

Keep the generated changelog entry itself short. Do not move the detailed
compatibility prose into `CHANGELOG.md`, and do not restore a permanent
free-floating `Unreleased` section for hand-maintained migration text. The
versioned compatibility document is the canonical detailed home; release
surfaces should link to it rather than duplicate it.

Before an authorized publisher or maintainer applies the configured release
approval label (default `release: ready`) or otherwise merges a qualifying
generated release PR, add the exact compatibility-document pointer to the
generated release PR body and read the
PR body back to confirm it persisted. If `release-please` regenerates the PR
body, repeat this checkpoint after the final update. For `v1.2.0`, that pointer is
`Compatibility and migration notes: https://github.com/orang-gaboets/octostate/blob/main/docs/maintainers/v1.2.0-compatibility.md`.

After publication, read back the GitHub Release body and confirm the same
pointer is present. If it is missing, an authorized publisher must add the
pointer to the GitHub Release body and read it back before considering issue
#218 closed. Those remote writes and read-backs are outside the local
documentation change here and must be performed separately.

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

For future release validation, use
[`release-readiness.md`](release-readiness.md). That guide explains that patch
releases usually need only lightweight validation, risky patch releases need
targeted validation, meaningful minor releases should use standard readiness,
and major releases should always use full readiness. The historical
[`v1.0.0-readiness.md`](v1.0.0-readiness.md) file is retained as evidence for
the first stable release.

## Go Toolchain Remediation Automation

Go toolchain vulnerability remediation is separate from the release-please
automation above. The authoritative workflow file is
`.github/workflows/go-toolchain-remediation.yml`.

Keep the trust boundary from #223 intact:

- `.github/workflows/govulncheck.yml` stays detection-only and read-only.
- `.github/workflows/go-toolchain-remediation.yml` is the only workflow that
  may create a remediation branch and draft PR.
- The remediation workflow does not run on `pull_request`; it runs only on its
  schedule and on `workflow_dispatch`, and the remediation job itself is gated
  to `refs/heads/main`.

The remediation workflow checks out `main`, fetches `origin/main`, and pins the
candidate run to the exact detached `origin/main` SHA before any write-capable
GitHub App token is created. Its default workflow permissions remain
`contents: read`, checkout disables persisted credentials, and the maintenance
App token requests only `contents: write` and `pull-requests: write`.

Actions configuration for this automation remains external and must be
maintained separately. The workflow declares the protected
`go-toolchain-remediation` environment with deployment record creation
disabled because the environment is used for credential scoping and access
control rather than application deployment. Configure the environment's branch
policy to allow `main` only, and scope access to repositories approved to run
this workflow:

- Organization Actions variable: `GO_TOOLCHAIN_REMEDIATION_APP_ID`
- Environment secret in `go-toolchain-remediation`:
  `GO_TOOLCHAIN_REMEDIATION_APP_PRIVATE_KEY`

After populating the environment secret, remove any old organization-level copy
of the private key. This repository documentation does not certify that the
environment, App installation, variable, or secret is currently configured, or
that any branch ruleset is configured a particular way. The required contract
is negative: do not add the maintenance App as a branch or ruleset bypass
actor. The workflow is designed to open a draft PR and then stop behind the
normal review process rather than bypass it.

The mutable boundary is intentionally narrow:

- write-capable token minting is pinned to immutable
  `actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1`
- vulnerability scanning is pinned to
  `golang.org/x/vuln/cmd/govulncheck@v1.5.0`
- the candidate diff must stay limited to `go.mod` and
  `docs/maintainers/development.md`

For an eligible finding set, the workflow creates branch
`ci/go-toolchain-X.Y.Z`, commits `ci: bump Go toolchain to X.Y.Z`, and
opens a draft PR with the same title against `main`. The PR body includes the
stable marker `<!-- octostate-go-toolchain-remediation:v1 -->` together with
current-version, target-version, and pinned-base markers. Duplicate detection
recognizes the stable marker or the deterministic branch prefix, while an exact
duplicate still requires the expected repository, `main` base, App bot, head,
and current/target metadata. The pinned-base marker is recorded for
audit/recovery context, not used as the duplicate-match predicate.

Duplicate handling and failure behavior are fail-closed:

- a matching open remediation PR exits cleanly without new mutation
- a different open recognized remediation PR blocks new automation and
  requires maintainer review
- an existing target branch, changed `origin/main`, classifier/schema mismatch,
  failed candidate validation, or GitHub API/authentication failure aborts the
  run without force-updating existing remediation work

Recovery is also maintainer-driven. If the remote remediation branch is created
but `gh pr create --draft` fails, inspect the branch first, confirm that no
matching PR already exists, then either create the draft PR manually with the
same marker comments or delete the orphan branch after confirming it has no
associated PR. Do not force-push, rewrite, or recycle an older remediation
branch or PR automatically.

Human review begins at the draft PR. The automation never approves its own PR,
marks it ready for review, enables auto-merge, merges it, deletes it, or
rewrites it after maintainers have started reviewing.

## Release PR Merge

This repository also uses `.github/workflows/automerge-release-please.yml` to
merge `release-please` PRs after an explicit approval label has been applied by
an authorized maintainer.

- The approval label is configured by the `RELEASE_READY_LABEL` Actions
  variable, which defaults to `release: ready`; references to `release: ready`
  below describe that current/default value
- Only active members of `@orang-gaboets/octostate-publishers` may apply the configured approval label to approve a release
- The workflow only targets same-repository PRs into `main`
- The workflow only targets release branches created by `release-please` (`release-please--branches--*`)
- The workflow verifies the PR author is the configured GitHub App bot
- The workflow verifies the label actor against the `octostate-publishers` team
- Unauthorized configured approval labels are removed automatically
- Unauthorized approval attempts leave a PR comment from the release-please app bot
- If `release-please` updates the PR head after approval, the stale configured approval label is removed and must be re-applied
- The workflow requires both the configured approval label and `autorelease: pending` before merging
- The workflow fails closed if either required label is absent
- The workflow revalidates the live PR state immediately before merging; confirmed missing lifecycle state invalidates approval, while unreadable state fails closed without removing approval
- Apply the configured approval label additively; do not replace existing labels or remove Release Please lifecycle labels
- Release Please owns the normal transition from `autorelease: pending` to its post-publication lifecycle state
- The workflow waits for the `CI`, `CodeQL Security`, and `Go vulnerability monitoring` release checks to complete before merging; see [Code scanning and Code Quality](code-scanning.md) for the ownership and identity guide
- The workflow uses the configured GitHub App token to merge the release PR directly
- Human-authored PRs are intentionally ignored

The configured release approval label belongs to this repository's approval
gate. The `autorelease:*` labels belong to `release-please` and should not be
manually edited during normal release flow:

- `autorelease: pending`
- `autorelease: tagged`
- `autorelease: snapshot`
- `autorelease: published`
- `autorelease: triggered`

Only edit `autorelease:*` labels, close generated release PRs, or delete
`release-please--branches--*` branches during an intentional release-state
repair. In normal operation, release-please manages those labels and generated
PR branches itself.

If the approval label ever changes, set the repository Actions variable
`RELEASE_READY_LABEL` to the new label name so runtime checks and workflow
concurrency cancellation use the same configured value.

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

If the configured release approval label is applied by someone who is not in
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
the configured approval label, so maintainers can retry after fixing the setup.

## Release State Repair

If a release PR merges but the expected tag or GitHub Release is missing, repair
the release state before merging another release PR:

- Confirm `CHANGELOG.md` and `.release-please-manifest.json` already contain the
  intended version
- Create the missing GitHub Release and tag at the merged release PR commit,
  using the matching changelog section as release notes
- Remove or update any temporary `last-release-sha` override so future
  release-please runs use the latest valid release baseline
- Remove stale release-please lifecycle labels such as `autorelease: pending`
  and `autorelease: triggered` from the stale generated release PR
- Close any generated release PR that was created from stale release state
- Delete the generated `release-please--branches--*` branch after closing the
  stale release PR
- Re-run the `Release Please` workflow manually and confirm it does not recreate
  the stale release PR
