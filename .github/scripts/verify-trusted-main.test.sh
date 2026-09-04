#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
test_root=$(mktemp -d)
trap 'rm -rf "$test_root"' EXIT

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

init_repo() {
  local repo_dir="$1"
  mkdir -p "$repo_dir"
  git init -b main "$repo_dir" >/dev/null
  git -C "$repo_dir" config user.name "Octostate Test"
  git -C "$repo_dir" config user.email "test@example.com"
}

commit_file() {
  local repo_dir="$1"
  local file_name="$2"
  local file_content="$3"

  printf '%s\n' "$file_content" > "$repo_dir/$file_name"
  git -C "$repo_dir" add "$file_name"
  git -C "$repo_dir" commit -m "$file_name" >/dev/null
  git -C "$repo_dir" rev-parse HEAD
}

run_case() {
  local case_name="$1"
  local expected_status="$2"
  local repo_dir="$3"
  local output_file="$4"
  local ref_value="$5"
  local sha_value="$6"
  local expected_stdout="${7:-empty}"
  local expected_stderr="${8:-empty}"

  : >"$output_file"
  : >"$test_root/stdout"
  : >"$test_root/stderr"

  set +e
  (
    cd "$repo_dir"
    GITHUB_REF="$ref_value" \
      GITHUB_SHA="$sha_value" \
      GITHUB_OUTPUT="$output_file" \
      bash "$script_dir/verify-trusted-main.sh"
  ) >"$test_root/stdout" 2>"$test_root/stderr"
  actual_status=$?
  set -e

  if [ "$actual_status" -ne "$expected_status" ]; then
    echo "case $case_name: expected status $expected_status, got $actual_status" >&2
    cat "$test_root/stderr" >&2 || true
    return 1
  fi

  case "$expected_stdout" in
    empty)
      if [ -s "$test_root/stdout" ]; then
        echo "case $case_name: expected empty stdout" >&2
        cat "$test_root/stdout" >&2
        return 1
      fi
      ;;
    nonempty)
      if [ ! -s "$test_root/stdout" ]; then
        echo "case $case_name: expected non-empty stdout" >&2
        return 1
      fi
      ;;
    *)
      fail "unsupported stdout expectation: $expected_stdout"
      ;;
  esac

  case "$expected_stderr" in
    empty)
      if [ -s "$test_root/stderr" ]; then
        echo "case $case_name: expected empty stderr" >&2
        cat "$test_root/stderr" >&2
        return 1
      fi
      ;;
    nonempty)
      if [ ! -s "$test_root/stderr" ]; then
        echo "case $case_name: expected non-empty stderr" >&2
        return 1
      fi
      ;;
    *)
      fail "unsupported stderr expectation: $expected_stderr"
      ;;
  esac
}

assert_output() {
  local expected_line="$1"
  local output_file="$2"

  if ! grep -Fx -- "$expected_line" "$output_file" >/dev/null; then
    echo "expected output line: $expected_line" >&2
    cat "$output_file" >&2 || true
    return 1
  fi
}

assert_no_output() {
  local output_file="$1"
  if [ -s "$output_file" ]; then
    echo "expected empty output file" >&2
    cat "$output_file" >&2
    return 1
  fi
}

origin_repo="$test_root/origin.git"
clone_repo="$test_root/clone"
refresh_clone="$test_root/refresh-clone"
alt_repo="$test_root/alt"
mkdir -p "$origin_repo"
git init --bare "$origin_repo" >/dev/null

init_repo "$test_root/work"
base_sha=$(commit_file "$test_root/work" base.txt base)
git -C "$test_root/work" remote add origin "$origin_repo"
git -C "$test_root/work" push -u origin main >/dev/null

git clone "$origin_repo" "$clone_repo" >/dev/null
git -C "$clone_repo" config user.name "Octostate Test"
git -C "$clone_repo" config user.email "test@example.com"
initial_sha=$(git -C "$clone_repo" rev-parse HEAD)

git clone "$origin_repo" "$refresh_clone" >/dev/null
git -C "$refresh_clone" config user.name "Octostate Test"
git -C "$refresh_clone" config user.email "test@example.com"

good_output="$test_root/good.out"
run_case success 0 "$clone_repo" "$good_output" refs/heads/main "$initial_sha"
assert_output "sha=$initial_sha" "$good_output"

printf '%s\n' local-only > "$clone_repo/local.txt"
git -C "$clone_repo" add local.txt
git -C "$clone_repo" commit -m local-only >/dev/null
local_only_sha=$(git -C "$clone_repo" rev-parse HEAD)

run_case non_main_ref 1 "$clone_repo" "$test_root/non-main.out" refs/heads/feature "$initial_sha" empty nonempty
assert_no_output "$test_root/non-main.out"

run_case event_sha_mismatch 1 "$clone_repo" "$test_root/mismatch.out" refs/heads/main "$base_sha" empty nonempty
assert_no_output "$test_root/mismatch.out"

git clone "$origin_repo" "$alt_repo" >/dev/null
git -C "$alt_repo" config user.name "Octostate Test"
git -C "$alt_repo" config user.email "test@example.com"
new_sha=$(commit_file "$alt_repo" advance.txt advance)
git -C "$alt_repo" push origin main >/dev/null

orphan_repo="$test_root/orphan"
init_repo "$orphan_repo"
printf '%s\n' orphan > "$orphan_repo/orphan.txt"
git -C "$orphan_repo" add orphan.txt
git -C "$orphan_repo" commit -m orphan >/dev/null
git -C "$refresh_clone" fetch "$orphan_repo" HEAD:refs/tmp/orphan >/dev/null
bogus_sha=$(git -C "$refresh_clone" rev-parse refs/tmp/orphan)
git -C "$refresh_clone" update-ref refs/remotes/origin/main "$bogus_sha"

run_case fetch_refreshes_origin_main 0 "$refresh_clone" "$test_root/fetch-refresh.out" refs/heads/main "$initial_sha"
assert_output "sha=$initial_sha" "$test_root/fetch-refresh.out"

run_case non_ancestor 1 "$clone_repo" "$test_root/non-ancestor.out" refs/heads/main "$local_only_sha" empty nonempty
assert_no_output "$test_root/non-ancestor.out"

echo "trusted-main verification tests passed"
