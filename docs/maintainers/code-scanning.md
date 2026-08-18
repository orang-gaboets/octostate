# Code Scanning and Code Quality

This guide is the maintainer source of truth for the repository-managed CodeQL
security scan and GitHub-managed Code Quality analysis. The two paths are
intentionally retained because they serve different purposes:

- Repository-managed CodeQL reports security analysis results through code
  scanning.
- GitHub-managed Code Quality reports Code Quality findings for pull requests
  and the default branch.

Neither path should be removed or weakened just to eliminate similar-looking
checks.

## Verified configuration snapshot

The following state was verified on 2026-08-18 against `main` at commit
`24afc95`, before the workflow display-name change in this branch:

- `.github/workflows/codeql.yml` was active with workflow name `CodeQL` and job
  name `Analyze Go`.
- GitHub's generated workflow `dynamic/github-code-scanning/codeql` was active
  and produced `Code Quality: PR #...`, `Code Quality: Push on main`, and
  `Code Quality: Scheduled` runs with job name `Analyze (go)`.
- The CodeQL default-setup API reported `state: not-configured`; this is not a
  separate active default-setup scan for the repository.
- Repository-managed CodeQL analyses were present, and the observed analyses
  reported no current code-scanning alerts.
- Classic branch protection was not configured. The active repository
  `main-protection` ruleset required pull-request review but no status checks.
- Organization-level rules were not accessible with the verification token's
  current permissions. An organization administrator must re-check them before
  approving a future status-check identity change.

This branch changes only the repository-managed workflow display name to
`CodeQL Security`. Its job remains `Analyze Go`; the generated Code Quality job
remains `Analyze (go)`.

The earlier decision in #110 said to leave GitHub Code Quality findings
disabled for the time being. Current GitHub-managed Code Quality runs show that
the external configuration has since changed. The intended configuration is
now explicit: keep GitHub Code Quality alongside repository-managed security
scanning. This repository change does not modify GitHub settings.

## Active analysis paths

| Path and owner | Triggers | Workflow/job or run identity | Purpose and results | Configuration control |
|---|---|---|---|---|
| [`.github/workflows/codeql.yml`](../../.github/workflows/codeql.yml) — repository-managed | Pushes and pull requests targeting `main`; weekly schedule at `24 3 * * 1` | Workflow `CodeQL Security`; job/check `Analyze Go` | Advanced CodeQL for Go with a manual `go build ./...`; uploads security code-scanning results | The tracked workflow file and its pinned action versions |
| `dynamic/github-code-scanning/codeql` — GitHub-managed | Pull requests, pushes to the default branch, and scheduled runs, as observed in recent runs | Workflow metadata may say `CodeQL`; run labels are `Code Quality: PR #...`, `Code Quality: Push on main`, or `Code Quality: Scheduled`; job `Analyze (go)` | CodeQL-powered Code Quality analysis; findings appear in the repository's Code Quality/security views and pull-request annotations | GitHub repository Code Quality / Advanced Security settings; no generated workflow file is tracked here |
| CodeQL default setup — GitHub setting | Not active in the verified snapshot | API state `not-configured` | No separate default-setup execution path was identified | Repository Advanced Security settings and the default-setup API |

GitHub documents that Code Quality and code scanning can both use the workflow
name `CodeQL`, so the workflow name alone is not a reliable discriminator. Use
the Actions run label, job/check name, workflow source/path, and analysis result
type together when classifying a run. See [CodeQL-powered analysis for Code
Quality](https://docs.github.com/en/code-security/reference/code-quality/codeql-detection).

## Settings and identity checks

Before changing a workflow, job, or check identity, inspect all consumers:

1. Repository rulesets, including required status checks.
2. Classic branch protection, if configured.
3. Organization-level rulesets and rules where the operator has access.
4. Release automation and any `gh pr checks` workflow filters.
5. Scripts, documentation, and external control-repository references.

The release auto-merge workflow relies on the repository workflow identity
`CodeQL Security`. It must continue to require that workflow together with `CI`
and `Go vulnerability monitoring`.

When default setup is enabled or changed, inspect whether GitHub has disabled
or overridden an existing advanced CodeQL workflow. See [Configuring default
setup for code scanning](https://docs.github.com/en/code-security/how-tos/find-and-fix-code-vulnerabilities/configure-code-scanning/configure-code-scanning)
and [Troubleshooting two CodeQL
workflows](https://docs.github.com/en/code-security/reference/code-scanning/troubleshoot-analysis-errors/two-codeql-workflows).

## Troubleshooting

Start with the failing check's details page and identify its execution source
before editing repository configuration.

### Repository-managed CodeQL failure

Classify a failure as repository-managed when it comes from
`.github/workflows/codeql.yml`, is labelled `CodeQL Security`, or exposes the
`Analyze Go` job. Inspect the workflow revision, action versions, Go setup,
manual build, permissions, and CodeQL upload step.

Do not rename this job to `Analyze (go)`: that is the GitHub-managed Code
Quality job identity and would recreate the ambiguity tracked by #230.

### GitHub-managed Code Quality failure

Classify a failure as GitHub-managed when the run label is `Code Quality: ...`,
the job is `Analyze (go)`, or the workflow source is under
`dynamic/github-code-scanning/codeql`. There is no repository workflow file to
edit for this path. Check the repository's Code Quality/Advanced Security
settings and the generated run details first.

### Transient GitHub service failure

If the log reports a GitHub infrastructure or service error during generated
workflow initialization, retry or observe the next run before changing a
repository workflow. A one-off service failure is not evidence that
`.github/workflows/codeql.yml` needs a new option, a renamed job, or reduced
security coverage. Propose a repository change only after a reproducible
repository-side failure has been established and its execution source has been
confirmed.

## Revalidation commands

Run these read-only checks when GitHub settings or workflow identities change:

```bash
gh api repos/orang-gaboets/octostate/code-scanning/default-setup
gh api 'repos/orang-gaboets/octostate/actions/workflows?per_page=100'
gh api 'repos/orang-gaboets/octostate/rulesets?includes_parents=true'
gh api repos/orang-gaboets/octostate/branches/main/protection
gh api 'orgs/orang-gaboets/rulesets?includes_parents=true'
gh pr checks <release-pr> --json name,workflow,bucket
```

The organization-ruleset query may require `admin:org`; record that limitation
instead of assuming that no organization-level rule exists. Confirm the Code
Quality/default-setup setting in the repository's GitHub Advanced Security UI
when the API does not expose the setting.
