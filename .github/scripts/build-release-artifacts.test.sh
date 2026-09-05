#!/usr/bin/env bash

set -euo pipefail

script_dir=$(cd "$(dirname "$0")" && pwd)
repo_root=$(cd "$script_dir/../.." && pwd)
fixture=$(mktemp -d)
cleanup() { chmod -R u+w "$fixture" 2>/dev/null || true; rm -rf "$fixture"; }
trap cleanup EXIT
export GOPATH="$fixture/gopath"
export GOMODCACHE="$fixture/gopath/pkg/mod"
export GOCACHE="$fixture/gocache"
export GOTOOLCHAIN=local

git clone --quiet "$repo_root" "$fixture/repo"
mkdir -p "$fixture/repo/.github/scripts"
cp "$script_dir/build-release-artifacts.sh" "$fixture/repo/.github/scripts/"
git -C "$fixture/repo" config user.email test@example.invalid
git -C "$fixture/repo" config user.name test
git -C "$fixture/repo" add -A
git -C "$fixture/repo" commit --quiet -m fixture --allow-empty
git -C "$fixture/repo" tag v0.0.0

output="$fixture/output"
(cd "$fixture/repo" && bash .github/scripts/build-release-artifacts.sh v0.0.0 "$output")

expected=(
  octostate_0.0.0_darwin_amd64.tar.gz
  octostate_0.0.0_darwin_arm64.tar.gz
  octostate_0.0.0_linux_amd64.tar.gz
  octostate_0.0.0_linux_arm64.tar.gz
  octostate_0.0.0_windows_amd64.zip
)
[[ $(wc -l < "$output/checksums.txt") -eq 5 ]]
for artifact in "${expected[@]}"; do
  [[ -s "$output/$artifact" ]]
  grep -q "  $artifact$" "$output/checksums.txt"
done

tar -tzf "$output/${expected[0]}" | sort > "$fixture/tar-list"
grep -qx 'LICENSE' "$fixture/tar-list"
grep -qx 'README.md' "$fixture/tar-list"
grep -qx 'CHANGELOG.md' "$fixture/tar-list"
grep -qx 'octostate' "$fixture/tar-list"
! grep -Eq '(^|/)(\.github|\.agents|cmd|pkg|internal|docs|.*_test\.go|AGENTS\.md|CONTRIBUTING\.md)' "$fixture/tar-list"

unzip -Z1 "$output/${expected[4]}" | sort > "$fixture/zip-list"
grep -qx 'LICENSE' "$fixture/zip-list"
grep -qx 'README.md' "$fixture/zip-list"
grep -qx 'CHANGELOG.md' "$fixture/zip-list"
grep -qx 'octostate.exe' "$fixture/zip-list"
! grep -Eq '(^|/)(\.github|\.agents|cmd|pkg|internal|docs|.*_test\.go|AGENTS\.md|CONTRIBUTING\.md)' "$fixture/zip-list"

echo 'release artifact packaging tests passed'
