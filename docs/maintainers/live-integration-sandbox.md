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
#247. Do not modify #243 or #248. Do not add workflow automation, a control
repository, or runtime behavior.

#243 may proceed with its targeted manual live validation after #247's
verification/documentation is complete. #248 remains the home for future
automated live integration testing and is non-blocking for v1.2.0.

The historical `docs/maintainers/v1.0.0-readiness.md` document remains
unchanged.
