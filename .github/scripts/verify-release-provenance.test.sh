#!/usr/bin/env bash
set -euo pipefail

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
verify="$script_dir/verify-release-provenance.sh"
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

fail() {
  echo "FAIL: $1" >&2
  exit 1
}

# Stub gh: reports STUB_GH_VERSION, logs each verify call, and fails the
# verification of any asset whose name contains STUB_GH_FAIL_ON.
mkdir -p "$work/bin"
cat >"$work/bin/gh" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == "--version" ]]; then
  echo "gh version ${STUB_GH_VERSION} (2026-01-01)"
  exit 0
fi
printf '%s\n' "$*" >>"$STUB_GH_LOG"
if [[ -n "${STUB_GH_FAIL_ON:-}" && "$3" == *"${STUB_GH_FAIL_ON}"* ]]; then
  exit 1
fi
STUB
chmod +x "$work/bin/gh"

make_dist() {
  local dir="$1" count="$2" i
  rm -rf "$dir"
  mkdir -p "$dir"
  local names=(darwin_amd64.tar.gz darwin_arm64.tar.gz linux_amd64.tar.gz linux_arm64.tar.gz windows_amd64.zip extra_amd64.zip)
  for ((i = 0; i < count; i++)); do
    : >"$dir/octostate_1.2.3_${names[$i]}"
  done
  : >"$dir/checksums.txt"
}

run_verify() {
  : >"$work/log"
  PATH="$work/bin:$PATH" STUB_GH_LOG="$work/log" STUB_GH_VERSION="${version:-2.102.0}" STUB_GH_FAIL_ON="${fail_on:-}" \
    GITHUB_SERVER_URL=https://github.com \
    bash "$verify" "$work/dist" orang-gaboets/octostate 0123456789abcdef0123456789abcdef01234567
}

make_dist "$work/dist" 5
run_verify >/dev/null 2>&1 || fail "five archives with a patched gh should verify"
[[ $(wc -l <"$work/log") -eq 5 ]] || fail "expected five verify calls, got $(wc -l <"$work/log")"
for asset in darwin_amd64.tar.gz darwin_arm64.tar.gz linux_amd64.tar.gz linux_arm64.tar.gz windows_amd64.zip; do
  grep -q "^attestation verify $work/dist/octostate_1.2.3_${asset} " "$work/log" || fail "missing verify call for $asset"
done
while IFS= read -r call; do
  for want in \
    "--repo orang-gaboets/octostate" \
    "--cert-identity https://github.com/orang-gaboets/octostate/.github/workflows/release-please.yml@refs/heads/main" \
    "--source-ref refs/heads/main" \
    "--source-digest 0123456789abcdef0123456789abcdef01234567" \
    "--deny-self-hosted-runners"; do
    [[ "$call" == *"$want"* ]] || fail "verify call missing '$want': $call"
  done
done <"$work/log"

make_dist "$work/dist" 4
run_verify >/dev/null 2>&1 && fail "four archives must fail"
make_dist "$work/dist" 6
run_verify >/dev/null 2>&1 && fail "six archives must fail"

make_dist "$work/dist" 5
fail_on=linux_arm64 run_verify >/dev/null 2>&1 && fail "a failed archive verification must fail"

version=2.101.9 run_verify >/dev/null 2>&1 && fail "gh older than 2.102.0 must fail"
[[ ! -s "$work/log" ]] || fail "no verification should run with an unsupported gh"
version=2.110.0 run_verify >/dev/null 2>&1 || fail "newer gh should be accepted"
version=garbage run_verify >/dev/null 2>&1 && fail "unparseable gh version must fail"

echo "verify-release-provenance tests passed"
