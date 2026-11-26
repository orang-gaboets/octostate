# Actions Watcher

The Actions Watcher tracks failures from a fixed set of Octostate workflows. It
creates at most one open incident issue per monitored workflow, records every
distinct eligible failed attempt, and updates the incident when a later
successful attempt is observed. It never reruns a workflow, remediates a
failure, or closes or reopens an issue.

The watcher runs as trusted code from `main`. It uses GitHub Actions run and
job metadata only; it does not download artifacts or read workflow logs.

## Maintainer-owned setup

The workflow requires a separate organization-owned GitHub App named
**Orang Gaboets Actions Watcher**. Create and install it only after the
maintainer who owns organization access has approved that setup.

Configure the App with these repository permissions:

| Permission | Access |
| --- | --- |
| Actions | Read |
| Issues | Read and write |
| Metadata | Read |

Do not grant source-code, pull-request, release, organization administration,
or Actions write access. Install the App on `orang-gaboets/octostate` only.
The workflow mints a short-lived installation token scoped again to that owner
and repository. It checks the App slug and verifies that the token can see
exactly the Octostate repository with the expected repository ID before it
plans or writes incidents. Missing credentials, a different App, an expanded
installation, or insufficient API access fail the run; no other credential is
used as a fallback.

Create the protected Actions environment named `actions-watcher` and restrict
its deployment branch policy to `main`. The workflow sets
`deployment: false`; it uses the environment for secret and variable scoping,
not for deployment records. Configure these values in that environment:

| Name | Type | Value |
| --- | --- | --- |
| `ACTIONS_WATCHER_APP_ID` | Variable | The dedicated App's client ID |
| `ACTIONS_WATCHER_PRIVATE_KEY` | Secret | The dedicated App's private key |

The workflow requests an installation token with Actions read, Issues write,
and Metadata read. Its default `GITHUB_TOKEN` remains contents read and is used
only for the trusted checkout. The App is expected to have the slug
`orang-gaboets-actions-watcher`; a mismatch stops the command.

The repository labels below must exist before an eligible incident can be
written. Before any issue mutation, the command reads and validates the label
inventory and fails if a needed label is missing. Maintainers should review
the current label inventory for naming conflicts before provisioning labels:

- `actions-watcher` identifies issues managed by this App.
- `workflow: failing` indicates that the latest relevant observed attempt
  failed.
- `workflow: recovered` indicates that a later relevant attempt succeeded.

Only one of the two status labels is applied at a time. Unrelated labels are
preserved. This repository change does not create the App, configure the
environment, install credentials, or provision these remote labels; those
changes require maintainer authorization.

## Monitored workflows

The allowlist is intentionally explicit and is checked against both the GitHub
workflow name and workflow file path:

| Workflow | File |
| --- | --- |
| CI | `.github/workflows/ci.yml` |
| CodeQL Security | `.github/workflows/codeql.yml` |
| Go vulnerability monitoring | `.github/workflows/govulncheck.yml` |
| Go toolchain remediation | `.github/workflows/go-toolchain-remediation.yml` |
| Release Please | `.github/workflows/release-please.yml` |
| Contributor showcase | `.github/workflows/contributors.yml` |
| Trusted Live Integration | `.github/workflows/live-integration.yml` |

Only completed runs whose original event is `push`, `schedule`, or
`workflow_dispatch`, whose branch is `main`, and whose base and head repository
IDs both match `orang-gaboets/octostate` can affect incidents. Pull request
runs (including PRs targeting `main`), other branches, cancellations, skipped
or neutral results, and unknown conclusions do not change status. Only
`failure`, `timed_out`, and `startup_failure` count as failures; only `success`
can recover an incident. The watcher itself is not in the allowlist.

`automerge-release-please.yml` is excluded because it is a PR-targeted helper.
`config-review.yml` is excluded because it is a reusable `workflow_call`
contract rather than a repository run on `main`. Pull request events are
excluded from the monitored workflows even where their workflow files also
run on PRs. The allowlist does not expand automatically when workflows are
added or renamed; update the workflow, routing check, and this table together
in a reviewed change.

## Reconciliation and incident lifecycle

The watcher runs on each completed allowlisted `workflow_run` whose source
event is `push`, `schedule`, or `workflow_dispatch` on `main`, 15 minutes after
a push to `main`, and hourly. Pull request source runs are filtered before the
reconciliation job starts. Every run performs a full authoritative
reconciliation instead of trusting the triggering event payload for issue
state. For a `workflow_run` trigger, it also re-fetches the event's run ID
directly from GitHub, so a delivered rerun can be reconciled even when its
original run was created before the seven-day lookback. Repository-wide
concurrency serializes active runs without cancelling one that has started.
GitHub may replace an older pending run in the same concurrency group; the
next hourly reconciliation recovers any missed event that remains within the
API lookback.

Before applying a change associated with a delivered run, the watcher records
that run ID in the open incident's managed state. If a comment or issue update
then fails partway through, the next reconciliation fetches the pending run
IDs directly from GitHub, including runs older than the lookback. A successful
update clears those IDs from the managed state.

Pending IDs are fetched in batches of at most 1,001 per run. Open incidents are
processed first, followed by the oldest closed incidents; unselected IDs remain
in their source checkpoint for a later reconciliation.

The command reads all pages of workflow, run, issue, label, and comment data.
It partitions an oversized run query when GitHub's workflow-run endpoint
reports more than its result cap. It reads each attempt separately and reads
attempt-specific jobs. A successful empty job response means job data is
available but contains no failed jobs. A `404` or `410` for attempt jobs is
recorded as unavailable; authorization, rate-limit, and other API errors fail
the reconciliation. GET requests have a bounded retry policy that observes
GitHub retry/reset headers. Mutations are never blindly retried: the command
re-reads the issue or comments after every create/update response and accepts
an ambiguous response only when the desired state is visible.

The run history lookback is seven days, based on GitHub's original run-created
time filter. Attempts that started before the first-parent commit that adds
`.github/workflows/actions-watcher.yml` are ignored. This avoids importing old
incidents at activation. GitHub's created-time filter does not move when an
old run is rerun, so a rerun of a run created more than seven days ago can be
missed by the hourly sweep if its `workflow_run` event is also lost. Event
delivery outages, disabled Actions, API or credential outages, and GitHub
retention can also exceed coverage. The watcher does not claim complete
monitoring beyond the available run history.

Before using an open incident's latest attempt as its ordering cursor, the
watcher fetches that exact run attempt from GitHub and checks the persisted
metadata against the API response. For a failed latest attempt with available
job data, it also checks the persisted failed-job names against the Actions
API. A checkpoint recorded while job data was unavailable remains valid if
GitHub later exposes those jobs; a subsequent scan enriches the managed failure
record when it sees that attempt again. Other missing or mismatched metadata
stops reconciliation rather than trusting editable issue content. If GitHub
has expired the stored attempt, a maintainer should review and manually close
the stale incident; later failures will then start a new linked incident.
Before archiving an unavailable job snapshot, the watcher rechecks the attempt
and adds job names if GitHub now exposes them.

That API verification covers the latest ordering cursor. Failure records kept
in the issue body are structurally checked and must not be later than the
verified cursor, but they are not individually fetched from GitHub on every
hourly run. A repository collaborator who edits those machine markers can
change older body history and its count. Archive comments are App-authored and
checksum-checked. Maintainers should not edit machine markers.
Before moving body-resident records into a new archive comment, the watcher
verifies each run attempt and its available failed-job metadata against the
Actions API. Missing, expired, or mismatched records stop the update. While a
record remains in the editable issue body, a repository collaborator can still
change it and its count; full authentication of all body-resident history needs
a separate durable integrity design because GitHub has no batch lookup for
attempts and old attempts can expire.

The issue title follows the form
`ci(actions): investigate <workflow> workflow failures on main`. The body has
one machine-readable repository/workflow identity marker and a managed region
for **Current status** and **Failure history**. The
**Maintainer investigation** section and all text outside the managed region
remain human-owned. The body records UTC timestamps, every distinct failure's
run URL, SHA, run and attempt number, and available failed job names. Missing
job data is identified as unavailable. Raw logs and sensitive data are never
copied. If the body reaches the size limit, older failure records move into
immutable, marked issue comments. The body links the latest archive comments
and stores a compact digest index for the full archive set; the watcher fails
closed if a referenced or indexed archive is missing.

Recovery changes the body and status label and adds a marked transition
comment. A later failure while the issue remains open returns it to failing and
adds a regression comment. Repeated deliveries and continued successes do not
create duplicate history or unnecessary notifications. A successful run is
evidence of an observed recovery only; it does not establish that the root
cause is fixed or that the same failing path ran.

Maintainers control closure. The watcher never closes or reopens an issue. A
failure attempt that started at or before the latest manual `closed_at` is
treated as delayed or replayed evidence and leaves that issue unchanged. A
distinct failure attempt that started after closure creates a new incident
with a fresh history and a link to the previous closed incident. Manually
reopening a managed issue resumes that issue as the active incident.

## Failure handling and recovery

An ambiguous identity marker, wrong App author, duplicate open incidents,
missing archive evidence, concurrent issue edit, or unexpected source
repository/workflow identity stops the reconciliation before it overwrites
ambiguous state. The command re-reads issue state immediately before writes;
GitHub does not provide a transaction across the body, labels, and comments,
so a human edit between that read and the write can still race. Readback and
the next hourly reconciliation expose and repair safe partial writes without
blindly duplicating comments.

When a run fails, inspect the Actions Watcher run's stderr and the GitHub API
response. Check, in order:

1. The environment is restricted to `main` and has the dedicated App ID and
   private key.
2. The App remains installed only on `orang-gaboets/octostate` with the three
   documented permissions.
3. All three watcher labels exist with the exact spelling above.
4. The allowlisted workflow names and paths still match GitHub's workflow
   inventory.
5. The incident body markers and any archive/transition comments remain
   intact and App-authored.

After correcting a configuration or API problem, the next delayed main push
reconciliation or hourly run retries a fresh read and repairs non-ambiguous
partial state. Do not create duplicate incidents or manually edit machine
markers to bypass a failed invariant. If the watcher refuses to write, resolve
the reported identity or ownership conflict first.

The watcher cannot reliably report its own failure because it does not watch
itself. Maintainers must inspect its Actions run status separately.

## Local checks

Run the focused checks while changing the watcher:

```bash
go test ./cmd/actions-watcher ./internal/actionswatcher
ruby .github/scripts/check-actions-watcher-routing.rb
ruby .github/scripts/check-actions-watcher-routing.test.rb
```

The repository-wide development checks are listed in
[Development](development.md). A local green result does not verify the
organization App, environment branch policy, remote labels, credential scope,
or hosted lifecycle behavior. Controlled hosted verification requires a
separate maintainer-approved setup and probe.
