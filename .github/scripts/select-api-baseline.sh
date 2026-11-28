#!/usr/bin/env bash
# Print the highest stable release tag in one major line (for example v1).
# Reads published, non-draft, non-prerelease release tags on stdin.
set -euo pipefail

major="${1:?usage: select-api-baseline.sh <major, e.g. v1>}"
[[ "$major" =~ ^v(0|[1-9][0-9]*)$ ]] || { echo "invalid major version: $major" >&2; exit 2; }

# awk exits 0 on no match, so pipefail still reports a real sort or tail failure.
baseline=$(awk -v major="$major" '$0 ~ ("^" major "[.](0|[1-9][0-9]*)[.](0|[1-9][0-9]*)$")' | sort -V | tail -n 1)
[[ -n "$baseline" ]] || { echo "no stable ${major} release found to use as API baseline" >&2; exit 1; }
printf '%s\n' "$baseline"
