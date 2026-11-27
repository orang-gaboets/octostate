#!/usr/bin/env bash
# Print the highest stable release tag in one major line (for example v1).
# Reads published, non-draft, non-prerelease release tags on stdin.
set -euo pipefail

major="${1:?usage: select-api-baseline.sh <major, e.g. v1>}"
[[ "$major" =~ ^v[0-9]+$ ]] || { echo "invalid major version: $major" >&2; exit 2; }

baseline=$(grep -E "^${major}\.[0-9]+\.[0-9]+$" | sort -V | tail -n 1 || true)
[[ -n "$baseline" ]] || { echo "no stable ${major} release found to use as API baseline" >&2; exit 1; }
printf '%s\n' "$baseline"
