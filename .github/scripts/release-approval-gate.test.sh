#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
TEST_ROOT="$(mktemp -d)"
trap 'rm -rf "$TEST_ROOT"' EXIT

mkdir -p "$TEST_ROOT/bin"
PR_FIXTURE="$TEST_ROOT/pr.json"
GH_LOG="$TEST_ROOT/gh.log"
GITHUB_OUTPUT="$TEST_ROOT/output"

cat > "$TEST_ROOT/bin/gh" <<'EOF'
#!/usr/bin/env bash

set -euo pipefail

case "${1:-}" in
  api)
    case "${2:-}" in
      repos/*/pulls/*)
        if [ "${GH_STUB_MODE:-}" = "read-failure" ]; then
          echo "gh: simulated pull request read failure" >&2
          exit 1
        fi
        if [ "${GH_STUB_MODE:-}" = "invalid-json" ]; then
          echo "not-json"
          exit 0
        fi
        cat "$GH_STUB_PR_JSON"
        ;;
      /orgs/*/teams/*/memberships/*)
        if [ "${GH_STUB_MODE:-}" = "membership-failure" ]; then
          echo "gh: simulated membership read failure (HTTP 500)" >&2
          exit 1
        fi
        if [ "${GH_STUB_MODE:-}" = "unauthorized" ]; then
          echo "gh: membership not found (HTTP 404)" >&2
          exit 1
        fi
        echo active
        ;;
      /orgs/*/teams/*)
        if [ "${GH_STUB_MODE:-}" = "team-failure" ]; then
          echo "gh: simulated team read failure (HTTP 500)" >&2
          exit 1
        fi
        echo octostate-publishers
        ;;
      *)
        echo "unexpected gh api target: ${2:-}" >&2
        exit 99
        ;;
    esac
    ;;
  pr)
    case "${2:-}" in
      edit)
        printf '%s\n' "$*" >> "$GH_STUB_LOG"
        if [ "${GH_STUB_REMOVE_MODE:-}" = "fail" ]; then
          echo "gh: simulated label removal failure" >&2
          exit 1
        fi
        ;;
      comment)
        printf '%s\n' "$*" >> "$GH_STUB_LOG"
        ;;
      merge)
        printf '%s\n' "$*" >> "$GH_STUB_LOG"
        if [ "${GH_STUB_MERGE_MODE:-}" = "fail" ]; then
          echo "gh: simulated merge failure" >&2
          exit 1
        fi
        ;;
      *)
        echo "unexpected gh pr operation: ${2:-}" >&2
        exit 99
        ;;
    esac
    ;;
  *)
    echo "unexpected gh command: ${*:-}" >&2
    exit 99
    ;;
esac
EOF
chmod +x "$TEST_ROOT/bin/gh"

export PATH="$TEST_ROOT/bin:$PATH"
export GH_STUB_PR_JSON="$PR_FIXTURE"
export GH_STUB_LOG="$GH_LOG"
export GITHUB_OUTPUT
export EXPECTED_REPOSITORY="orang-gaboets/octostate"
export EXPECTED_BASE_BRANCH="main"
export EXPECTED_HEAD_BRANCH_PREFIX="release-please--branches--"
export EXPECTED_BOT="app/orang-gaboets-release-please"
export RELEASE_APPROVER_ORG="orang-gaboets"
export RELEASE_APPROVER_TEAM="octostate-publishers"
export PR_NUMBER=250
export PR_URL="https://github.com/orang-gaboets/octostate/pull/250"
export PR_HEAD_SHA="head-sha"
export RELEASE_READY_LABEL="release: ready"
export EVENT_ACTION=labeled
export EVENT_LABEL="release: ready"
export EVENT_SENDER="publisher"
export EVENT_LABELS_JSON='[{"name":"release: ready"},{"name":"autorelease: pending"}]'

# shellcheck source=./release-approval-gate.sh
source "$SCRIPT_DIR/release-approval-gate.sh"

assert_status() {
  local expected="$1"
  local actual="$2"
  if [ "$actual" -ne "$expected" ]; then
    echo "expected status $expected, got $actual" >&2
    cat "$TEST_ROOT/stderr" >&2 || true
    exit 1
  fi
}

assert_contains() {
  local needle="$1"
  local file="$2"
  grep -F -- "$needle" "$file" >/dev/null || {
    echo "expected $file to contain: $needle" >&2
    cat "$file" >&2 || true
    exit 1
  }
}

assert_not_contains() {
  local needle="$1"
  local file="$2"
  if grep -F -- "$needle" "$file" >/dev/null; then
    echo "expected $file not to contain: $needle" >&2
    cat "$file" >&2 || true
    exit 1
  fi
}

assert_line_order() {
  local first="$1"
  local second="$2"
  local file="$3"
  local first_line
  local second_line

  first_line="$(grep -nF -- "$first" "$file" | head -n 1 | cut -d: -f1)"
  second_line="$(grep -nF -- "$second" "$file" | head -n 1 | cut -d: -f1)"
  if [ -z "$first_line" ] || [ -z "$second_line" ] || [ "$first_line" -ge "$second_line" ]; then
    echo "expected '$first' before '$second' in $file" >&2
    exit 1
  fi
}

assert_no_finalized_authorization() {
  assert_not_contains 'authorization_finalized=' "$GITHUB_OUTPUT"
  assert_not_contains 'authorized_head_sha=' "$GITHUB_OUTPUT"
}

write_pr() {
  local labels="$1"
  local base="${2:-main}"
  local head_ref="${3:-release-please--branches--main}"
  local head_repo="${4:-orang-gaboets/octostate}"
  local draft="${5:-false}"
  local author="${6:-app/orang-gaboets-release-please}"
  local head_sha="${7:-head-sha}"

  jq -n \
    --arg base "$base" \
    --arg head_ref "$head_ref" \
    --arg head_repo "$head_repo" \
    --argjson draft "$draft" \
    --arg author "$author" \
    --arg head_sha "$head_sha" \
    --argjson labels "$labels" \
    '{base:{ref:$base},head:{ref:$head_ref,repo:{full_name:$head_repo},sha:$head_sha},draft:$draft,user:{login:$author},labels:$labels}' \
    > "$PR_FIXTURE"
}

run_gate() {
  : > "$GH_LOG"
  : > "$GITHUB_OUTPUT"
  : > "$TEST_ROOT/stdout"
  : > "$TEST_ROOT/stderr"
  set +e
  "$@" > "$TEST_ROOT/stdout" 2> "$TEST_ROOT/stderr"
  GATE_STATUS=$?
  set -e
}

assert_no_gh_mutation() {
  if [ -s "$GH_LOG" ]; then
    echo "unexpected GitHub mutation:" >&2
    cat "$GH_LOG" >&2
    exit 1
  fi
}

both_labels='[{"name":"release: ready"},{"name":"autorelease: pending"},{"name":"other"}]'
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 0 "$GATE_STATUS"
assert_contains 'should_merge=true' "$GITHUB_OUTPUT"

EVENT_LABELS_JSON='[{"name":"release: ready"}]'
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 1 "$GATE_STATUS"
assert_contains 'pr edit' "$GH_LOG"
assert_contains 'approval was applied while autorelease: pending was absent' "$TEST_ROOT/stderr"
if grep -F 'autorelease: pending is absent while' "$TEST_ROOT/stderr" >/dev/null; then
  echo "event-time invalidation emitted a live-state diagnostic" >&2
  cat "$TEST_ROOT/stderr" >&2
  exit 1
fi
EVENT_LABELS_JSON="$both_labels"

EVENT_LABEL=other
run_gate release_approval_gate_initial
assert_status 0 "$GATE_STATUS"
assert_contains 'should_merge=false' "$GITHUB_OUTPUT"
assert_no_gh_mutation
EVENT_LABEL="$RELEASE_READY_LABEL"

write_pr '[{"name":"release: ready"}]'
run_gate release_approval_gate_initial
assert_status 1 "$GATE_STATUS"
assert_contains 'pr edit' "$GH_LOG"
assert_contains '--remove-label release: ready' "$GH_LOG"
assert_contains 'autorelease: pending is absent' "$TEST_ROOT/stderr"

write_pr '[{"name":"autorelease: pending"}]'
run_gate release_approval_gate_initial
assert_status 1 "$GATE_STATUS"
assert_no_gh_mutation

write_pr '[]'
run_gate release_approval_gate_initial
assert_status 1 "$GATE_STATUS"
assert_no_gh_mutation

RELEASE_READY_LABEL='release: approved'
EVENT_LABEL="$RELEASE_READY_LABEL"
write_pr '[{"name":"release: approved"},{"name":"autorelease: pending"}]'
run_gate release_approval_gate_initial
assert_status 0 "$GATE_STATUS"
assert_contains 'should_merge=true' "$GITHUB_OUTPUT"
RELEASE_READY_LABEL='release: ready'
EVENT_LABEL="$RELEASE_READY_LABEL"

EVENT_ACTION=synchronize
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 0 "$GATE_STATUS"
assert_contains 'pr edit' "$GH_LOG"
assert_contains 'should_merge=false' "$GITHUB_OUTPUT"
EVENT_ACTION=labeled

EVENT_ACTION=unlabeled
EVENT_LABEL="$RELEASE_LIFECYCLE_LABEL"
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 0 "$GATE_STATUS"
assert_contains 'pr edit' "$GH_LOG"
assert_contains '--remove-label release: ready' "$GH_LOG"
assert_contains 'should_merge=false' "$GITHUB_OUTPUT"
EVENT_ACTION=labeled
EVENT_LABEL="$RELEASE_READY_LABEL"

EVENT_LABEL="$RELEASE_LIFECYCLE_LABEL"
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 0 "$GATE_STATUS"
assert_contains 'pr edit' "$GH_LOG"
assert_contains '--remove-label release: ready' "$GH_LOG"
assert_contains 'was restored while release: ready was present' "$TEST_ROOT/stderr"
assert_contains 'should_merge=false' "$GITHUB_OUTPUT"
EVENT_LABEL="$RELEASE_READY_LABEL"

write_pr "$both_labels"
run_gate release_approval_gate_final
assert_status 0 "$GATE_STATUS"
assert_contains 'authorization_finalized=true' "$GITHUB_OUTPUT"
assert_contains 'authorized_head_sha=head-sha' "$GITHUB_OUTPUT"

# Simulate the configured approval being removed after initial approval while
# prerequisite checks run. The post-check final read must reject that state.
EVENT_ACTION=labeled
EVENT_LABEL="$RELEASE_READY_LABEL"
EVENT_LABELS_JSON="$both_labels"
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 0 "$GATE_STATUS"
assert_contains 'should_merge=true' "$GITHUB_OUTPUT"
write_pr '[{"name":"autorelease: pending"}]'
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_finalized_authorization
assert_no_gh_mutation

# A retry must re-read live state. The approval is absent in the new fixture,
# so the successful prior authorization cannot be reused.
write_pr '[{"name":"autorelease: pending"}]'
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_finalized_authorization
assert_no_gh_mutation

write_pr '[{"name":"release: ready"}]'
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_finalized_authorization
assert_contains 'pr edit' "$GH_LOG"
assert_contains '--remove-label release: ready' "$GH_LOG"

write_pr '[{"name":"autorelease: pending"}]'
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_finalized_authorization
assert_no_gh_mutation

export GH_STUB_MODE=read-failure
write_pr "$both_labels"
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_finalized_authorization
assert_no_gh_mutation
assert_contains 'preserving release: ready' "$TEST_ROOT/stderr"
unset GH_STUB_MODE

export GH_STUB_MODE=invalid-json
write_pr "$both_labels"
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_finalized_authorization
assert_no_gh_mutation
assert_contains 'not valid JSON' "$TEST_ROOT/stderr"
unset GH_STUB_MODE

export GH_STUB_REMOVE_MODE=fail
write_pr '[{"name":"release: ready"}]'
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_finalized_authorization
assert_contains 'manual remediation is required' "$TEST_ROOT/stderr"
unset GH_STUB_REMOVE_MODE

assert_final_reject() {
  local description="$1"
  shift
  write_pr "$both_labels" "$@"
  run_gate release_approval_gate_final
  assert_status 1 "$GATE_STATUS"
  assert_no_finalized_authorization
  assert_no_gh_mutation
  echo "checked final precondition: $description"
}

assert_final_reject 'base branch' dev
assert_final_reject 'head branch' main 'feature-branch'
assert_final_reject 'head repository' main 'release-please--branches--main' 'other/octostate'
assert_final_reject 'draft state' main 'release-please--branches--main' 'orang-gaboets/octostate' true
assert_final_reject 'author' main 'release-please--branches--main' 'orang-gaboets/octostate' false 'someone-else'
assert_final_reject 'head SHA' main 'release-please--branches--main' 'orang-gaboets/octostate' false 'app/orang-gaboets-release-please' other-sha

# The merge wrapper receives only the SHA finalized by the live-state gate,
# and its command failure must remain a workflow failure.
write_pr "$both_labels"
run_gate release_approval_gate_final
assert_status 0 "$GATE_STATUS"
# If the run reaches merge after a label is removed, its one-shot authorization
# remains bound to the finalized SHA. This fixture does not model whether
# GitHub Actions cancels the run before that step.
FINALIZED_HEAD_SHA="$(sed -n 's/^authorized_head_sha=//p' "$GITHUB_OUTPUT")"
write_pr '[{"name":"autorelease: pending"}]'
PR_HEAD_SHA="$FINALIZED_HEAD_SHA"
run_gate release_approval_gate_merge
assert_status 0 "$GATE_STATUS"
assert_contains 'pr merge --admin --squash --delete-branch --match-head-commit head-sha https://github.com/orang-gaboets/octostate/pull/250' "$GH_LOG"

export GH_STUB_MERGE_MODE=fail
run_gate release_approval_gate_merge
assert_status 1 "$GATE_STATUS"
assert_contains 'pr merge --admin --squash --delete-branch --match-head-commit head-sha https://github.com/orang-gaboets/octostate/pull/250' "$GH_LOG"
unset GH_STUB_MERGE_MODE

# The shell harness does not execute GitHub's merge API. Guard the workflow
# integration that feeds the finalized head to its existing merge precondition.
WORKFLOW_FILE="$SCRIPT_DIR/../workflows/automerge-release-please.yml"
assert_contains "steps.final-release-state.outputs.authorization_finalized == 'true'" "$WORKFLOW_FILE"
# This assertion checks the literal GitHub Actions expression.
# shellcheck disable=SC2016
assert_contains 'PR_HEAD_SHA: ${{ steps.final-release-state.outputs.authorized_head_sha }}' "$WORKFLOW_FILE"
assert_contains 'release_approval_gate_merge' "$WORKFLOW_FILE"
# This assertion checks the literal shell command and its variable references.
# shellcheck disable=SC2016
assert_contains 'gh pr merge --admin --squash --delete-branch --match-head-commit "$PR_HEAD_SHA" "$PR_URL"' "$SCRIPT_DIR/release-approval-gate.sh"
assert_not_contains 'merge_ready=true' "$WORKFLOW_FILE"
assert_line_order 'name: Wait for release checks' 'name: Finalize release approval for verified head' "$WORKFLOW_FILE"
assert_line_order 'name: Finalize release approval for verified head' 'name: Merge release-please PR' "$WORKFLOW_FILE"

export GH_STUB_MODE=unauthorized
EVENT_ACTION=labeled
EVENT_LABEL="$RELEASE_READY_LABEL"
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 1 "$GATE_STATUS"
assert_contains 'pr edit' "$GH_LOG"
assert_contains 'pr comment' "$GH_LOG"
unset GH_STUB_MODE

export GH_STUB_MODE=team-failure
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 1 "$GATE_STATUS"
assert_no_gh_mutation
assert_contains 'Failed to verify release approver team' "$TEST_ROOT/stderr"
unset GH_STUB_MODE

export GH_STUB_MODE=membership-failure
write_pr "$both_labels"
run_gate release_approval_gate_initial
assert_status 1 "$GATE_STATUS"
assert_no_gh_mutation
assert_contains 'Leaving release: ready in place' "$TEST_ROOT/stderr"
unset GH_STUB_MODE

echo "release approval gate tests passed"
