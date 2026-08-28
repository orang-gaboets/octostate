#!/usr/bin/env bash

# Shared by the trusted release workflow and its deterministic fixture tests.
# Callers provide the workflow environment and enable `set -euo pipefail`.

RELEASE_LIFECYCLE_LABEL="autorelease: pending"

release_gate_write_output() {
  printf 'should_merge=%s\n' "$1" >> "$GITHUB_OUTPUT"
}

release_gate_has_label() {
  local pr_json="$1"
  local label="$2"

  jq -e --arg label "$label" 'any(.labels[]?; .name == $label)' <<<"$pr_json" >/dev/null
}

release_gate_event_has_label() {
  local labels_json="$1"
  local label="$2"

  jq -e --arg label "$label" 'any(.[]?; .name == $label)' <<<"$labels_json" >/dev/null
}

release_gate_read_pr() {
  local stderr_file
  local read_status=0

  stderr_file="$(mktemp)"
  RELEASE_GATE_PR_JSON="$(gh api "repos/$EXPECTED_REPOSITORY/pulls/$PR_NUMBER" 2>"$stderr_file")" || read_status=$?
  if [ "$read_status" -ne 0 ]; then
    echo "Failed to read the live release PR state; preserving $RELEASE_READY_LABEL and aborting." >&2
    cat "$stderr_file" >&2
    rm -f "$stderr_file"
    return "$read_status"
  fi

  if ! jq -e 'type == "object" and (.labels | type == "array")' <<<"$RELEASE_GATE_PR_JSON" >/dev/null; then
    echo "The live release PR state was not valid JSON; preserving $RELEASE_READY_LABEL and aborting." >&2
    rm -f "$stderr_file"
    return 1
  fi
  rm -f "$stderr_file"
}

release_gate_remove_approval() {
  if gh pr edit "$PR_URL" --remove-label "$RELEASE_READY_LABEL"; then
    return 0
  fi

  echo "Failed to remove $RELEASE_READY_LABEL from PR #$PR_NUMBER; manual remediation is required and merge is aborted." >&2
  return 1
}

release_gate_invalidate_approval() {
  if ! release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_READY_LABEL"; then
    echo "The configured release approval label is absent; merge is aborted." >&2
    return 1
  fi

  if ! release_gate_remove_approval; then
    return 1
  fi
  echo "Fresh publisher approval is required after Release Please lifecycle state is repaired." >&2
  return 0
}

release_gate_invalidate_missing_lifecycle() {
  echo "$RELEASE_LIFECYCLE_LABEL is absent while $RELEASE_READY_LABEL is present; invalidating stale approval." >&2
  release_gate_invalidate_approval
}

release_gate_require_labels() {
  if release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_READY_LABEL" && \
    release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_LIFECYCLE_LABEL"; then
    return 0
  fi

  if release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_READY_LABEL"; then
    if ! release_gate_invalidate_missing_lifecycle; then
      return 1
    fi
    return 1
  fi

  echo "Both $RELEASE_READY_LABEL and $RELEASE_LIFECYCLE_LABEL are required; merge is aborted." >&2
  return 1
}

release_gate_verify_approver() {
  if [ "$EVENT_SENDER" = "$EXPECTED_BOT" ]; then
    return 1
  fi

  local team_stderr
  local team_status=0
  team_stderr="$(mktemp)"
  gh api "/orgs/$RELEASE_APPROVER_ORG/teams/$RELEASE_APPROVER_TEAM" --jq '.slug' > /dev/null 2>"$team_stderr" || team_status=$?
  if [ "$team_status" -ne 0 ]; then
    echo "Failed to verify release approver team @$RELEASE_APPROVER_ORG/$RELEASE_APPROVER_TEAM." >&2
    echo "Confirm the team slug is correct and the release-please app has Members: read." >&2
    cat "$team_stderr" >&2
    rm -f "$team_stderr"
    return 2
  fi
  rm -f "$team_stderr"

  local membership_stderr
  local membership_status=0
  local team_state=""
  membership_stderr="$(mktemp)"
  team_state="$(gh api "/orgs/$RELEASE_APPROVER_ORG/teams/$RELEASE_APPROVER_TEAM/memberships/$EVENT_SENDER" --jq '.state' 2>"$membership_stderr")" || membership_status=$?
  if [ "$membership_status" -eq 0 ] && [ "$team_state" = "active" ]; then
    rm -f "$membership_stderr"
    return 0
  fi
  if [ "$membership_status" -ne 0 ] && ! grep -q "(HTTP 404)" "$membership_stderr"; then
    echo "Failed to verify whether @$EVENT_SENDER is a release approver." >&2
    echo "Leaving $RELEASE_READY_LABEL in place; confirm the app token can read organization members and retry." >&2
    cat "$membership_stderr" >&2
    rm -f "$membership_stderr"
    return 2
  fi
  rm -f "$membership_stderr"
  return 1
}

release_gate_reject_unauthorized() {
  echo "$RELEASE_READY_LABEL was applied by @$EVENT_SENDER, but only active members of @$RELEASE_APPROVER_ORG/$RELEASE_APPROVER_TEAM may approve release auto-merge." >&2
  local label_removed=false
  if release_gate_remove_approval; then
    label_removed=true
  fi

  local comment_status
  if [ "$label_removed" = true ]; then
    comment_status="The label was removed automatically and this PR was not merged."
  else
    comment_status="This PR was not merged. The workflow also failed to remove the label automatically, so a maintainer should remove it manually."
  fi

  local comment_body
  printf -v comment_body '%s\n\n%s\n\n%s\n' \
    "\`$RELEASE_READY_LABEL\` was applied by @$EVENT_SENDER, but only active members of [@$RELEASE_APPROVER_ORG/$RELEASE_APPROVER_TEAM](https://github.com/orgs/$RELEASE_APPROVER_ORG/teams/$RELEASE_APPROVER_TEAM) may approve release auto-merge." \
    "$comment_status" \
    "An authorized Octostate Publisher should review the release PR and re-apply \`$RELEASE_READY_LABEL\` if it is ready to publish."

  if ! gh pr comment "$PR_URL" --body "$comment_body"; then
    echo "Failed to post unauthorized approval comment on PR #$PR_NUMBER." >&2
  fi
  return 1
}

release_approval_gate_initial() {
  local should_merge=false

  case "$EVENT_ACTION" in
    labeled)
      if [ "$EVENT_LABEL" = "$RELEASE_LIFECYCLE_LABEL" ]; then
        if ! release_gate_read_pr; then
          release_gate_write_output "$should_merge"
          return 1
        fi
        if release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_READY_LABEL"; then
          if release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_LIFECYCLE_LABEL"; then
            echo "$RELEASE_LIFECYCLE_LABEL was restored while $RELEASE_READY_LABEL was present; invalidating stale approval." >&2
            if ! release_gate_invalidate_approval; then
              release_gate_write_output "$should_merge"
              return 1
            fi
          elif ! release_gate_invalidate_missing_lifecycle; then
            release_gate_write_output "$should_merge"
            return 1
          fi
        fi
        release_gate_write_output "$should_merge"
        return 0
      fi

      if [ "$EVENT_LABEL" != "$RELEASE_READY_LABEL" ]; then
        echo "Ignoring label event for $EVENT_LABEL; only $RELEASE_READY_LABEL can approve a release PR."
        release_gate_write_output "$should_merge"
        return 0
      fi

      local approver_status=0
      release_gate_verify_approver || approver_status=$?
      if [ "$approver_status" -eq 2 ]; then
        release_gate_write_output "$should_merge"
        return 1
      fi
      if [ "$approver_status" -ne 0 ]; then
        release_gate_reject_unauthorized
        release_gate_write_output "$should_merge"
        return 1
      fi

      if ! release_gate_read_pr; then
        release_gate_write_output "$should_merge"
        return 1
      fi
      if ! release_gate_event_has_label "${EVENT_LABELS_JSON:-[]}" "$RELEASE_LIFECYCLE_LABEL"; then
        echo "The approval was applied while $RELEASE_LIFECYCLE_LABEL was absent; invalidating stale approval." >&2
        if ! release_gate_invalidate_approval; then
          release_gate_write_output "$should_merge"
          return 1
        fi
        release_gate_write_output "$should_merge"
        return 1
      fi
      if ! release_gate_require_labels; then
        release_gate_write_output "$should_merge"
        return 1
      fi
      should_merge=true
      ;;
    synchronize)
      if ! release_gate_read_pr; then
        release_gate_write_output "$should_merge"
        return 1
      fi
      if release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_READY_LABEL"; then
        echo "Removing stale $RELEASE_READY_LABEL approval after PR update."
        if ! release_gate_remove_approval; then
          release_gate_write_output "$should_merge"
          return 1
        fi
      fi
      ;;
    unlabeled)
      if [ "$EVENT_LABEL" = "$RELEASE_LIFECYCLE_LABEL" ]; then
        if ! release_gate_read_pr; then
          release_gate_write_output "$should_merge"
          return 1
        fi
        if release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_READY_LABEL"; then
          echo "$RELEASE_LIFECYCLE_LABEL was removed while $RELEASE_READY_LABEL was present; invalidating stale approval." >&2
          if ! release_gate_remove_approval; then
            release_gate_write_output "$should_merge"
            return 1
          fi
        fi
      fi
      ;;
    *)
      echo "Ignoring unsupported release approval event: $EVENT_ACTION"
      ;;
  esac

  release_gate_write_output "$should_merge"
}

release_gate_validate_live_state() {
  local validation_status=0

  if ! jq -e --arg expected "$EXPECTED_BASE_BRANCH" '.base.ref == $expected' <<<"$RELEASE_GATE_PR_JSON" >/dev/null; then
    echo "Release PR base branch changed; merge is aborted." >&2
    validation_status=1
  fi
  if ! jq -e --arg expected "$EXPECTED_REPOSITORY" '.head.repo.full_name == $expected' <<<"$RELEASE_GATE_PR_JSON" >/dev/null; then
    echo "Release PR head repository changed; merge is aborted." >&2
    validation_status=1
  fi
  if ! jq -e --arg prefix "$EXPECTED_HEAD_BRANCH_PREFIX" '.head.ref | startswith($prefix)' <<<"$RELEASE_GATE_PR_JSON" >/dev/null; then
    echo "Release PR head branch no longer matches the Release Please contract; merge is aborted." >&2
    validation_status=1
  fi
  if ! jq -e '.draft == false' <<<"$RELEASE_GATE_PR_JSON" >/dev/null; then
    echo "Release PR became a draft; merge is aborted." >&2
    validation_status=1
  fi
  if ! jq -e --arg expected "$EXPECTED_BOT" '.user.login == $expected' <<<"$RELEASE_GATE_PR_JSON" >/dev/null; then
    echo "Release PR author changed; merge is aborted." >&2
    validation_status=1
  fi
  if ! jq -e --arg expected "$PR_HEAD_SHA" '.head.sha == $expected' <<<"$RELEASE_GATE_PR_JSON" >/dev/null; then
    echo "Release PR head SHA changed; merge is aborted." >&2
    validation_status=1
  fi
  if ! release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_READY_LABEL"; then
    echo "The configured release approval label is no longer present; merge is aborted." >&2
    validation_status=1
  fi
  if ! release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_LIFECYCLE_LABEL"; then
    echo "$RELEASE_LIFECYCLE_LABEL is no longer present; merge is aborted." >&2
    validation_status=1
  fi

  return "$validation_status"
}

release_approval_gate_final() {
  if ! release_gate_read_pr; then
    return 1
  fi

  if release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_READY_LABEL" && \
    ! release_gate_has_label "$RELEASE_GATE_PR_JSON" "$RELEASE_LIFECYCLE_LABEL"; then
    if ! release_gate_invalidate_missing_lifecycle; then
      return 1
    fi
    return 1
  fi

  release_gate_validate_live_state
}
