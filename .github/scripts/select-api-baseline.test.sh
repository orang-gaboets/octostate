#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
select_baseline="$script_dir/select-api-baseline.sh"

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

expect_baseline() {
  local name="$1" major="$2" tags="$3" want="$4" got
  got=$(printf '%s' "$tags" | bash "$select_baseline" "$major") || fail "$name: selection failed"
  [[ "$got" == "$want" ]] || fail "$name: got '$got', want '$want'"
}

expect_failure() {
  local name="$1" major="$2" tags="$3" stderr
  if stderr=$(printf '%s' "$tags" | bash "$select_baseline" "$major" 2>&1 >/dev/null); then
    fail "$name: expected failure"
  fi
  [[ "$stderr" == *"no stable ${major} release"* ]] || fail "$name: unexpected diagnostic '$stderr'"
}

expect_baseline "semantic order" v1 $'v1.9.0\nv1.10.0\nv1.2.3\n' v1.10.0
expect_baseline "prereleases excluded" v1 $'v1.3.0\nv1.4.0-rc.1\nv1.4.0-beta\n' v1.3.0
expect_baseline "other majors excluded" v1 $'v1.3.0\nv2.0.0\nv0.9.0\n' v1.3.0
expect_baseline "input order ignored" v1 $'v1.3.0\nv1.1.0\nv1.2.0' v1.3.0
expect_failure "no releases" v1 ''
expect_failure "only prereleases" v1 $'v1.0.0-rc.1\n'
expect_failure "only other major" v1 $'v2.0.0\n'

echo "select-api-baseline tests passed"
