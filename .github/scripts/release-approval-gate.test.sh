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

write_pr '[{"name":"release: ready"}]'
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_contains 'pr edit' "$GH_LOG"
assert_contains '--remove-label release: ready' "$GH_LOG"

write_pr '[{"name":"autorelease: pending"}]'
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_gh_mutation

export GH_STUB_MODE=read-failure
write_pr "$both_labels"
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_gh_mutation
assert_contains 'preserving release: ready' "$TEST_ROOT/stderr"
unset GH_STUB_MODE

export GH_STUB_MODE=invalid-json
write_pr "$both_labels"
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_no_gh_mutation
assert_contains 'not valid JSON' "$TEST_ROOT/stderr"
unset GH_STUB_MODE

export GH_STUB_REMOVE_MODE=fail
write_pr '[{"name":"release: ready"}]'
run_gate release_approval_gate_final
assert_status 1 "$GATE_STATUS"
assert_contains 'manual remediation is required' "$TEST_ROOT/stderr"
unset GH_STUB_REMOVE_MODE

assert_final_reject() {
  local description="$1"
  shift
  write_pr "$both_labels" "$@"
  run_gate release_approval_gate_final
  assert_status 1 "$GATE_STATUS"
  assert_no_gh_mutation
  echo "checked final precondition: $description"
}

assert_final_reject 'base branch' dev
assert_final_reject 'head branch' main 'feature-branch'
assert_final_reject 'head repository' main 'release-please--branches--main' 'other/octostate'
assert_final_reject 'draft state' main 'release-please--branches--main' 'orang-gaboets/octostate' true
assert_final_reject 'author' main 'release-please--branches--main' 'orang-gaboets/octostate' false 'someone-else'
assert_final_reject 'head SHA' main 'release-please--branches--main' 'orang-gaboets/octostate' false 'app/orang-gaboets-release-please' other-sha

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
