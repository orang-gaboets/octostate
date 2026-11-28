#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
filter="$script_dir/release-relevant-checks.sh"
gate="Auto-merge Release Please"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

names() {
  bash "$filter" "$gate" <<<"$1" | jq -r '[.[] | "\(.workflow)/\(.name)/\(.bucket)"] | join(",")'
}

expect() {
  local name="$1" checks="$2" want="$3" got
  got=$(names "$checks") || fail "$name: filter failed"
  [[ "$got" == "$want" ]] || fail "$name: got '$got', want '$want'"
}

expect "exact advisory exclusion" \
  '[{"workflow":"CI","name":"Go API compatibility (advisory)","bucket":"fail"},{"workflow":"CI","name":"Test","bucket":"pass"}]' \
  "CI/Test/pass"
expect "same name in another workflow kept" \
  '[{"workflow":"Other","name":"Go API compatibility (advisory)","bucket":"fail"}]' \
  "Other/Go API compatibility (advisory)/fail"
expect "similar CI name kept" \
  '[{"workflow":"CI","name":"Go API compatibility","bucket":"fail"}]' \
  "CI/Go API compatibility/fail"
expect "other failed, cancelled and pending checks kept" \
  '[{"workflow":"CI","name":"Lint","bucket":"fail"},{"workflow":"CI","name":"Race","bucket":"cancel"},{"workflow":"CodeQL Security","name":"Analyze","bucket":"pending"}]' \
  "CI/Lint/fail,CI/Race/cancel,CodeQL Security/Analyze/pending"
expect "gate workflow excluded" \
  "[{\"workflow\":\"$gate\",\"name\":\"merge\",\"bucket\":\"pending\"}]" \
  ""

# Advisory-only CI must not satisfy the gate's required CI presence check.
bash "$filter" "$gate" <<<'[{"workflow":"CI","name":"Go API compatibility (advisory)","bucket":"pass"}]' |
  jq -e 'any(.[]; .workflow == "CI") | not' >/dev/null || fail "advisory-only CI counted as CI presence"

echo "release-relevant-checks tests passed"
