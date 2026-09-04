#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
test_root=$(mktemp -d)
trap 'rm -rf "$test_root"' EXIT

sentinel='sentinel-live-token-should-not-leak'

fail() {
  echo "FAIL: $1" >&2
  exit 1
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
    echo "did not expect $file to contain: $needle" >&2
    cat "$file" >&2 || true
    exit 1
  fi
}

count_log() {
  local needle="$1"
  local file="$2"
  grep -F -- "$needle" "$file" | wc -l | tr -d ' ' || true
}

assert_count() {
  local expected="$1"
  local needle="$2"
  local file="$3"
  local actual
  actual=$(count_log "$needle" "$file")
  if [ "$actual" != "$expected" ]; then
    echo "expected $expected occurrences of $needle, got $actual" >&2
    cat "$file" >&2 || true
    exit 1
  fi
}

write_stubs() {
  local bin_dir="$1"
  mkdir -p "$bin_dir"

  cat >"$bin_dir/gh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

printf 'gh %s\n' "$*" >>"$LIVE_STUB_LOG"

case "${1:-}" in
  api)
    case "${2:-}" in
      /orgs/octostate-test)
        case "${LIVE_STUB_ORG_MODE:-ok}" in
          malformed) printf '{not-json\n' ;;
          null) printf 'null\n' ;;
          wrong-type) printf '[]\n' ;;
          login-mismatch) jq -nc '{login:"wrong-org", id:321418529}' ;;
          id-mismatch) jq -nc '{login:"octostate-test", id:1}' ;;
          ok) jq -nc '{login:"octostate-test", id:321418529}' ;;
          *) echo "unknown org mode" >&2; exit 99 ;;
        esac
        ;;
      /repos/octostate-test/octostate-fixture-repo)
        case "${LIVE_STUB_REPO_MODE:-ok}" in
          malformed) printf '{not-json\n' ;;
          null) printf 'null\n' ;;
          wrong-type) printf '[]\n' ;;
          *) ;;
        esac
        case "${LIVE_STUB_REPO_MODE:-ok}" in
          malformed|null|wrong-type) exit 0 ;;
        esac

        topics='[]'
        case "$(cat "$LIVE_STUB_STATE")" in
          baseline) topics='[]' ;;
          mutated) topics='["octostate-live-integration"]' ;;
          dirty) topics='["someone-else"]' ;;
          *) echo "unknown state" >&2; exit 99 ;;
        esac

        jq -nc \
          --argjson topics "$topics" \
          --arg mode "${LIVE_STUB_REPO_MODE:-ok}" '
          {
            id: (if $mode == "id-mismatch" then 1 else 1347356483 end),
            owner: {login: (if $mode == "owner-mismatch" then "wrong-owner" else "octostate-test" end)},
            name: (if $mode == "name-mismatch" then "wrong-repo" else "octostate-fixture-repo" end),
            visibility: (if $mode == "visibility-mismatch" then "private" else "public" end),
            default_branch: (if $mode == "branch-mismatch" then "trunk" else "main" end),
            description: (if $mode == "description-mismatch" then "wrong" else "Persistent fixture for Octostate live integration testing." end),
            archived: (if $mode == "archived-mismatch" then true else false end),
            is_template: (if $mode == "template-mismatch" then true else false end),
            topics: (if $mode == "topics-wrong-type" then "bad" else $topics end)
          }'
        ;;
      *)
        echo "unexpected gh api target: ${2:-}" >&2
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
  chmod +x "$bin_dir/gh"

  cat >"$bin_dir/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

printf 'go %s\n' "$*" >>"$LIVE_STUB_LOG"

if [ "${1:-}" != "run" ] || [ "${2:-}" != "./cmd/octostate" ]; then
  echo "unexpected go command: $*" >&2
  exit 99
fi
shift 2

find_config_dir() {
  local previous=''
  for arg in "$@"; do
    if [ "$previous" = "--config-dir" ]; then
      printf '%s\n' "$arg"
      return 0
    fi
    previous="$arg"
  done
  return 1
}

find_state_dir() {
  local previous=''
  for arg in "$@"; do
    if [ "$previous" = "--state-dir" ]; then
      printf '%s\n' "$arg"
      return 0
    fi
    previous="$arg"
  done
  return 1
}

desired_state() {
  local config_dir
  config_dir=$(find_config_dir "$@")
  if grep -F 'octostate-live-integration' "$config_dir/organization.yaml" >/dev/null; then
    printf 'mutated\n'
  else
    printf 'baseline\n'
  fi
}

action_json() {
  local desired="$1"
  local extra="${2:-none}"
  local to_json='[]'
  local from_json='["octostate-live-integration"]'
  if [ "$desired" = "mutated" ]; then
    to_json='["octostate-live-integration"]'
    from_json='[]'
  fi
  jq -nc \
    --argjson from "$from_json" \
    --argjson to "$to_json" \
    --arg extra "$extra" '
    {
      resource_type:"repository",
      operation:"update",
      resource_id:"octostate-test/octostate-fixture-repo",
      executable:true,
      message:"update repository octostate-test/octostate-fixture-repo",
      changes:[{field:(if $extra == "wrong-field" then "description" else "topics" end), from:$from, to:$to}]
    }'
}

plan_payload() {
  local desired="$1"
  local current
  current=$(cat "$LIVE_STUB_STATE")
  local actions='[]'
  local executable=0
  if [ "${LIVE_STUB_EXTRA_ACTION:-0}" = "1" ] && [ "$desired" = "mutated" ] && [ "$current" = "baseline" ]; then
    actions=$(jq -nc --argjson first "$(action_json mutated)" '[ $first, {resource_type:"team", operation:"update", resource_id:"octostate-test/team", executable:true, message:"extra", changes:[{field:"description", to:"x"}]} ]')
    executable=2
  elif [ "${LIVE_STUB_WRONG_FIELD_ACTION:-0}" = "1" ] && [ "$desired" = "mutated" ] && [ "$current" = "baseline" ]; then
    actions=$(jq -nc --argjson first "$(action_json mutated wrong-field)" '[ $first ]')
    executable=1
  elif [ "${LIVE_STUB_POLL_PLAN_DIRTY:-0}" = "1" ] && [ "$desired" = "mutated" ] && [ "$current" = "mutated" ]; then
    actions=$(jq -nc --argjson first "$(action_json mutated)" '[ $first ]')
    executable=1
  elif [ "$desired" != "$current" ]; then
    case "$current" in
      baseline|mutated)
        actions=$(jq -nc --argjson first "$(action_json "$desired")" '[ $first ]')
        executable=1
        ;;
      dirty)
        actions='[{"resource_type":"repository","operation":"update","resource_id":"octostate-test/octostate-fixture-repo","executable":true,"message":"dirty","changes":[{"field":"topics","to":[]}]}]'
        executable=1
        ;;
    esac
  fi
  jq -nc \
    --arg org "octostate-test" \
    --argjson actions "$actions" \
    --argjson executable "$executable" '
    {
      organization:$org,
      plan_summary:{
        has_changes:($executable > 0),
        actions:($actions|length),
        executable_actions:$executable,
        non_executable_actions:0,
        create_actions:0,
        update_actions:$executable,
        delete_actions:0,
        remove_actions:0
      },
      executable_actions:$actions,
      skipped_actions:[]
    }'
}

case "${1:-} ${2:-}" in
  "config validate")
    jq -nc '{valid:true, summary:{errors:0,warnings:0}, errors:[], warnings:[]}'
    ;;
  "config plan")
    if [ "${LIVE_STUB_PLAN_MALFORMED:-0}" = "1" ]; then
      printf '{not-json\n'
      exit 0
    fi
    plan_payload "$(desired_state "$@")"
    ;;
  "config apply")
    desired=$(desired_state "$@")
    if [ "${3:-}" = "--check" ] || [ "${4:-}" = "--check" ] || [ "${5:-}" = "--check" ] || printf '%s\n' "$*" | grep -F -- '--check' >/dev/null; then
      payload=$(plan_payload "$desired")
      jq -nc --argjson payload "$payload" '{status:"check", message:"checked", data:{organization:$payload.organization, plan_summary:$payload.plan_summary, checked_actions:$payload.executable_actions, skipped_actions:$payload.skipped_actions}}'
      exit 0
    fi

    if [ "$desired" = "mutated" ]; then
      printf 'apply-mutated\n' >>"$LIVE_STUB_APPLY_LOG"
      if [ "${LIVE_STUB_SIGNAL_ON_MUTATION:-}" = "TERM" ]; then
        printf 'mutated\n' >"$LIVE_STUB_STATE"
        kill -TERM "$PPID"
        /bin/sleep 1
      fi
      if [ "${LIVE_STUB_SIGNAL_ON_MUTATION:-}" = "INT" ]; then
        printf 'mutated\n' >"$LIVE_STUB_STATE"
        kill -INT "$PPID"
        /bin/sleep 1
      fi
      if [ "${LIVE_STUB_MUTATION_FAIL_AFTER_WRITE:-0}" = "1" ]; then
        printf 'mutated\n' >"$LIVE_STUB_STATE"
        echo "simulated mutation failure" >&2
        exit 1
      fi
    else
      printf 'apply-baseline\n' >>"$LIVE_STUB_APPLY_LOG"
      if [ "${LIVE_STUB_RESTORE_FAIL:-0}" = "1" ]; then
        echo "simulated restoration failure" >&2
        exit 1
      fi
    fi

    payload=$(plan_payload "$desired")
    printf '%s\n' "$desired" >"$LIVE_STUB_STATE"
    jq -nc --argjson payload "$payload" '{status:"success", message:"applied", data:{organization:$payload.organization, plan_summary:$payload.plan_summary, executed_actions:$payload.executable_actions, skipped_actions:$payload.skipped_actions}}'
    ;;
  "audit pull")
    state_dir=$(find_state_dir "$@")
    mkdir -p "$state_dir/actual"
    jq -nc '{organization:"octostate-test"}' >"$state_dir/actual/snapshot.json"
    jq -nc '{status:"success", message:"wrote actual-state snapshot", data:{organization:"octostate-test", path:"snapshot.json", pulled_at:"2026-09-04T00:00:00Z"}}'
    ;;
  "audit diff")
    jq -nc '{organization:"octostate-test", snapshot_pulled_at:"2026-09-04T00:00:00Z", summary:{has_changes:false, actions:0, executable_actions:0, non_executable_actions:0}, actions:[]}'
    ;;
  *)
    echo "unexpected octostate command: $*" >&2
    exit 99
    ;;
esac
EOF
  chmod +x "$bin_dir/go"

  cat >"$bin_dir/sleep" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'sleep %s\n' "$*" >>"$LIVE_STUB_LOG"
EOF
  chmod +x "$bin_dir/sleep"
}

run_case() {
  local name="$1"
  local mode="$2"
  local expected_status="$3"
  shift 3

  local case_dir="$test_root/$name"
  mkdir -p "$case_dir"
  printf '%s\n' "${LIVE_STUB_INITIAL_STATE:-baseline}" >"$case_dir/state"
  : >"$case_dir/commands.log"
  : >"$case_dir/apply.log"
  : >"$case_dir/stdout"
  : >"$case_dir/stderr"
  : >"$case_dir/summary"
  write_stubs "$case_dir/bin"

  set +e
  (
    cd "$(git -C "$script_dir/../.." rev-parse --show-toplevel)"
    PATH="$case_dir/bin:$PATH" \
      RUNNER_TEMP="$case_dir/runner-temp" \
      GITHUB_RUN_ID="12345" \
      GITHUB_SHA="test-sha" \
      GITHUB_STEP_SUMMARY="$case_dir/summary" \
      OCTOSTATE_GITHUB_TOKEN="$sentinel" \
      GH_TOKEN="$sentinel" \
      LIVE_STUB_LOG="$case_dir/commands.log" \
      LIVE_STUB_APPLY_LOG="$case_dir/apply.log" \
      LIVE_STUB_STATE="$case_dir/state" \
      "$@" \
      bash "$script_dir/live-integration.sh" "$mode"
  ) >"$case_dir/stdout" 2>"$case_dir/stderr"
  status=$?
  set -e

  if [ "$status" -ne "$expected_status" ]; then
    echo "case $name: expected status $expected_status, got $status" >&2
    cat "$case_dir/stderr" >&2 || true
    exit 1
  fi

  assert_not_contains "$sentinel" "$case_dir/stdout"
  assert_not_contains "$sentinel" "$case_dir/stderr"
  assert_not_contains "$sentinel" "$case_dir/summary"

  CASE_DIR="$case_dir"
}

run_case invalid_flag --bogus 2
assert_count 0 "go run ./cmd/octostate config apply" "$CASE_DIR/commands.log"

for mode in malformed null wrong-type; do
  run_case "org_$mode" --read-only 1 env LIVE_STUB_ORG_MODE="$mode"
  assert_count 0 "go run ./cmd/octostate config apply" "$CASE_DIR/commands.log"
done

for mode in login-mismatch id-mismatch; do
  run_case "org_$mode" --read-only 1 env LIVE_STUB_ORG_MODE="$mode"
  assert_count 0 "go run ./cmd/octostate config apply" "$CASE_DIR/commands.log"
done

for mode in malformed null wrong-type id-mismatch owner-mismatch name-mismatch visibility-mismatch branch-mismatch description-mismatch archived-mismatch template-mismatch topics-wrong-type; do
  run_case "repo_$mode" --read-only 1 env LIVE_STUB_REPO_MODE="$mode"
  assert_count 0 "go run ./cmd/octostate config apply" "$CASE_DIR/commands.log"
done

LIVE_STUB_INITIAL_STATE=dirty run_case dirty_baseline --mutate 1
assert_count 0 "go run ./cmd/octostate config apply" "$CASE_DIR/commands.log"
unset LIVE_STUB_INITIAL_STATE

run_case read_only_success --read-only 0
assert_count 1 "go run ./cmd/octostate config apply --config-dir" "$CASE_DIR/commands.log"
assert_count 0 "apply-mutated" "$CASE_DIR/apply.log"
assert_contains "Final: PASS" "$CASE_DIR/summary"

run_case mutate_success --mutate 0
assert_count 1 "apply-mutated" "$CASE_DIR/apply.log"
assert_count 1 "apply-baseline" "$CASE_DIR/apply.log"
assert_contains "Final: PASS" "$CASE_DIR/summary"
if [ "$(cat "$CASE_DIR/state")" != "baseline" ]; then
  fail "mutation success did not restore baseline"
fi

run_case mutation_failure_cleans_once --mutate 1 env LIVE_STUB_MUTATION_FAIL_AFTER_WRITE=1
assert_count 1 "apply-mutated" "$CASE_DIR/apply.log"
assert_count 1 "apply-baseline" "$CASE_DIR/apply.log"
assert_contains "Final: FAIL" "$CASE_DIR/summary"
if [ "$(cat "$CASE_DIR/state")" != "baseline" ]; then
  fail "mutation failure cleanup did not restore baseline"
fi

run_case restoration_failure_dirty --mutate 1 env LIVE_STUB_RESTORE_FAIL=1
assert_count 1 "apply-mutated" "$CASE_DIR/apply.log"
assert_count 1 "apply-baseline" "$CASE_DIR/apply.log"
assert_contains "dirty sandbox" "$CASE_DIR/summary"
if [ "$(cat "$CASE_DIR/state")" != "mutated" ]; then
  fail "restoration failure should leave the stub dirty"
fi

run_case extra_executable_action --mutate 1 env LIVE_STUB_EXTRA_ACTION=1
assert_count 0 "apply-mutated" "$CASE_DIR/apply.log"

run_case wrong_field_action --mutate 1 env LIVE_STUB_WRONG_FIELD_ACTION=1
assert_count 0 "apply-mutated" "$CASE_DIR/apply.log"

run_case polling_timeout_after_mutation --mutate 1 env LIVE_STUB_POLL_PLAN_DIRTY=1
assert_count 1 "apply-mutated" "$CASE_DIR/apply.log"
assert_count 1 "apply-baseline" "$CASE_DIR/apply.log"
assert_count 11 "sleep 5" "$CASE_DIR/commands.log"
assert_contains "convergence: FAIL" "$CASE_DIR/summary"

run_case malformed_plan_json --mutate 1 env LIVE_STUB_PLAN_MALFORMED=1
assert_count 0 "apply-mutated" "$CASE_DIR/apply.log"

run_case term_cleanup_reentry --mutate 143 env LIVE_STUB_SIGNAL_ON_MUTATION=TERM
assert_count 1 "apply-mutated" "$CASE_DIR/apply.log"
assert_count 1 "apply-baseline" "$CASE_DIR/apply.log"

run_case int_cleanup_reentry --mutate 143 env LIVE_STUB_SIGNAL_ON_MUTATION=INT
assert_count 1 "apply-mutated" "$CASE_DIR/apply.log"
assert_count 1 "apply-baseline" "$CASE_DIR/apply.log"

echo "live integration harness tests passed"
