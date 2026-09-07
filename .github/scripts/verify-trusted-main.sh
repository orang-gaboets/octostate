#!/usr/bin/env bash
set -euo pipefail

verify_trusted_main() {
  if [ "${GITHUB_REF:-}" != "refs/heads/main" ]; then
    echo "trusted-main verification failed: GITHUB_REF must be refs/heads/main" >&2
    return 1
  fi

  local event_sha="${GITHUB_SHA:-}"
  if [ -z "$event_sha" ]; then
    echo "trusted-main verification failed: GITHUB_SHA is required" >&2
    return 1
  fi

  local checked_out_sha
  checked_out_sha="$(git rev-parse HEAD)"
  if [ "$event_sha" != "$checked_out_sha" ]; then
    echo "trusted-main verification failed: GITHUB_SHA does not match the checked-out commit" >&2
    return 1
  fi

  git fetch origin main --quiet

  local origin_main_sha
  origin_main_sha="$(git rev-parse origin/main)"
  if ! git merge-base --is-ancestor "$event_sha" "$origin_main_sha"; then
    echo "trusted-main verification failed: checked-out commit is not an ancestor of origin/main" >&2
    return 1
  fi

  if [ -z "${GITHUB_OUTPUT:-}" ]; then
    echo "trusted-main verification failed: GITHUB_OUTPUT is required" >&2
    return 1
  fi

  printf 'sha=%s\n' "$event_sha" >>"$GITHUB_OUTPUT"
}

verify_trusted_main "$@"
