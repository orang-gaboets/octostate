#!/usr/bin/env bash
# Filter `gh pr checks --json name,bucket,workflow` output on stdin to the
# checks the release auto-merge gate evaluates. It drops the gate's own
# workflow and only the advisory Go API compatibility job in CI.
set -euo pipefail

workflow="${1:?usage: release-relevant-checks.sh <gate workflow name>}"
jq --arg workflow "$workflow" \
  '[.[] | select(.workflow != $workflow and (.workflow != "CI" or .name != "Go API compatibility (advisory)"))]'
