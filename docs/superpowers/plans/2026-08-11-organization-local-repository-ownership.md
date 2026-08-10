# Organization-Local Repository Ownership Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enforce organization-local ownership for managed repositories and team repository permissions across validation, GitOps APIs, CLI commands, proposals, and sync-from-live while preserving external template references.

**Architecture:** `config.Validate` remains the single semantic authority. A typed `*config.ValidationError` carries its complete report through exported GitOps entry points, while one small owner-comparison helper is shared by semantic validation and the `--repo-org` CLI guard. Existing command-level validation and sync-generated-config validation remain in place as early safety boundaries.

**Tech Stack:** Go, Cobra, YAML, existing GitHub service interfaces, GitOps packages, and the repository's current test seams.

## Global Constraints

- Keep #208 as one cross-cutting issue; organize implementation into subtasks and commits without splitting the issue.
- Base implementation on the current `fix/organization-local-repository-ownership` branch, which is based on PR #209.
- Use `repository_owner_scope` for both `repositories[i].owner` and `teams[i].repositories[j].owner` violations.
- Match owners after trimming and case normalization; omitted managed owners continue to default to `organization`.
- Suppress owner-scope diagnostics when `organization` is missing or blank; add no organization-name format validation.
- External `template.owner` references remain valid.
- Same-organization team permission targets need not appear in top-level `repositories`.
- Explicit mismatched managed owners are never silently rewritten or omitted.
- `config.Validate` remains authoritative; do not duplicate owner-matching logic in plan, diff, apply, syncfromlive, or proposal packages.
- `plan.Build`, `diff.Build`, `apply.Check`, `apply.Execute`, `syncfromlive.BuildAdoptConfig`, and `syncfromlive.BuildMaterializeConfig` reject invalid desired configuration.
- Preserve `--repo-org`: omitted values default to `--org`; explicit normalized same-org values work; cross-org values fail before auth, GitHub calls, proposal mutation, or success/dry-run output.
- Keep sync-from-live generated-config validation before encoding, stdout output, replacement, or writing.
- Update `docs/gitops/config-schema.md` with the ownership boundary and migration guidance.
- Call out the compatibility-affecting validation change in PR/release notes without prescribing a `BREAKING CHANGE:` footer or major-version bump.
- Run formatting, `go test ./...`, `go vet ./...`, and `golangci-lint run` before completion.

---

## File map

- `pkg/gitops/config/`: ownership rule, typed validation error, direct validation tests, and `EncodeYAML` regression coverage.
- `pkg/gitops/{plan,diff,apply,syncfromlive}/`: exported package-boundary validation and in-memory API tests.
- `cmd/octostate/team/repo/permissions/`: `--repo-org` guard and CLI behavior tests.
- `cmd/octostate/config/`, `cmd/octostate/audit/`, and `cmd/octostate/internal/configproposal/`: command, proposal, and sync acceptance coverage.
- `docs/gitops/config-schema.md` and `docs/cli/primitives.md`: user-facing ownership and compatibility documentation.

## Shared interfaces

Add the smallest shared config API needed by both validation and CLI parsing:

```go
const ValidationIssueCodeRepositoryOwnerScope ValidationIssueCode = "repository_owner_scope"

type ValidationError struct {
	Report ValidationReport
}

func (e *ValidationError) Error() string
func ValidateAndError(cfg OrganizationConfig) error
func RepositoryOwnerMatchesOrganization(owner, organization string) bool
```

`RepositoryOwnerMatchesOrganization` trims both arguments, compares them case-insensitively, and returns false when either value is blank. Callers apply owner fallback before comparing omitted managed owners.

`ValidateAndError` returns `nil` for a valid config and `*ValidationError` for an invalid config. `ValidationError.Error()` is the sole deterministic formatter for validation issues; proposal code must reuse it rather than retain a second formatter.

## Task 1: Centralize ownership validation and validation-error formatting

**Files:**

- Modify: `pkg/gitops/config/report.go`
- Modify: `pkg/gitops/config/validate.go`
- Create: `pkg/gitops/config/validation_error.go`
- Test: `pkg/gitops/config/validate_test.go`
- Test: `pkg/gitops/config/encode_test.go`

**Interfaces:**

- Consumes: existing `OrganizationConfig`, `ValidationReport`, and validation helpers.
- Produces: `ValidationIssueCodeRepositoryOwnerScope`, `ValidationError`, `ValidateAndError`, and `RepositoryOwnerMatchesOrganization` for later tasks.

- [ ] **Step 1: Add failing direct in-memory validation tests.** Construct configs without `LoadFile` normalization using trimmed/case-varied organizations and owners. Assert valid omitted/same-org cases, both required invalid paths, deterministic multiple violations, blank-organization suppression, external template exemption, and same-org team targets absent from top-level repositories.

- [ ] **Step 2: Run the focused validation tests.**

  Run: `go test ./pkg/gitops/config -run 'TestValidate|TestEncode'`

  Expected: the new ownership assertions fail because no ownership issue code/helper exists.

- [ ] **Step 3: Add the shared comparison helper and validation issue.** Default empty managed owners to the trimmed organization, then emit `repository_owner_scope` only when the organization is nonblank and the explicit/effective owner does not match. Keep template validation unchanged.

- [ ] **Step 4: Add the typed error using one formatter.** Implement `ValidateAndError` by calling `Validate`; implement `ValidationError.Error()` by formatting all report errors with path, code, and message in report order. Do not add a second formatter elsewhere.

- [ ] **Step 5: Add the direct encoder regression test.** Call `EncodeYAML` with explicit mismatched top-level and team owners and assert the emitted YAML still contains those owners. Do not change `encode.go` behavior.

- [ ] **Step 6: Run the focused tests.**

  Run: `go test ./pkg/gitops/config -run 'TestValidate|TestEncode'`

  Expected: PASS, including direct unnormalized values and the no-rewrite encoder contract.

- [ ] **Step 7: Commit the self-contained config change.**

  Run:

  ```bash
  git add pkg/gitops/config/report.go pkg/gitops/config/validate.go pkg/gitops/config/validation_error.go pkg/gitops/config/validate_test.go pkg/gitops/config/encode_test.go
  git commit -m "fix(config): enforce organization-local repository ownership"
  ```

## Task 2: Enforce semantic validation at exported GitOps boundaries

**Files:**

- Modify: `pkg/gitops/plan/build.go`
- Modify: `pkg/gitops/diff/build.go`
- Modify: `pkg/gitops/apply/execute.go`
- Modify: `pkg/gitops/syncfromlive/adopt.go`
- Modify: `pkg/gitops/syncfromlive/materialize.go`
- Test: `pkg/gitops/plan/build_test.go`
- Test: `pkg/gitops/diff/build_test.go`
- Test: `pkg/gitops/apply/check_test.go`
- Test: `pkg/gitops/apply/execute_test.go`
- Test: `pkg/gitops/syncfromlive/adopt_test.go`
- Test: `pkg/gitops/syncfromlive/materialize_test.go`

**Interfaces:**

- Consumes: `config.ValidateAndError` and `config.ValidationError` from Task 1.
- Produces: package-level rejection behavior for all six named exported entry points.

- [ ] **Step 1: Add failing `errors.As` tests for invalid in-memory desired configs.** Cover `plan.Build`, `diff.Build`, `apply.Check`, `apply.Execute`, `BuildAdoptConfig`, and `BuildMaterializeConfig`. Supply otherwise valid actual/snapshot/plan inputs where needed and assert the returned error is `*config.ValidationError` with the expected owner path/code.

- [ ] **Step 2: Run the focused GitOps tests.**

  Run: `go test ./pkg/gitops/plan ./pkg/gitops/diff ./pkg/gitops/apply ./pkg/gitops/syncfromlive`

  Expected: the new boundary tests fail because package validators do not yet call the semantic validator.

- [ ] **Step 3: Call `config.ValidateAndError` from each package's existing `Options.Validate`.** Keep current missing-input and organization-consistency checks. Perform desired-config validation before executor construction, service use, plan execution, or sync desired-state processing. Add no owner logic to these packages.

- [ ] **Step 4: Run focused tests and confirm valid behavior remains unchanged.**

  Run: `go test ./pkg/gitops/plan ./pkg/gitops/diff ./pkg/gitops/apply ./pkg/gitops/syncfromlive`

  Expected: PASS, including existing valid configs and new typed-error tests.

- [ ] **Step 5: Commit the package-boundary change.**

  Run:

  ```bash
  git add pkg/gitops/plan/build.go pkg/gitops/diff/build.go pkg/gitops/apply/execute.go pkg/gitops/syncfromlive/adopt.go pkg/gitops/syncfromlive/materialize.go pkg/gitops/plan/build_test.go pkg/gitops/diff/build_test.go pkg/gitops/apply/check_test.go pkg/gitops/apply/execute_test.go pkg/gitops/syncfromlive/adopt_test.go pkg/gitops/syncfromlive/materialize_test.go
  git commit -m "fix(gitops): validate desired config at package boundaries"
  ```

## Task 3: Enforce the existing `--repo-org` compatibility contract

**Files:**

- Modify: `cmd/octostate/team/repo/permissions/add.go`
- Modify: `cmd/octostate/team/repo/permissions/remove.go`
- Test: `cmd/octostate/team/repo/permissions/add_test.go`
- Test: `cmd/octostate/team/repo/permissions/remove_test.go`
- Modify: `docs/cli/primitives.md`

**Interfaces:**

- Consumes: `config.RepositoryOwnerMatchesOrganization` from Task 1.
- Produces: normalized same-org acceptance and pre-side-effect cross-org rejection for add/remove live, dry-run, and proposal paths.

- [ ] **Step 1: Add failing table-driven CLI tests.** Cover omitted `--repo-org`, explicit same-org, whitespace/case-equivalent same-org, and cross-org values for both add and remove. For cross-org cases, use a nil service/no credentials, dry-run, and `--to-config` fixtures; assert the owner error appears before auth/service/proposal work and stdout remains empty.

- [ ] **Step 2: Run focused permission tests.**

  Run: `go test ./cmd/octostate/team/repo/permissions`

  Expected: cross-org cases currently produce dry-run/success/auth/proposal behavior instead of the owner rejection.

- [ ] **Step 3: Add the normalized owner guard immediately after `--repo-org` fallback.** Use the shared helper before repository-name/permission side effects, dry-run output, proposal mutation, client construction, and GitHub calls. Keep the flag and all same-org compatibility behavior unchanged.

- [ ] **Step 4: Document the guard in `docs/cli/primitives.md`.** State fallback, trim/case normalization, same-org acceptance, and pre-side-effect cross-org rejection without describing removal or deprecation.

- [ ] **Step 5: Run focused tests.**

  Run: `go test ./cmd/octostate/team/repo/permissions`

  Expected: PASS with no cross-org operation output or side effect.

- [ ] **Step 6: Commit the CLI change.**

  Run:

  ```bash
  git add cmd/octostate/team/repo/permissions/add.go cmd/octostate/team/repo/permissions/remove.go cmd/octostate/team/repo/permissions/add_test.go cmd/octostate/team/repo/permissions/remove_test.go docs/cli/primitives.md
  git commit -m "fix(team): enforce repo owner organization boundary"
  ```

## Task 4: Add command, proposal, and sync acceptance coverage

**Files:**

- Modify: `cmd/octostate/internal/configproposal/configproposal.go`
- Test: `cmd/octostate/internal/configproposal/configproposal_test.go`
- Test: `cmd/octostate/config/validate_test.go`
- Test: `cmd/octostate/config/plan_test.go`
- Test: `cmd/octostate/config/apply_test.go`
- Test: `cmd/octostate/audit/diff_test.go`
- Test: `cmd/octostate/config/sync_from_live_test.go`

**Interfaces:**

- Consumes: `config.ValidateAndError` and its single deterministic formatter from Task 1.
- Produces: explicit command-level and proposal-level rejection coverage, including sync no-output/no-write behavior.

- [ ] **Step 1: Replace proposal-local validation formatting.** In `ApplyToConfigFile`, call `config.ValidateAndError` before mutation and after mutation, wrap the returned typed error with the existing phase context, and delete `formatValidationIssues`. Preserve validation-before-mutation, validation-before-replacement, and unchanged-on-error behavior.

- [ ] **Step 2: Add failing proposal tests.** Cover invalid loaded ownership and mutation-introduced ownership, assert `repository_owner_scope` details, and verify mutation/replacement/bytes remain unchanged. Keep existing validation cases passing through the shared formatter.

- [ ] **Step 3: Add named command integration tests.** Exercise invalid loaded ownership through `config validate`, `config plan`, `config apply`, `config apply --check`, and `audit diff`. Assert the exact path/code and, where applicable, that client construction, snapshot reading, GitHub calls, and successful output do not occur.

- [ ] **Step 4: Run proposal and command tests.**

  Run: `go test ./cmd/octostate/internal/configproposal ./cmd/octostate/config ./cmd/octostate/audit`

  Expected: new tests fail until the proposal path uses the shared formatter and fixtures contain explicit mismatched managed owners.

- [ ] **Step 5: Add one table-driven sync regression test over all three modes.** Parameterize bootstrap, adopt, and materialize with generated configs containing a mismatched managed owner. Reuse existing sync command seams and assert for every case: validation stops before encoding, stdout is empty, the target file is unchanged, and the owner is neither omitted nor rewritten. Keep the existing normal semantic validation path; do not add sync-specific owner validation.

  Test shape:

  ```go
  func TestSyncFromLiveRejectsGeneratedOwnershipViolations(t *testing.T) {
      cases := []struct {
          name string
          mode string
      }{
          {name: "bootstrap", mode: syncFromLiveModeBootstrap},
          {name: "adopt", mode: syncFromLiveModeAdopt},
          {name: "materialize", mode: syncFromLiveModeMaterialize},
      }

      for _, tc := range cases {
          t.Run(tc.name, func(t *testing.T) {
              stdout, writeResult, err := runGeneratedOwnershipFailure(t, tc.mode)
              if err == nil {
                  t.Fatal("expected generated ownership validation error")
              }
              if len(stdout) != 0 {
                  t.Fatalf("expected no generated YAML, got %q", stdout)
              }
              if writeResult != nil {
                  t.Fatalf("expected no write result, got %#v", writeResult)
              }
          })
      }
  }
  ```

  Implement `runGeneratedOwnershipFailure` in the same test file by reusing the existing client/collector hooks and selecting the bootstrap, adopt, or materialize builder. Each selected builder must return `organization: org-a` with a managed repository or team-repository owner `org-b`; set the encoder hook to fail the test if called, and compare the target file bytes before and after the command when `--write` is used.

- [ ] **Step 6: Run the focused acceptance suite.**

  Run: `go test ./cmd/octostate/internal/configproposal ./cmd/octostate/config ./cmd/octostate/audit`

  Expected: PASS for all named command paths, proposal failure safety, and the three sync modes.

- [ ] **Step 7: Commit the acceptance coverage and formatter consolidation.**

  Run:

  ```bash
  git add cmd/octostate/internal/configproposal/configproposal.go cmd/octostate/internal/configproposal/configproposal_test.go cmd/octostate/config/validate_test.go cmd/octostate/config/plan_test.go cmd/octostate/config/apply_test.go cmd/octostate/config/sync_from_live_test.go cmd/octostate/audit/diff_test.go
  git commit -m "test(config): cover organization-local ownership boundaries"
  ```

## Task 5: Document the schema boundary and migration

**Files:**

- Modify: `docs/gitops/config-schema.md`
- Update: resulting PR description and release notes

**Interfaces:**

- Consumes: the finalized validation and CLI behavior from Tasks 1–4.
- Produces: migration guidance without changing release classification.

- [ ] **Step 1: Add schema documentation.** Document managed top-level and team repository owners, omitted-owner fallback, trim/case matching, the `repository_owner_scope` rejection, same-org partial team targets, and the distinction between managed owners and external `template.owner` references.

- [ ] **Step 2: Add migration guidance.** Explain how to correct previously accepted explicit cross-org managed owners, how to remove redundant same-org owner fields, and that external template references require no migration. Point users to `config validate`.

- [ ] **Step 3: Add compatibility wording to the PR/release notes.** Call out that previously accepted invalid managed state is now rejected before GitHub or file mutation. Do not add a breaking-commit footer or major-version instruction.

- [ ] **Step 4: Verify documentation.** Check referenced commands and paths against the repository and run the documentation checks from `docs/maintainers/development.md`.

- [ ] **Step 5: Commit the schema documentation.**

  Run:

  ```bash
  git add docs/gitops/config-schema.md
  git commit -m "docs(config): document repository ownership boundary"
  ```

## Final verification and self-review

- [ ] Run `gofmt -w` on changed Go files.
- [ ] Run `go test ./...`.
- [ ] Run `go vet ./...`.
- [ ] Run `golangci-lint run`.
- [ ] Confirm `repository_owner_scope` is used at both required paths and no old owner-scope code is introduced.
- [ ] Confirm all six exported GitOps entry points reject invalid in-memory desired configs with `*config.ValidationError`.
- [ ] Confirm `configproposal` has no second validation formatter or owner-matching rule.
- [ ] Confirm CLI cross-org rejection precedes auth, GitHub calls, proposal mutation, and output.
- [ ] Confirm sync regression coverage uses one table-driven test across bootstrap, adopt, and materialize.
- [ ] Confirm `EncodeYAML` preserves explicit mismatched owners when called directly.
- [ ] Confirm external templates and same-org team-only targets remain valid.
- [ ] Confirm docs and PR/release notes describe compatibility impact without deciding release classification.

## Decision checkpoint

No new decision question is required. The typed `ValidationError` contract and `--repo-org` compatibility decision were already locked, and both ponytail-review changes are mechanical simplifications. Recommendation: use this plan as written, with the shared formatter and table-driven sync test.

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-08-11-organization-local-repository-ownership.md`.

1. **Subagent-Driven (recommended):** dispatch one fresh worker per task and review between commits using `superpowers:subagent-driven-development`.
2. **Inline Execution:** execute the tasks in this session with checkpoints using `superpowers:executing-plans`.
