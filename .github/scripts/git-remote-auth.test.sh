#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
token='test-installation-token'
stub_status=0
stub_calls=0
expected_args=()

fail() {
	echo "FAIL: $1" >&2
	exit 1
}

git() {
	local -a actual_args=( "$@" )
	if (( $# != ${#expected_args[@]} )); then
		fail "expected ${#expected_args[@]} Git arguments, got $#"
	fi
	for index in "${!expected_args[@]}"; do
		[[ ${actual_args[index]} == "${expected_args[index]}" ]] || fail "expected Git argument ${expected_args[index]}, got ${actual_args[index]}"
	done
	((stub_calls += 1))

	[[ ${GIT_CONFIG_COUNT:-} == "1" ]] || fail "GIT_CONFIG_COUNT was not scoped to git"
	[[ ${GIT_CONFIG_KEY_0:-} == "http.https://github.com/.extraheader" ]] || fail "unexpected Git config key"
	[[ ${GIT_CONFIG_VALUE_0:-} == "AUTHORIZATION: bearer $token" ]] || fail "unexpected Git config value"

	for argument in "$@"; do
		[[ $argument != *"$token"* ]] || fail "token appeared in argv"
	done

	return "$stub_status"
}

unset GIT_CONFIG_COUNT GIT_CONFIG_KEY_0 GIT_CONFIG_VALUE_0
# shellcheck source=./git-remote-auth.sh
source "$script_dir/git-remote-auth.sh"

expected_args=(ls-remote --exit-code --heads origin main)
for expected_status in 0 2 7; do
	stub_status=$expected_status
	if git_with_app_token "$token" ls-remote --exit-code --heads origin main; then
		actual_status=0
	else
		actual_status=$?
	fi
	[[ $actual_status == "$expected_status" ]] || fail "expected status $expected_status, got $actual_status"
done

expected_args=(fetch --no-tags origin main)
stub_status=0
git_with_app_token "$token" fetch --no-tags origin main || fail "fetch failed"

[[ $stub_calls == 4 ]] || fail "expected four Git calls, got $stub_calls"
[[ -z ${GIT_CONFIG_COUNT:-} ]] || fail "GIT_CONFIG_COUNT leaked into the parent shell"
[[ -z ${GIT_CONFIG_KEY_0:-} ]] || fail "GIT_CONFIG_KEY_0 leaked into the parent shell"
[[ -z ${GIT_CONFIG_VALUE_0:-} ]] || fail "GIT_CONFIG_VALUE_0 leaked into the parent shell"

echo "git remote auth tests passed"
