#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
test_root=$(mktemp -d)
trap 'rm -rf -- "$test_root"' EXIT

stub_bin="$test_root/bin"
mkdir -p "$stub_bin"
git_args_log="$test_root/git-args.log"
git_calls_log="$test_root/git-calls.log"
git_header_log="$test_root/git-header.log"
git_environment_log="$test_root/git-environment.log"
xtrace_log="$test_root/xtrace.log"

cat > "$stub_bin/git" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$@" >> "$TEST_GIT_ARGS_LOG"

if [[ ${1:-} == remote && ${2:-} == get-url ]]; then
	printf '%s\n' "$TEST_ORIGIN"
	exit 0
fi

has_ls_remote=false
for argument in "$@"; do
	if [[ $argument == ls-remote ]]; then
		has_ls_remote=true
		break
	fi
done
if [[ $has_ls_remote != true ]]; then
	echo "unexpected Git invocation" >&2
	exit 99
fi

printf 'called\n' >> "$TEST_GIT_CALLS_LOG"
printf '%s\n' "${OCTOSTATE_GIT_AUTH_HEADER:-}" >> "$TEST_GIT_HEADER_LOG"
{
	printf 'AUTH_HEADER=%s\n' "${OCTOSTATE_GIT_AUTH_HEADER:-}"
	printf 'GH_TOKEN_SET=%s\n' "${GH_TOKEN+x}"
	printf 'GIT_TERMINAL_PROMPT=%s\n' "${GIT_TERMINAL_PROMPT:-}"
	printf 'GIT_ASKPASS=%s\n' "${GIT_ASKPASS:-}"
	printf 'SSH_ASKPASS=%s\n' "${SSH_ASKPASS:-}"
	printf 'GCM_INTERACTIVE=%s\n' "${GCM_INTERACTIVE:-}"
	printf 'GIT_TRACE=%s\n' "${GIT_TRACE:-}"
	printf 'GIT_TRACE_CURL=%s\n' "${GIT_TRACE_CURL:-}"
	printf 'GIT_TRACE_PACKET=%s\n' "${GIT_TRACE_PACKET:-}"
	printf 'GIT_TRACE_PACKFILE=%s\n' "${GIT_TRACE_PACKFILE:-}"
	printf 'GIT_TRACE_PERFORMANCE=%s\n' "${GIT_TRACE_PERFORMANCE:-}"
	printf 'GIT_TRACE_SETUP=%s\n' "${GIT_TRACE_SETUP:-}"
	printf 'GIT_TRACE2=%s\n' "${GIT_TRACE2:-}"
	printf 'GIT_TRACE2_EVENT=%s\n' "${GIT_TRACE2_EVENT:-}"
	printf 'GIT_TRACE2_PERF=%s\n' "${GIT_TRACE2_PERF:-}"
	printf 'GIT_CURL_VERBOSE_SET=%s\n' "${GIT_CURL_VERBOSE+x}"
	printf 'GIT_CONFIG_COUNT=%s\n' "${GIT_CONFIG_COUNT:-}"
	printf 'GIT_CONFIG_VALUE_0_SET=%s\n' "${GIT_CONFIG_VALUE_0+x}"
	printf 'GIT_CONFIG_PARAMETERS_SET=%s\n' "${GIT_CONFIG_PARAMETERS+x}"
} >> "$TEST_GIT_ENVIRONMENT_LOG"

printf '%s' "$TEST_GIT_STDOUT"
printf '%s' "$TEST_GIT_STDERR" >&2
exit "$TEST_GIT_STATUS"
STUB
chmod +x "$stub_bin/git"

export PATH="$stub_bin:$PATH"
export GITHUB_SERVER_URL='https://github.com'
export GITHUB_REPOSITORY='orang-gaboets/octostate'
export TEST_ORIGIN='https://github.com/orang-gaboets/octostate'
export TEST_GIT_ARGS_LOG="$git_args_log"
export TEST_GIT_CALLS_LOG="$git_calls_log"
export TEST_GIT_HEADER_LOG="$git_header_log"
export TEST_GIT_ENVIRONMENT_LOG="$git_environment_log"
export TEST_GIT_STATUS=0
export TEST_GIT_STDOUT=$'0123456789abcdef0123456789abcdef01234567\trefs/heads/main\n'
export TEST_GIT_STDERR=''
export GH_TOKEN=$(printf '%0200d' 0)
export GIT_CURL_VERBOSE=1
export GIT_TRACE_PACKET=1
export GIT_TRACE_PACKFILE=1
export GIT_TRACE_PERFORMANCE=1
export GIT_TRACE_SETUP=1
export GIT_CONFIG_COUNT=1
export GIT_CONFIG_KEY_0='http.https://github.com/.extraheader'
export GIT_CONFIG_VALUE_0='AUTHORIZATION: Basic inherited-header-sentinel'
export GIT_CONFIG_PARAMETERS="'http.https://github.com/.extraheader=AUTHORIZATION: Basic inherited-parameter-sentinel'"

fail() {
	echo "FAIL: $1" >&2
	exit 1
}

reset_logs() {
	: > "$git_args_log"
	: > "$git_calls_log"
	: > "$git_header_log"
	: > "$git_environment_log"
}

expect_result() {
	local branch="$1"
	local auth_mode="$2"
	local expected="$3"
	local actual

	actual=$(git_remote_branch_result "$branch" "$auth_mode")
	[[ $actual == "$expected" ]] || fail "expected $auth_mode/$branch to be $expected, got $actual"
}

assert_log_lacks() {
	local value="$1"
	local path="$2"
	if grep -Fq -- "$value" "$path"; then
		fail "unexpected sensitive value found in $(basename "$path")"
	fi
}

assert_log_contains() {
	local value="$1"
	local path="$2"
	grep -Fq -- "$value" "$path" || fail "expected value not found in $(basename "$path")"
}

assert_log_has_line() {
	local value="$1"
	local path="$2"
	grep -Fqx -- "$value" "$path" || fail "expected line not found in $(basename "$path")"
}

# shellcheck source=./git-remote-auth.sh
source "$script_dir/git-remote-auth.sh"

reset_logs
expect_result main basic present:0
expected_basic_header="AUTHORIZATION: Basic $(printf 'x-access-token:%s' "$GH_TOKEN" | base64 | tr -d '\r\n')"
assert_log_contains "$expected_basic_header" "$git_header_log"
assert_log_lacks "$GH_TOKEN" "$git_args_log"
assert_log_lacks "$(printf 'x-access-token:%s' "$GH_TOKEN" | base64 | tr -d '\r\n')" "$git_args_log"
assert_log_contains '--config-env=http.https://github.com/orang-gaboets/octostate.extraheader=OCTOSTATE_GIT_AUTH_HEADER' "$git_args_log"
assert_log_contains 'http.https://github.com/orang-gaboets/octostate.extraheader=' "$git_args_log"
assert_log_contains 'http.https://github.com/orang-gaboets/octostate.followredirects=false' "$git_args_log"
assert_log_contains 'credential.helper=' "$git_args_log"
assert_log_contains 'ls-remote' "$git_args_log"
assert_log_contains 'refs/heads/main' "$git_args_log"
assert_log_has_line 'GH_TOKEN_SET=' "$git_environment_log"
assert_log_contains 'GIT_TERMINAL_PROMPT=0' "$git_environment_log"
assert_log_contains 'GIT_ASKPASS=/bin/false' "$git_environment_log"
assert_log_contains 'SSH_ASKPASS=/bin/false' "$git_environment_log"
assert_log_contains 'GCM_INTERACTIVE=never' "$git_environment_log"
assert_log_contains 'GIT_TRACE=0' "$git_environment_log"
assert_log_contains 'GIT_TRACE_CURL=0' "$git_environment_log"
assert_log_contains 'GIT_TRACE_PACKET=0' "$git_environment_log"
assert_log_contains 'GIT_TRACE_PACKFILE=0' "$git_environment_log"
assert_log_contains 'GIT_TRACE_PERFORMANCE=0' "$git_environment_log"
assert_log_contains 'GIT_TRACE_SETUP=0' "$git_environment_log"
assert_log_contains 'GIT_TRACE2=0' "$git_environment_log"
assert_log_contains 'GIT_TRACE2_EVENT=0' "$git_environment_log"
assert_log_contains 'GIT_TRACE2_PERF=0' "$git_environment_log"
assert_log_contains 'GIT_CURL_VERBOSE_SET=' "$git_environment_log"
assert_log_contains 'GIT_CONFIG_COUNT=0' "$git_environment_log"
assert_log_has_line 'GIT_CONFIG_VALUE_0_SET=' "$git_environment_log"
assert_log_has_line 'GIT_CONFIG_PARAMETERS_SET=' "$git_environment_log"

reset_logs
TEST_ORIGIN='https://github.com/orang-gaboets/octostate.git'
expect_result main basic present:0
assert_log_contains 'AUTHORIZATION: Basic ' "$git_header_log"

reset_logs
TEST_ORIGIN='https://example.invalid/orang-gaboets/octostate.git'
expect_result main basic setup-error
[[ ! -s $git_calls_log ]] || fail 'credentialed Git ran for a mismatched effective origin'

reset_logs
TEST_ORIGIN=$'https://github.com/orang-gaboets/octostate.git\nhttps://example.invalid/other.git'
expect_result main basic setup-error
[[ ! -s $git_calls_log ]] || fail 'credentialed Git ran for a multi-URL origin'
TEST_ORIGIN='https://github.com/orang-gaboets/octostate'

reset_logs
TEST_GIT_STATUS=2
TEST_GIT_STDOUT=''
TEST_GIT_STDERR=''
expect_result ci/go-toolchain-1.26.12 basic absent:2

reset_logs
TEST_GIT_STATUS=0
TEST_GIT_STDOUT=$'0123456789abcdef0123456789abcdef01234567\trefs/heads/archive/refs/heads/ci/go-toolchain-1.26.12\n'
expect_result ci/go-toolchain-1.26.12 basic absent:0

reset_logs
TEST_GIT_STATUS=0
TEST_GIT_STDOUT='not-a-valid-ls-remote-row'
expect_result main basic git-error:0

reset_logs
TEST_GIT_STDOUT=$'0123456789abcdef0123456789abcdef01234567\trefs/heads/unrelated\n'
expect_result main basic git-error:0

reset_logs
TEST_GIT_STATUS=128
TEST_GIT_STDOUT=''
TEST_GIT_STDERR="fatal: could not read Username for 'https://github.com': terminal prompts disabled"
expect_result main invalid-basic auth-failed:128
assert_log_contains "AUTHORIZATION: Basic $(printf '%s' 'x-access-token:invalid-issue346-probe-token' | base64 | tr -d '\r\n')" "$git_header_log"

reset_logs
TEST_GIT_STATUS=128
TEST_GIT_STDERR='fatal: Authentication failed for https://github.com/orang-gaboets/octostate'
expect_result main bearer auth-failed:128
assert_log_contains "AUTHORIZATION: Bearer $GH_TOKEN" "$git_header_log"
assert_log_lacks "$GH_TOKEN" "$git_args_log"

reset_logs
TEST_GIT_STATUS=128
TEST_GIT_STDERR='fatal: unable to access https://github.com: Could not resolve host'
expect_result main basic git-error:128

reset_logs
TEST_GIT_STATUS=128
TEST_GIT_STDERR='fatal: unable to access https://github.com: The requested URL returned error: 403'
expect_result main basic permission-failed:128

reset_logs
TEST_GIT_STATUS=1
TEST_GIT_STDERR='unexpected Git failure'
expect_result main basic git-error:1

reset_logs
TEST_GIT_STATUS=2
TEST_GIT_STDERR='fatal: transport warning'
expect_result main basic git-error:2

reset_logs
TEST_GIT_STATUS=0
TEST_GIT_STDERR=''
TEST_GIT_STDOUT=$'0123456789abcdef0123456789abcdef01234567\trefs/heads/main\n'
exec 9> "$xtrace_log"
BASH_XTRACEFD=9
set -x
trace_result=$(git_remote_branch_result main basic)
set +x
unset BASH_XTRACEFD
exec 9>&-
[[ $trace_result == present:0 ]] || fail 'traced production call did not identify the existing branch'
assert_log_lacks "$GH_TOKEN" "$xtrace_log"
assert_log_lacks "$(printf 'x-access-token:%s' "$GH_TOKEN" | base64 | tr -d '\r\n')" "$xtrace_log"

echo 'git remote auth tests passed'
