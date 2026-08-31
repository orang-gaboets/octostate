#!/usr/bin/env bash

# Run GitHub remote queries with the installation token in Git's environment-backed
# configuration rather than in the child process argument list.
git_ls_remote_with_app_token() {
  local token="$1"
  shift

  GIT_CONFIG_COUNT=1 \
    GIT_CONFIG_KEY_0='http.https://github.com/.extraheader' \
    GIT_CONFIG_VALUE_0="AUTHORIZATION: bearer $token" \
    git ls-remote "$@"
}
