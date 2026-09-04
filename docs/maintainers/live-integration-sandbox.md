# Live Integration Sandbox

This document is the operational contract for Octostate's manually operated
live-integration sandbox. It covers the verification and documentation work in
[#247](https://github.com/orang-gaboets/octostate/issues/247). It does not own
the final release-readiness mutation or convergence evidence; those belong to
[#243](https://github.com/orang-gaboets/octostate/issues/243).

## Sandbox boundary

Use the already-established `octostate-test` organization only.

| Resource | Required value |
| --- | --- |
| Organization | `octostate-test` |
| Organization ID | `321418529` |
| Fixture repository | `octostate-test/octostate-fixture-repo` |
| Fixture repository ID | `1347356483` |
| App ID | `4726852` |
| Installation ID | `156749227` |

The production organization `orang-gaboets` must never be a live-test target or
receive mutation authority from sandbox credentials. The historical
`orang-gaboets-test` organization and
`orang-gaboets-test/octostate-control-test` must remain untouched.

Do not create, delete, replace, transfer, or recreate the established
organization or fixture repository. Do not create a control repository for
this sandbox.

## Established fixture baseline

Before any future mutating scenario, verify that
`octostate-test/octostate-fixture-repo` has this baseline:

- visibility: `public`;
- default branch: `main`;
- archived: `false`;
- `is_template`: `false`;
- topics: `[]`; and
- description: `Persistent fixture for Octostate live integration testing.`

Keep future fixture expansion demand-driven. Do not create a broad resource
matrix pre-emptively.

The final deliberate reversible mutation, convergence check, restoration, and
exact v1.2.0 release-candidate evidence are owned by #243. #247 must not run
that mutation or represent its smoke checks as release-readiness evidence.

## Dedicated manual authentication

The dedicated GitHub App is owned by `octostate-test` and has this installation
boundary:

- installability is restricted to **Only on this account**;
- it is installed on `octostate-test`;
- repository access is restricted to `octostate-fixture-repo`; and
- it is not installed on production `orang-gaboets`.

Required permissions are:

- Organization: Members read;
- Repository: Administration read/write; and
- Repository: Metadata read.

The private key is secret. Never request, expose, print, commit, or record the
private key, tokens, credential contents, or sensitive authentication output.
Use an externally supplied key path when running the commands below.

If App settings are not available through a read-only interface, an authorized
maintainer must verify them through the appropriate GitHub settings surface.
Do not infer installability, installation scope, repository selection, or
permissions from a successful CLI authentication check.

## Future mutation and restoration contract

#247's verification and setup/authentication smoke checks remain non-mutating.
Any later mutation must be separately authorized, target only a designated test
resource, and prefer one small, reversible repository-metadata change. It must
not require repository deletion, visibility changes, archive/unarchive,
template conversion, real-user invitations, member mutations, or unrelated
organization-setting changes.

The final deliberate mutation, convergence check, restoration, and exact
v1.2.0 release-candidate evidence remain owned by #243. An authorized future
mutation follows this sequence:

1. verify the documented baseline;
2. record the original state;
3. perform the controlled mutation;
4. verify the changed state and convergence;
5. restore the original state; and
6. verify restoration and convergence.

### Dirty-sandbox recovery

This recovery procedure applies only when an authorized mutating operation has
started and its resulting state or restoration cannot be verified:

1. stop further mutating tests;
2. preserve enough non-secret evidence to identify the observed state;
3. do not silently reset or continue; and
4. require a trusted maintainer to restore and independently re-verify the
   documented baseline.

## Verification procedure

Verify the established state in this order, using read-only operations:

1. Confirm the organization login and immutable organization ID.
2. Verify organization ownership and ordinary administrative access are limited
   to trusted Octostate maintainers, and record a sanitized result.
3. Confirm the fixture repository name, owner, immutable repository ID, default
   branch, visibility, archive/template flags, topics, and description.
4. Confirm the App owner, App ID, installation ID, installability boundary,
   installation account, selected repository, permissions, and absence from
   production.
5. Confirm that the historical organization and control repository are not
   targets of the work.
6. Confirm that #243 remains the owner of final live mutation,
   convergence/restoration, and exact release-candidate evidence.
7. Confirm that #248 remains future automated live integration work and does
   not block v1.2.0.
8. Run the non-mutating setup/authentication smoke checks below.

### Mismatch handling

#247 is verification/documentation-only. If any organization, fixture, App,
installation, ownership/access, or baseline value does not match, stop before
any write and record the sanitized expected value, observed value, source, and
timestamp. Report the mismatch for separate authorization and scope; do not
repair it from this issue and do not perform a proactive write-capability test.

Do not correct identity, ownership, default branch, refs, branch content,
organization membership, invitations, teams, production or legacy resources,
repository content, branches, pull requests, or the final reversible test
state. Never run bare `config apply` for #247.

Timeouts, network interruptions, partial or ambiguous results, or lost
authentication during read-only verification do not by themselves dirty the
sandbox. They block completion until the state can be read and verified
reliably. If an authorized mutation has started, use the dirty-sandbox recovery
procedure above when its resulting state or restoration is uncertain.

## Non-mutating setup and authentication smoke checks

Use a fresh temporary configuration directory. The configuration declares the
fixture only and intentionally leaves organization members, invitations, and
teams undeclared. Save it as
`$OCTOSTATE_TEST_CONFIG_DIR/organization.yaml`:

```yaml
organization: octostate-test
members: []
invites: []
repositories:
  - name: octostate-fixture-repo
    visibility: public
    description: Persistent fixture for Octostate live integration testing.
    topics: []
    archived: false
    is_template: false
teams: []
```

Run:

```bash
go run ./cmd/octostate config validate --config-dir "$OCTOSTATE_TEST_CONFIG_DIR"

go run ./cmd/octostate config plan \
  --config-dir "$OCTOSTATE_TEST_CONFIG_DIR" \
  --app-id 4726852 \
  --installation-id 156749227 \
  --app-key-path "$OCTOSTATE_TEST_APP_KEY_PATH"

go run ./cmd/octostate config apply \
  --config-dir "$OCTOSTATE_TEST_CONFIG_DIR" \
  --check \
  --app-id 4726852 \
  --installation-id 156749227 \
  --app-key-path "$OCTOSTATE_TEST_APP_KEY_PATH"
```

Expected results:

- `config validate` exits `0`;
- `config plan` reports top-level `organization` `octostate-test`,
  `plan_summary.executable_actions == 0`, and
  `executable_actions == []`; and
- `config apply --check` reports `data.organization == "octostate-test"`,
  `data.plan_summary.executable_actions == 0`, and
  `data.checked_actions == []`.

Expected non-executable `skipped_actions` for intentionally undeclared
members, invitations, teams, memberships, or permissions are acceptable.
`config plan` and `config apply --check` are setup/authentication smoke
evidence only. `config apply --check` is best-effort preflight, not a
transactional dry-run, and neither command authorizes a later write.

An identity or baseline mismatch, unexpected executable action, fixture
creation/update proposal, authentication or collection failure, or failed
verification blocks completion.

## Evidence and recovery

Record a compact, sanitized evidence entry containing:

- verification date and operator;
- observation time and source for each remote fact;
- authentication type: dedicated GitHub App;
- organization login and immutable ID;
- fixture name and immutable ID;
- expected and observed baseline;
- sanitized organization ownership/access verification result;
- App ID, installation ID, owner, installability, account, repository scope,
  and permissions;
- command names, exit codes, and relevant output fields;
- whether a mismatch was found and reported; and
- complete verification of the documented baseline.

When no mismatch exists, explicitly record `no remote write performed`.

## Ownership and release treatment

This document and its index link are the only repository changes owned by
#247. For #247, do not modify #243 or #248, add workflow automation, introduce
a control repository, or change runtime behavior.

#243 may proceed with its targeted manual live validation after #247's
verification/documentation is complete. #248 remains the home for future
automated live integration testing and is non-blocking for v1.2.0.

The historical `docs/maintainers/v1.0.0-readiness.md` document remains
unchanged.

## Automated live integration workflow (#248)

Issue [#248](https://github.com/orang-gaboets/octostate/issues/248) adds
optional reusable live-integration evidence. It does not replace offline CI,
does not block v1.2.0, and never substitutes for #243-owned exact-candidate
description mutation, convergence, restoration, or release-readiness
evidence.

### Activation and authorization

Run `Trusted Live Integration` manually from the Actions UI with the `main`
branch selected, or use:

```bash
gh workflow run live-integration.yml --ref main
```

The workflow has no pull-request, push, or scheduled trigger and accepts no
branch, SHA, organization, or repository inputs. The trust job runs for a
manual dispatch but fails closed unless `github.ref` is `refs/heads/main`; it
checks out the dispatch SHA, fetches `origin/main`, verifies that SHA is an
ancestor, and validates the committed fixture before the protected job can
run.

The `live-integration` job references the protected environment
`octostate-test`. Maintainers must configure and read back these external
settings before activation; the repository workflow does not create or change
them:

- require the `@orang-gaboets/octostate-live-testers` reviewer team where that
  environment feature is supported;
- allow self-review initially if that is the approved operating choice; and
- restrict the environment to the `main` branch.

The environment variable `OCTOSTATE_TEST_APP_CLIENT_ID` and environment
secret `OCTOSTATE_TEST_APP_PRIVATE_KEY` are required. The App private key is
referenced only by the protected job, never by `trusted-dispatch` and never by
ordinary pull-request workflows. The dedicated App must remain installed only
on `octostate-test`, restricted to `octostate-fixture-repo`, with Members read,
Administration read/write, and Metadata read permissions. The documented
non-secret App facts remain App ID `4726852` and installation ID `156749227`.

### Run phases and credential sequence

The protected job checks out the SHA emitted by the trust job and verifies the
checkout before using credentials. It then performs these steps in order:

The implementation intentionally keeps the read-only scenarios in this
protected job rather than running them before environment approval. This keeps
the App private key unavailable until approval, including for the read-only
token; the trust job only validates trusted code and the committed fixture.

1. Mint a short-lived installation token scoped to owner `octostate-test` and
   repository `octostate-fixture-repo`, with Administration read, Members
   read, and Metadata read. Run
   `.github/scripts/live-integration.sh --read-only` for config validate,
   config plan, config apply `--check`, audit pull, and audit diff.
2. Revoke the read-only installation token through GitHub and clear its
   variables from the step environment. No write-scoped token exists during
   the trust job or before this read-only phase completes.
3. Mint a new short-lived token with Administration write, Members read, and
   Metadata read, using the same owner and repository scope. Invoke
   `.github/scripts/live-integration.sh --mutate` exactly once.

The harness targets only `octostate-test` (ID `321418529`) and
`octostate-test/octostate-fixture-repo` (ID `1347356483`). It requires the
canonical baseline before mutation, and the only deliberate mutation is the
reversible topic `octostate-live-integration`.

The mutation path records a normalized stable repository projection containing
only `id`, `owner.login`, `name`, `visibility`, `default_branch`,
`description`, `archived`, `is_template`, and sorted `topics`. It validates the
mutated plan and apply-check as exactly one repository update whose only field
change is `topics`, marks the mutation started immediately before the single
write, then requires the post-apply success envelope and matching executed
action. It polls only bounded read observations and requires both the exact
changed-topic projection and a fresh zero-executable plan. Restoration first
requires the exact expected post-mutation projection (or accepts an exact
baseline no-op), applies the committed baseline once, verifies convergence,
and runs the final read-only audit checks.

Expected non-executable drift for undeclared organization members, invites, or
teams is allowed only when the action is explicitly non-executable and its
resource type is `organization_member`, `invite`, or `team`. Any executable
action, unrelated skipped action, malformed result envelope, identity mismatch,
ambiguous state, or restoration failure fails closed. Exact action guards cover
the plan, apply-check, apply, and restoration envelopes.

Runs are serialized by the literal concurrency group
`octostate-test-live-integration` and `cancel-in-progress: false`. The trust and
protected jobs have 10-minute and 45-minute timeouts respectively, and their
checkout, setup, validation, token, and live-operation steps have explicit
timeouts. The mutation step is limited to 30 minutes, with a 20-minute active
operation budget and a separate 9-minute restoration deadline. The harness also
uses bounded convergence polling.
The step summary is compact and sanitized: it records run ID, tested SHA,
fixed target names/IDs, phase results, expected topic, exact-action-guard
result, convergence/restoration results, final PASS/FAIL, and recovery
guidance. It never records tokens, private keys, raw API payloads, or secret
values.

Normal failures enter guarded one-shot restoration. A forced cancellation or
SIGKILL cannot be trapped; if restoration is FAIL or unknown, stop all further
live automation and follow the dirty-sandbox recovery procedure above. A
trusted maintainer must restore and independently re-verify the documented
baseline before another run.
