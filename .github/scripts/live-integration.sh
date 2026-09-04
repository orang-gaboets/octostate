#!/usr/bin/env bash
set -euo pipefail

EXPECTED_ORGANIZATION='octostate-test'
EXPECTED_ORGANIZATION_ID='321418529'
EXPECTED_REPOSITORY='octostate-fixture-repo'
EXPECTED_REPOSITORY_ID='1347356483'
MUTATION_TOPIC='octostate-live-integration'
BASELINE_CONFIG_DIR='testdata/live-integration'
POLL_ATTEMPTS=12
POLL_DELAY_SECONDS=5
CLEANUP_DEADLINE_SECONDS=${OCTOSTATE_CLEANUP_DEADLINE_SECONDS:-540}

mode=''

SCRATCH_DIR=''
SUMMARY_FILE=${GITHUB_STEP_SUMMARY:-}
ORIGINAL_PROJECTION=''
EXPECTED_MUTATED_PROJECTION=''
MUTATION_STARTED=0
RESTORE_ATTEMPTED=0
RESTORATION_COMPLETED=0
CLEANUP_RUNNING=0
FINAL_STATUS=1
PHASE='startup'
PHASE_BASELINE='NOT RUN'
PHASE_READ_ONLY='NOT RUN'
PHASE_MUTATION='NOT RUN'
PHASE_CONVERGENCE='NOT RUN'
RESTORATION_CONVERGENCE='NOT RUN'
PHASE_RESTORATION='NOT RUN'
ACTION_GUARD_RESULT='NOT RUN'

fail() {
  echo "live integration: $*" >&2
  return 1
}

require_commands() {
  command -v gh >/dev/null 2>&1 || fail 'gh is required'
  command -v go >/dev/null 2>&1 || fail 'go is required'
  command -v jq >/dev/null 2>&1 || fail 'jq is required'
  [ -n "${OCTOSTATE_GITHUB_TOKEN:-}" ] || fail 'OCTOSTATE_GITHUB_TOKEN is required'
}

expected_baseline_projection() {
  jq -ncS \
    --argjson id "$EXPECTED_REPOSITORY_ID" \
    --arg owner "$EXPECTED_ORGANIZATION" \
    --arg name "$EXPECTED_REPOSITORY" \
    '{id:$id, owner:$owner, name:$name, visibility:"public", default_branch:"main", description:"Persistent fixture for Octostate live integration testing.", archived:false, is_template:false, topics:[]}'
}

normalize_repository() {
  local input=$1
  local output=$2
  jq -e -cS \
    'if (type == "object" and
        (.id | type) == "number" and
        (.owner | type) == "object" and (.owner.login | type) == "string" and
        (.name | type) == "string" and
        (.visibility | type) == "string" and
        (.default_branch | type) == "string" and
        (.description | type) == "string" and
        (.archived | type) == "boolean" and
        (.is_template | type) == "boolean" and
        (.topics | type) == "array" and all(.topics[]; type == "string"))
     then {id:.id, owner:.owner.login, name:.name, visibility:.visibility,
           default_branch:.default_branch, description:.description,
           archived:.archived, is_template:.is_template, topics:(.topics | sort)}
     else error("invalid repository shape") end' \
    "$input" >"$output" || return 1
}

classify_gh_failure() {
  local error_file=$1
  if grep -Eq '(^|[^[:digit:]])(408|429|5[0-9]{2})([^[:digit:]]|$)|timed out|timeout|temporary|connection|network|resolve|TLS|EOF|reset' "$error_file"; then
    return 2
  fi
  return 3
}

read_organization_json() {
  local output=$1
  local error_file=$SCRATCH_DIR/organization.error.txt
  if ! gh api "/orgs/$EXPECTED_ORGANIZATION" >"$output" 2>"$error_file"; then
    classify_gh_failure "$error_file"
    return $?
  fi
  return 0
}

read_repository_projection() {
  local raw=$SCRATCH_DIR/repository.raw.json
  local error_file=$SCRATCH_DIR/repository.error.txt
  local projection=$1
  if ! gh api "/repos/$EXPECTED_ORGANIZATION/$EXPECTED_REPOSITORY" >"$raw" 2>"$error_file"; then
    classify_gh_failure "$error_file"
    return $?
  fi
  if ! normalize_repository "$raw" "$projection"; then
    return 3
  fi
  return 0
}

read_with_retries() {
  local reader=$1
  local output=$2
  local attempt status
  for ((attempt = 1; attempt <= POLL_ATTEMPTS; attempt++)); do
    status=0
    "$reader" "$output" || status=$?
    if [ "$status" -eq 0 ] || [ "$status" -eq 3 ]; then
      return "$status"
    fi
    if [ "$attempt" -eq "$POLL_ATTEMPTS" ]; then
      return "$status"
    fi
    sleep "$POLL_DELAY_SECONDS"
  done
}

is_transient_failure_file() {
  local error_file=$1
  grep -Eq '(^|[^[:digit:]])(408|429|5[0-9]{2})([^[:digit:]]|$)|timed out|timeout|temporary|connection|network|resolve|TLS|EOF|reset' "$error_file"
}

assert_baseline_projection() {
  local projection=$1
  local expected
  expected=$(expected_baseline_projection)
  [ "$(cat "$projection")" = "$expected" ]
}

assert_allowed_skipped_actions() {
  local report=$1
  local action_path=$2
  jq -e "$action_path |
    if type != \"array\" then false
    else all(.[]; . as \$action |
      (\$action.executable == false) and
      (if \$action.resource_type == \"organization_member\" then \$action.operation == \"delete\"
       elif \$action.resource_type == \"invite\" then \$action.operation == \"remove\"
       elif \$action.resource_type == \"team\" then \$action.operation == \"delete\"
       else false end))
    end" \
    "$report" >/dev/null
}

verify_target_and_baseline() {
  PHASE='baseline'
  PHASE_BASELINE='FAIL'
  local org_json=$SCRATCH_DIR/organization.json
  local repository_projection=$SCRATCH_DIR/repository.baseline.json

  local org_status=0
  read_with_retries read_organization_json "$org_json" || org_status=$?
  if [ "$org_status" -ne 0 ]; then
    fail 'organization lookup failed or had an invalid response'
    return 1
  fi
  if ! jq -e \
    --arg login "$EXPECTED_ORGANIZATION" \
    --argjson id "$EXPECTED_ORGANIZATION_ID" \
    'type == "object" and .login == $login and .id == $id' \
    "$org_json" >/dev/null; then
    fail 'organization identity mismatch'
    return 1
  fi

  local repository_status=0
  read_with_retries read_repository_projection "$repository_projection" || repository_status=$?
  if [ "$repository_status" -ne 0 ]; then
    fail 'repository response was malformed or had an invalid shape'
    return 1
  fi
  if ! assert_baseline_projection "$repository_projection"; then
    fail 'fixture baseline is dirty or has an unexpected identity'
    return 1
  fi

  PHASE_BASELINE='PASS'
  return 0
}

run_octostate() {
  local output=$1
  shift
  go run ./cmd/octostate "$@" >"$output" 2>"$output.stderr"
}

assert_validate_result() {
  local report=$1
  jq -e '
    type == "object" and .valid == true and
    (.summary | type == "object" and .errors == 0) and
    (.errors | type == "array") and (.warnings | type == "array")' \
    "$report" >/dev/null
}

assert_audit_pull_result() {
  local report=$1
  jq -e '
    type == "object" and .status == "success" and
    (.data | type == "object" and .organization == "octostate-test" and
      (.path | type == "string" and length > 0) and
      (.pulled_at | type == "string" and length > 0))' \
    "$report" >/dev/null
}

assert_zero_plan() {
  local report=$1
  if ! is_plan_envelope_valid "$report"; then
    return 1
  fi
  if ! assert_allowed_skipped_actions "$report" '.skipped_actions'; then
    return 1
  fi
  jq -e '
    .organization == "octostate-test" and
    .plan_summary.executable_actions == 0 and
    .plan_summary.actions == (.plan_summary.executable_actions + .plan_summary.non_executable_actions) and
    .plan_summary.has_changes == (.plan_summary.actions > 0) and
    (.executable_actions | length == 0) and
    (.skipped_actions | length) == .plan_summary.non_executable_actions' \
    "$report" >/dev/null
}

is_plan_envelope_valid() {
  local report=$1
  jq -e '
    type == "object" and .organization == "octostate-test" and
    (.plan_summary | type == "object" and
      (.has_changes | type == "boolean") and
      (.actions | type == "number") and
      (.executable_actions | type == "number") and
      (.non_executable_actions | type == "number") and
      (.create_actions | type == "number") and
      (.update_actions | type == "number") and
      (.delete_actions | type == "number") and
      (.remove_actions | type == "number") and
      .actions == (.executable_actions + .non_executable_actions) and
      .has_changes == (.actions > 0) and
      (.create_actions + .update_actions + .delete_actions + .remove_actions) == .actions) and
    (.executable_actions | type == "array") and
    (.skipped_actions | type == "array") and
    (.executable_actions | length) == .plan_summary.executable_actions and
    (.skipped_actions | length) == .plan_summary.non_executable_actions and
    .plan_summary.create_actions == ([.executable_actions[], .skipped_actions[]] | map(select(.operation == "create")) | length) and
    .plan_summary.update_actions == ([.executable_actions[], .skipped_actions[]] | map(select(.operation == "update")) | length) and
    .plan_summary.delete_actions == ([.executable_actions[], .skipped_actions[]] | map(select(.operation == "delete")) | length) and
    .plan_summary.remove_actions == ([.executable_actions[], .skipped_actions[]] | map(select(.operation == "remove")) | length)' \
    "$report" >/dev/null
}

assert_zero_check() {
  local report=$1
  if ! jq -e '
    type == "object" and .status == "check" and
    (.data | type == "object" and .organization == "octostate-test" and
      (.plan_summary | type == "object" and .executable_actions == 0 and
        .actions == (.executable_actions + .non_executable_actions) and
        .has_changes == (.actions > 0)) and
      (.checked_actions | type == "array" and length == 0) and
      (.skipped_actions | type == "array") and
      (.skipped_actions | length) == .plan_summary.non_executable_actions) and
    .data.plan_summary.create_actions == ([.data.checked_actions[], .data.skipped_actions[]] | map(select(.operation == "create")) | length) and
    .data.plan_summary.update_actions == ([.data.checked_actions[], .data.skipped_actions[]] | map(select(.operation == "update")) | length) and
    .data.plan_summary.delete_actions == ([.data.checked_actions[], .data.skipped_actions[]] | map(select(.operation == "delete")) | length) and
    .data.plan_summary.remove_actions == ([.data.checked_actions[], .data.skipped_actions[]] | map(select(.operation == "remove")) | length)' \
    "$report" >/dev/null; then
    return 1
  fi
  assert_allowed_skipped_actions "$report" '.data.skipped_actions'
}

assert_zero_diff() {
  local report=$1
  if ! jq -e '
    type == "object" and .organization == "octostate-test" and
    (.summary | type == "object" and .executable_actions == 0 and
      .actions == .non_executable_actions and .has_changes == (.actions > 0)) and
    .summary.create_actions == ([.actions[]] | map(select(.operation == "create")) | length) and
    .summary.update_actions == ([.actions[]] | map(select(.operation == "update")) | length) and
    .summary.delete_actions == ([.actions[]] | map(select(.operation == "delete")) | length) and
    .summary.remove_actions == ([.actions[]] | map(select(.operation == "remove")) | length) and
    (.actions | type == "array") and
    (.actions | length) == .summary.non_executable_actions' \
    "$report" >/dev/null; then
    return 1
  fi
  assert_allowed_skipped_actions "$report" '.actions'
}

run_read_only_scenarios() {
  PHASE='read-only'
  PHASE_READ_ONLY='FAIL'
  local config_dir=$1
  local plan=$SCRATCH_DIR/read-only.plan.json
  local check=$SCRATCH_DIR/read-only.check.json
  local pull=$SCRATCH_DIR/read-only.pull.json
  local diff=$SCRATCH_DIR/read-only.diff.json
  local state_dir=$SCRATCH_DIR/read-only-state

  if ! run_octostate "$SCRATCH_DIR/read-only.validate.json" config validate --config-dir "$config_dir"; then
    fail 'config validate failed'
    return 1
  fi
  if ! assert_validate_result "$SCRATCH_DIR/read-only.validate.json"; then
    fail 'config validate returned an invalid result envelope'
    return 1
  fi
  if ! run_octostate "$plan" config plan --config-dir "$config_dir"; then
    fail 'config plan failed'
    return 1
  fi
  if ! assert_zero_plan "$plan"; then
    fail 'baseline config plan contained executable actions'
    return 1
  fi
  if ! run_octostate "$check" config apply --config-dir "$config_dir" --check; then
    fail 'config apply --check failed'
    return 1
  fi
  if ! assert_zero_check "$check"; then
    fail 'baseline config apply --check was not a zero-action check'
    return 1
  fi
  if ! run_octostate "$pull" audit pull --config-dir "$config_dir" --state-dir "$state_dir"; then
    fail 'audit pull failed'
    return 1
  fi
  if ! assert_audit_pull_result "$pull"; then
    fail 'audit pull returned an invalid result envelope'
    return 1
  fi
  if ! jq -e '.organization == "octostate-test"' "$state_dir/actual/snapshot.json" >/dev/null; then
    fail 'audit snapshot organization mismatch'
    return 1
  fi
  if ! run_octostate "$diff" audit diff --config-dir "$config_dir" --state-dir "$state_dir"; then
    fail 'audit diff failed'
    return 1
  fi
  if ! assert_zero_diff "$diff"; then
    fail 'audit diff contained executable actions'
    return 1
  fi

  PHASE_READ_ONLY='PASS'
  return 0
}

write_mutated_config() {
  local baseline_dir=$1
  local mutated_dir=$2
  cp -R "$baseline_dir" "$mutated_dir"
  sed -i.bak "s/topics: \[\]/topics: [$MUTATION_TOPIC]/" "$mutated_dir/organization.yaml"
  rm -f "$mutated_dir/organization.yaml.bak"
}

assert_exact_topic_action() {
  local report=$1
  local action_path=$2
  local from_topics=$3
  local to_topics=$4
  local envelope=$5
  local skipped_path
  ACTION_GUARD_RESULT='FAIL'
  case "$envelope" in
    plan)
      skipped_path='.skipped_actions'
      if ! jq -e '
        type == "object" and .organization == "octostate-test" and
        (.plan_summary | type == "object" and .has_changes == true and
          .actions == (.executable_actions + .non_executable_actions) and
          .executable_actions == 1 and .update_actions >= 1 and
          (.create_actions + .update_actions + .delete_actions + .remove_actions) == .actions) and
        (.executable_actions | type == "array" and length == 1) and
        (.skipped_actions | type == "array") and
        (.skipped_actions | length) == .plan_summary.non_executable_actions and
        .plan_summary.create_actions == ([.executable_actions[], .skipped_actions[]] | map(select(.operation == "create")) | length) and
        .plan_summary.update_actions == ([.executable_actions[], .skipped_actions[]] | map(select(.operation == "update")) | length) and
        .plan_summary.delete_actions == ([.executable_actions[], .skipped_actions[]] | map(select(.operation == "delete")) | length) and
        .plan_summary.remove_actions == ([.executable_actions[], .skipped_actions[]] | map(select(.operation == "remove")) | length)' "$report" >/dev/null; then
        return 1
      fi
      ;;
    check)
      skipped_path='.data.skipped_actions'
      if ! jq -e '
        type == "object" and .status == "check" and
        (.data | type == "object" and .organization == "octostate-test" and
          (.plan_summary | type == "object" and .has_changes == true and
            .actions == (.executable_actions + .non_executable_actions) and
            .executable_actions == 1 and .update_actions >= 1 and
            (.create_actions + .update_actions + .delete_actions + .remove_actions) == .actions) and
          (.checked_actions | type == "array" and length == 1) and
          (.skipped_actions | type == "array") and
          (.skipped_actions | length) == .plan_summary.non_executable_actions) and
        .data.plan_summary.create_actions == ([.data.checked_actions[], .data.skipped_actions[]] | map(select(.operation == "create")) | length) and
        .data.plan_summary.update_actions == ([.data.checked_actions[], .data.skipped_actions[]] | map(select(.operation == "update")) | length) and
        .data.plan_summary.delete_actions == ([.data.checked_actions[], .data.skipped_actions[]] | map(select(.operation == "delete")) | length) and
        .data.plan_summary.remove_actions == ([.data.checked_actions[], .data.skipped_actions[]] | map(select(.operation == "remove")) | length)' "$report" >/dev/null; then
        return 1
      fi
      ;;
    apply)
      skipped_path='.data.skipped_actions'
      if ! jq -e '
        type == "object" and .status == "success" and
        (.data | type == "object" and .organization == "octostate-test" and
          (.plan_summary | type == "object" and .has_changes == true and
            .actions == (.executable_actions + .non_executable_actions) and
            .executable_actions == 1 and .update_actions >= 1 and
            (.create_actions + .update_actions + .delete_actions + .remove_actions) == .actions) and
          (.executed_actions | type == "array" and length == 1) and
          (.skipped_actions | type == "array") and
          (.skipped_actions | length) == .plan_summary.non_executable_actions) and
        .data.plan_summary.create_actions == ([.data.executed_actions[], .data.skipped_actions[]] | map(select(.operation == "create")) | length) and
        .data.plan_summary.update_actions == ([.data.executed_actions[], .data.skipped_actions[]] | map(select(.operation == "update")) | length) and
        .data.plan_summary.delete_actions == ([.data.executed_actions[], .data.skipped_actions[]] | map(select(.operation == "delete")) | length) and
        .data.plan_summary.remove_actions == ([.data.executed_actions[], .data.skipped_actions[]] | map(select(.operation == "remove")) | length)' "$report" >/dev/null; then
        return 1
      fi
      ;;
    *)
      return 1
      ;;
  esac
  if ! assert_allowed_skipped_actions "$report" "$skipped_path"; then
    return 1
  fi
  if ! jq -e "$action_path | type == \"array\" and length == 1" "$report" >/dev/null; then
    return 1
  fi
  if ! jq -e \
    --arg repo "$EXPECTED_ORGANIZATION/$EXPECTED_REPOSITORY" \
    --arg topic "$MUTATION_TOPIC" \
    --argjson from "$from_topics" \
    --argjson to "$to_topics" \
    "$action_path | .[0] |
      .resource_type == \"repository\" and .operation == \"update\" and
      .resource_id == \$repo and .executable == true and
      (.changes | type == \"array\" and length == 1) and
      .changes[0].field == \"topics\" and
      (.changes[0].from == \$from) and (.changes[0].to == \$to)" \
    "$report" >/dev/null; then
    return 1
  fi
  ACTION_GUARD_RESULT='PASS'
  return 0
}

prepare_mutation() {
  PHASE='prepare mutation'
  local mutated_dir=$SCRATCH_DIR/mutated-config
  local validate=$SCRATCH_DIR/mutated.validate.json
  local plan=$SCRATCH_DIR/mutated.plan.json
  local check=$SCRATCH_DIR/mutated.check.json
  local baseline_projection=$SCRATCH_DIR/repository.before-apply.json

  write_mutated_config "$SCRATCH_DIR/baseline-config" "$mutated_dir"
  if ! run_octostate "$validate" config validate --config-dir "$mutated_dir"; then
    fail 'mutated config validate failed'
    return 1
  fi
  if ! assert_validate_result "$validate"; then
    fail 'mutated config validate returned an invalid result envelope'
    return 1
  fi
  ORIGINAL_PROJECTION=$SCRATCH_DIR/original.projection.json
  if ! read_repository_projection "$ORIGINAL_PROJECTION" || ! assert_baseline_projection "$ORIGINAL_PROJECTION"; then
    fail 'baseline changed before mutation'
    return 1
  fi
  cp "$ORIGINAL_PROJECTION" "$baseline_projection"
  EXPECTED_MUTATED_PROJECTION=$SCRATCH_DIR/expected.mutated.projection.json
  jq -cS --arg topic "$MUTATION_TOPIC" '.topics = [$topic]' "$baseline_projection" >"$EXPECTED_MUTATED_PROJECTION"

  ACTION_GUARD_RESULT='FAIL'
  if ! run_octostate "$plan" config plan --config-dir "$mutated_dir"; then
    fail 'mutated config plan failed'
    return 1
  fi
  if ! assert_exact_topic_action "$plan" '.executable_actions' '[]' "[\"$MUTATION_TOPIC\"]" plan; then
    fail 'mutated config plan was not exactly one topic-only repository update'
    return 1
  fi
  ACTION_GUARD_RESULT='FAIL'
  if ! run_octostate "$check" config apply --config-dir "$mutated_dir" --check; then
    fail 'mutated config apply --check failed'
    return 1
  fi
  if ! assert_exact_topic_action "$check" '.data.checked_actions' '[]' "[\"$MUTATION_TOPIC\"]" check; then
    fail 'mutated config apply --check was not exactly one topic-only repository update'
    return 1
  fi
  return 0
}

apply_mutation_once() {
  local mutated_dir=$SCRATCH_DIR/mutated-config
  local apply=$SCRATCH_DIR/mutated.apply.json
  MUTATION_STARTED=1
  PHASE='mutation'
  PHASE_MUTATION='FAIL'
  ACTION_GUARD_RESULT='FAIL'
  if ! run_octostate "$apply" config apply --config-dir "$mutated_dir"; then
    fail 'mutating config apply failed'
    return 1
  fi
  if ! assert_exact_topic_action "$apply" '.data.executed_actions' '[]' "[\"$MUTATION_TOPIC\"]" apply; then
    fail 'mutating config apply executed an unexpected action'
    return 1
  fi
  PHASE_MUTATION='PASS'
  return 0
}

projection_matches() {
  local expected=$1
  local actual=$2
  [ "$(cat "$expected")" = "$(cat "$actual")" ]
}

stable_projection_matches() {
  local expected=$1
  local actual=$2
  jq -ne --slurpfile expected "$expected" --slurpfile actual "$actual" \
    '$expected[0] | del(.topics) == ($actual[0] | del(.topics))'
}

poll_for_topics() {
  local expected_projection=$1
  local desired_config=$2
  local attempt
  PHASE='convergence'
  PHASE_CONVERGENCE='FAIL'
  for ((attempt = 1; attempt <= POLL_ATTEMPTS; attempt++)); do
    local current=$SCRATCH_DIR/poll.repository.$attempt.json
    local plan=$SCRATCH_DIR/poll.plan.$attempt.json
    local read_status=0
    read_repository_projection "$current" || read_status=$?
    if [ "$read_status" -eq 3 ]; then
      fail 'poll repository response was malformed or had a non-transient API failure'
      return 1
    fi
    if [ "$read_status" -eq 0 ]; then
      if ! stable_projection_matches "$expected_projection" "$current"; then
        fail 'poll repository identity or stable baseline fields changed'
        return 1
      fi
    fi
    if [ "$read_status" -eq 0 ] && projection_matches "$expected_projection" "$current"; then
      if ! run_octostate "$plan" config plan --config-dir "$desired_config"; then
        if is_transient_failure_file "$plan.stderr" && [ "$attempt" -lt "$POLL_ATTEMPTS" ]; then
          sleep "$POLL_DELAY_SECONDS"
          continue
        fi
        fail 'poll config plan failed'
        return 1
      fi
      if ! assert_zero_plan "$plan"; then
        if ! is_plan_envelope_valid "$plan"; then
          fail 'poll config plan response was malformed'
          return 1
        fi
        if ! assert_allowed_skipped_actions "$plan" '.skipped_actions'; then
          fail 'poll config plan contained an unexpected skipped action'
          return 1
        fi
      else
        PHASE_CONVERGENCE='PASS'
        return 0
      fi
    fi
    if [ "$attempt" -eq "$POLL_ATTEMPTS" ]; then
      break
    fi
    sleep "$POLL_DELAY_SECONDS"
  done
  return 1
}

restore_baseline() {
  PHASE='restoration'
  PHASE_RESTORATION='FAIL'
  local current=$SCRATCH_DIR/repository.before-restore.json
  local baseline_dir=$SCRATCH_DIR/baseline-config
  local apply=$SCRATCH_DIR/restore.apply.json
  local baseline_projection
  baseline_projection=$(expected_baseline_projection)

  local read_status=0
  read_with_retries read_repository_projection "$current" || read_status=$?
  if [ "$read_status" -ne 0 ]; then
    fail 'restoration identity/state read failed'
    return 1
  fi
  if [ "$(cat "$current")" = "$baseline_projection" ]; then
    RESTORATION_COMPLETED=1
    PHASE_RESTORATION='PASS (already baseline)'
    RESTORATION_CONVERGENCE='PASS (already baseline)'
    return 0
  fi
  if [ "$EXPECTED_MUTATED_PROJECTION" = '' ] || ! projection_matches "$EXPECTED_MUTATED_PROJECTION" "$current"; then
    fail 'refusing restoration because fixture state is ambiguous or unrelated'
    return 1
  fi
  if ! run_octostate "$apply" config apply --config-dir "$baseline_dir"; then
    fail 'baseline restoration apply failed'
    return 1
  fi
  if ! assert_exact_topic_action "$apply" '.data.executed_actions' "[\"$MUTATION_TOPIC\"]" '[]' apply; then
    fail 'baseline restoration executed an unexpected action'
    return 1
  fi
  local mutation_convergence=$PHASE_CONVERGENCE
  if ! poll_for_topics "$ORIGINAL_PROJECTION" "$baseline_dir"; then
    PHASE_CONVERGENCE=$mutation_convergence
    RESTORATION_CONVERGENCE='FAIL'
    fail 'baseline restoration did not converge'
    return 1
  fi
  PHASE_CONVERGENCE=$mutation_convergence
  RESTORATION_CONVERGENCE='PASS'
  RESTORATION_COMPLETED=1
  PHASE_RESTORATION='PASS'
  return 0
}

write_summary() {
  local status=$1
  local summary=${SUMMARY_FILE:-${RUNNER_TEMP:-/tmp}/octostate-live-summary.$$.md}
  mkdir -p "$(dirname "$summary")"
  cat >"$summary" <<EOF
# Trusted Live Integration

- Run ID: ${GITHUB_RUN_ID:-unknown}
- Tested commit SHA: ${GITHUB_SHA:-unknown}
- Target organization: octostate-test (ID 321418529)
- Fixture repository: octostate-test/octostate-fixture-repo (ID 1347356483)
- Phase: $PHASE
- Baseline: $PHASE_BASELINE
- Read-only scenarios: $PHASE_READ_ONLY
- Mutation: $PHASE_MUTATION (expected topic: octostate-live-integration)
- Exact action guard: $ACTION_GUARD_RESULT
- convergence: $PHASE_CONVERGENCE
- Restoration convergence: $RESTORATION_CONVERGENCE
- Restoration: $PHASE_RESTORATION
- Final: $status
- Recovery: If restoration is FAIL or unknown, stop automation and have a maintainer restore and re-verify the canonical fixture baseline before any further run.
EOF
}

terminate_process_tree() {
  local pid=$1
  local signal=$2
  local child

  if [ -r "/proc/$pid/task/$pid/children" ]; then
    while read -r child; do
      [ -n "$child" ] || continue
      terminate_process_tree "$child" "$signal"
    done <"/proc/$pid/task/$pid/children"
  else
    while read -r child; do
      [ -n "$child" ] || continue
      terminate_process_tree "$child" "$signal"
    done < <(pgrep -P "$pid" 2>/dev/null || true)
  fi
  kill "-$signal" "$pid" 2>/dev/null || true
}

restore_with_deadline() {
  local result_file=$SCRATCH_DIR/cleanup.restore.result
  local child
  local deadline
  local grace
  local timed_out=0
  local guard_before=$ACTION_GUARD_RESULT

  RESTORE_ATTEMPTED=1
  : >"$result_file"
  (
    trap - EXIT TERM INT
    if restore_baseline; then
      printf 'PASS\n' >"$result_file"
    else
      printf 'FAIL\n' >"$result_file"
    fi
  ) &
  child=$!
  deadline=$((SECONDS + CLEANUP_DEADLINE_SECONDS))
  while kill -0 "$child" 2>/dev/null; do
    if [ "$SECONDS" -ge "$deadline" ]; then
      timed_out=1
      terminate_process_tree "$child" TERM
      grace=0
      while kill -0 "$child" 2>/dev/null && [ "$grace" -lt 5 ]; do
        /bin/sleep 1
        grace=$((grace + 1))
      done
      terminate_process_tree "$child" KILL
      wait "$child" 2>/dev/null || true
      ACTION_GUARD_RESULT='FAIL'
      PHASE_RESTORATION='FAIL (cleanup deadline)'
      return 1
    fi
    /bin/sleep 1
  done
  wait "$child" 2>/dev/null || true
  if [ "$timed_out" -eq 1 ] || [ "$SECONDS" -ge "$deadline" ]; then
    ACTION_GUARD_RESULT='FAIL'
    PHASE_RESTORATION='FAIL (cleanup deadline)'
    return 1
  fi
  if [ "$(cat "$result_file" 2>/dev/null)" = 'PASS' ]; then
    ACTION_GUARD_RESULT=$guard_before
    RESTORATION_COMPLETED=1
    RESTORATION_CONVERGENCE='PASS'
    PHASE_RESTORATION='PASS'
    return 0
  fi
  ACTION_GUARD_RESULT='FAIL'
  PHASE_RESTORATION='FAIL (dirty sandbox)'
  return 1
}

cleanup_after_failure() {
  if [ "$CLEANUP_RUNNING" -eq 1 ]; then
    return 0
  fi
  CLEANUP_RUNNING=1
  if [ "$MUTATION_STARTED" -eq 1 ] && [ "$RESTORATION_COMPLETED" -eq 0 ] && [ "$RESTORE_ATTEMPTED" -eq 0 ]; then
    restore_with_deadline || true
  fi
  return 0
}

on_signal() {
  PHASE="interrupted ($1)"
  exit 143
}

on_exit() {
  local status=$?
  local exit_status=$status
  trap - EXIT TERM INT
  set +e
  if [ "$status" -ne 0 ]; then
    cleanup_after_failure
  fi
  if [ "$status" -eq 0 ] && { [ "$mode" = '--read-only' ] || [ "$RESTORATION_COMPLETED" -eq 1 ]; }; then
    FINAL_STATUS=0
    write_summary PASS
  else
    FINAL_STATUS=1
    if [ "$RESTORATION_COMPLETED" -eq 0 ] && [ "$MUTATION_STARTED" -eq 1 ] && [ "$PHASE_RESTORATION" != 'FAIL (cleanup deadline)' ]; then
      PHASE_RESTORATION='FAIL (dirty sandbox)'
    fi
    write_summary FAIL
  fi
  if [ -n "$SCRATCH_DIR" ]; then
    rm -rf "$SCRATCH_DIR"
  fi
  if [ "$status" -eq 2 ]; then
    exit_status=2
  elif [ "$status" -lt 128 ]; then
    exit_status=$FINAL_STATUS
  fi
  exit "$exit_status"
}

main() {
  local runner_temp=${RUNNER_TEMP:-/tmp}
  umask 077
  trap 'on_exit' EXIT
  trap 'on_signal TERM' TERM
  trap 'on_signal INT' INT

  if [ "$#" -ne 1 ] || { [ "${1:-}" != '--read-only' ] && [ "${1:-}" != '--mutate' ]; }; then
    echo 'usage: live-integration.sh --read-only|--mutate' >&2
    return 2
  fi
  mode=$1

  if ! mkdir -p "$runner_temp"; then
    fail "could not create runner temp directory: $runner_temp"
    return 1
  fi
  SCRATCH_DIR=$(mktemp -d "$runner_temp/octostate-live.XXXXXX")
  chmod 700 "$SCRATCH_DIR"

  if ! require_commands; then
    return 1
  fi

  cp -R "$BASELINE_CONFIG_DIR" "$SCRATCH_DIR/baseline-config"
  if ! verify_target_and_baseline; then
    return 1
  fi

  if [ "$mode" = '--read-only' ]; then
    if ! run_read_only_scenarios "$SCRATCH_DIR/baseline-config"; then
      return 1
    fi
    return 0
  fi

  if ! prepare_mutation; then
    return 1
  fi
  if ! apply_mutation_once; then
    return 1
  fi
  if ! poll_for_topics "$EXPECTED_MUTATED_PROJECTION" "$SCRATCH_DIR/mutated-config"; then
    fail 'mutation did not converge'
    return 1
  fi
  if ! restore_with_deadline; then
    return 1
  fi
  PHASE='final verification'
  if ! run_read_only_scenarios "$SCRATCH_DIR/baseline-config"; then
    return 1
  fi
  return 0
}

main "$@"
