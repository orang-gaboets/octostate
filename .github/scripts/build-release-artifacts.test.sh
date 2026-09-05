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

for artifact in "${expected[@]}"; do
  list="$fixture/${artifact}.list"
  if [[ "$artifact" == *.zip ]]; then
    unzip -Z1 "$output/$artifact" | sort > "$list"
    executable=octostate.exe
  else
    tar -tzf "$output/$artifact" | sort > "$list"
    executable=octostate
  fi
  for member in LICENSE README.md CHANGELOG.md "$executable"; do
    grep -qx "$member" "$list"
  done
  ! grep -Eq '(^|/)(\.github|\.agents|cmd|pkg|internal|docs|.*_test\.go|AGENTS\.md|CONTRIBUTING\.md)' "$list"
done

if (cd "$fixture/repo" && bash .github/scripts/build-release-artifacts.sh invalid "$output") 2>/dev/null; then
  echo 'invalid release tag was accepted' >&2
  exit 1
fi

git -C "$fixture/repo" commit --quiet --allow-empty -m mismatch
if (cd "$fixture/repo" && bash .github/scripts/build-release-artifacts.sh v0.0.0 "$output") 2>/dev/null; then
  echo 'tag/HEAD mismatch was accepted' >&2
  exit 1
fi

echo 'release artifact packaging tests passed'
