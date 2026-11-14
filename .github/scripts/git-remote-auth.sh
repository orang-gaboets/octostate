#!/usr/bin/env bash

# Query an exact branch on the configured GitHub origin without placing the App
# token or its encoded form in a child process argument list.
git_remote_branch_result() (
  # The function's subshell keeps this security-sensitive option change local.
  # Turn tracing off before reading GH_TOKEN so xtrace cannot print its value.
  set +x
  set -euo pipefail

  local branch="${1:-}"
  local auth_mode="${2:-basic}"
  local app_token="${GH_TOKEN-}"
  local expected_origin origin_url target_ref auth_header encoded_token
  local git_status stderr_text line object_id remote_ref config_variable trace_variable
  local found_target=false malformed_output=false

  unset GH_TOKEN GIT_CURL_VERBOSE
  unset GIT_CONFIG_PARAMETERS
  for config_variable in ${!GIT_CONFIG_KEY_@} ${!GIT_CONFIG_VALUE_@}; do
    unset "$config_variable"
  done
  export GIT_CONFIG_COUNT=0
  export GIT_TERMINAL_PROMPT=0
  export GIT_ASKPASS=/bin/false
  export SSH_ASKPASS=/bin/false
  export GCM_INTERACTIVE=never
  for trace_variable in \
    GIT_TRACE \
    GIT_TRACE_CURL \
    GIT_TRACE_FSMONITOR \
    GIT_TRACE_PACK_ACCESS \
    GIT_TRACE_PACKET \
    GIT_TRACE_PACKFILE \
    GIT_TRACE_PERFORMANCE \
    GIT_TRACE_REFS \
    GIT_TRACE_SETUP \
    GIT_TRACE_SHALLOW \
    GIT_TRACE2 \
    GIT_TRACE2_EVENT \
    GIT_TRACE2_PERF; do
    export "$trace_variable=0"
  done
  unset GIT_TRACE2_CONFIG_PARAMS GIT_TRACE2_ENV_VARS
  export GIT_TRACE_REDACT=1

  if [[ ! "$branch" =~ ^[A-Za-z0-9][A-Za-z0-9._/-]*$ || "$branch" == *..* || "$branch" == */ ]]; then
    printf 'setup-error\n'
    exit 0
  fi
  if [[ "${GITHUB_SERVER_URL:-}" != 'https://github.com' || ! "${GITHUB_REPOSITORY:-}" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]]; then
    printf 'setup-error\n'
    exit 0
  fi
  if [[ "$auth_mode" != basic && "$auth_mode" != bearer && "$auth_mode" != invalid-basic ]]; then
    printf 'setup-error\n'
    exit 0
  fi

  expected_origin="https://github.com/${GITHUB_REPOSITORY}"
  if ! origin_url="$(git remote get-url --all origin 2>/dev/null)"; then
    printf 'setup-error\n'
    exit 0
  fi
  if [[ "$origin_url" != "$expected_origin" && "$origin_url" != "$expected_origin.git" ]]; then
    printf 'setup-error\n'
    exit 0
  fi

  if [[ "$auth_mode" == invalid-basic ]]; then
    encoded_token="$(printf '%s' 'x-access-token:invalid-issue346-probe-token' | base64 | tr -d '\r\n')"
    auth_header="AUTHORIZATION: Basic ${encoded_token}"
  elif [[ -z "$app_token" ]]; then
    printf 'setup-error\n'
    exit 0
  elif [[ "$auth_mode" == bearer ]]; then
    auth_header="AUTHORIZATION: Bearer ${app_token}"
  else
    encoded_token="$(printf 'x-access-token:%s' "$app_token" | base64 | tr -d '\r\n')"
    auth_header="AUTHORIZATION: Basic ${encoded_token}"
  fi
  unset app_token encoded_token

  scratch_dir="$(mktemp -d "${TMPDIR:-/tmp}/octostate-git-auth.XXXXXX")" || {
    printf 'setup-error\n'
    exit 0
  }
  trap 'rm -rf -- "$scratch_dir"' EXIT

  target_ref="refs/heads/${branch}"
  if OCTOSTATE_GIT_AUTH_HEADER="$auth_header" \
    git \
      -c "http.${origin_url}.extraheader=" \
      --config-env="http.${origin_url}.extraheader=OCTOSTATE_GIT_AUTH_HEADER" \
      -c "http.${origin_url}.followredirects=false" \
      -c credential.helper= \
      -c credential.interactive=false \
      ls-remote --exit-code --heads "$origin_url" "$target_ref" \
      >"$scratch_dir/stdout" 2>"$scratch_dir/stderr"; then
    git_status=0
  else
    git_status=$?
  fi

  if [[ "$git_status" -eq 0 ]]; then
    if [[ ! -s "$scratch_dir/stdout" || -s "$scratch_dir/stderr" ]]; then
      printf 'git-error:0\n'
      exit 0
    fi
    while IFS= read -r line || [[ -n "$line" ]]; do
      if [[ "$line" != *$'\t'* || "$line" == *$'\t'*$'\t'* ]]; then
        malformed_output=true
        break
      fi
      object_id="${line%%$'\t'*}"
      remote_ref="${line#*$'\t'}"
      if [[ ! "$object_id" =~ ^([0-9a-f]{40}|[0-9a-f]{64})$ || ! "$remote_ref" =~ ^refs/heads/[^[:space:][:cntrl:]]+$ ]]; then
        malformed_output=true
        break
      fi
      if [[ "$remote_ref" == "$target_ref" ]]; then
        found_target=true
      elif [[ "$remote_ref" != *"$target_ref" ]]; then
        malformed_output=true
        break
      fi
    done < "$scratch_dir/stdout"

    if [[ "$malformed_output" == true ]]; then
      printf 'git-error:0\n'
    elif [[ "$found_target" == true ]]; then
      printf 'present:0\n'
    else
      # Git matches ls-remote patterns as ref-name suffixes. A returned nested
      # ref is a valid response, but it is not the requested exact branch.
      printf 'absent:0\n'
    fi
    exit 0
  fi

  if [[ "$git_status" -eq 2 ]]; then
    if [[ ! -s "$scratch_dir/stdout" && ! -s "$scratch_dir/stderr" ]]; then
      printf 'absent:2\n'
    else
      printf 'git-error:2\n'
    fi
    exit 0
  fi

  if [[ "$git_status" -eq 128 ]]; then
    stderr_text="$(<"$scratch_dir/stderr")"
    shopt -s nocasematch
    case "$stderr_text" in
      *'returned error: 403'*|*'resource not accessible by integration'*)
        printf 'permission-failed:128\n'
        exit 0
        ;;
      *'authentication failed'*|*'could not read username'*|*'http basic: access denied'*|*'invalid username or password'*|*'returned error: 401'*|*'bad credentials'*)
        printf 'auth-failed:128\n'
        exit 0
        ;;
    esac
  fi

  printf 'git-error:%s\n' "$git_status"
)
