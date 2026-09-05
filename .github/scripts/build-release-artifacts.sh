#!/usr/bin/env bash

set -euo pipefail

usage() {
  echo "usage: $0 <release-tag> <output-directory>" >&2
  exit 2
}

tag=${1:-}
output_dir=${2:-}
[[ -n "$tag" && -n "$output_dir" ]] || usage
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
  echo "release tag must match vX.Y.Z: $tag" >&2
  exit 1
}

repo_root=$(git rev-parse --show-toplevel)
mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd)
tag_commit=$(git rev-parse "$tag^{commit}")
head_commit=$(git rev-parse "HEAD^{commit}")
[[ "$tag_commit" == "$head_commit" ]] || {
  echo "HEAD does not match release tag $tag" >&2
  exit 1
}

version=${tag#v}
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT
rm -f "$output_dir"/octostate_*.tar.gz "$output_dir"/octostate_*.zip "$output_dir/checksums.txt"

sha256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{print $1}'
  else
    shasum -a 256 "$1" | awk '{print $1}'
  fi
}

build_archive() {
  local goos=$1
  local goarch=$2
  local extension=$3
  local archive=$4
  local executable=octostate$extension
  local stage="$work_dir/$goos-$goarch"

  mkdir -p "$stage"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    go build -trimpath -ldflags='-s -w' -o "$stage/$executable" ./cmd/octostate
  cp "$repo_root/LICENSE" "$repo_root/README.md" "$repo_root/CHANGELOG.md" "$stage/"
  touch -t 197001010000 "$stage"/*

  if [[ "$extension" == ".exe" ]]; then
    (cd "$stage" && zip -X -q "$output_dir/$archive" "$executable" LICENSE README.md CHANGELOG.md)
  elif tar --help 2>&1 | grep -q -- '--sort'; then
    tar --sort=name --mtime='UTC 1970-01-01' --owner=0 --group=0 --numeric-owner \
      -czf "$output_dir/$archive" -C "$stage" "$executable" LICENSE README.md CHANGELOG.md
  else
    COPYFILE_DISABLE=1 tar -czf "$output_dir/$archive" \
      -C "$stage" "$executable" LICENSE README.md CHANGELOG.md
  fi
}

build_archive darwin amd64 '' "octostate_${version}_darwin_amd64.tar.gz"
build_archive darwin arm64 '' "octostate_${version}_darwin_arm64.tar.gz"
build_archive linux amd64 '' "octostate_${version}_linux_amd64.tar.gz"
build_archive linux arm64 '' "octostate_${version}_linux_arm64.tar.gz"
build_archive windows amd64 '.exe' "octostate_${version}_windows_amd64.zip"

if [[ "$(uname -s)" == "Linux" && "$(uname -m)" == "x86_64" ]]; then
  smoke_dir="$work_dir/smoke"
  mkdir -p "$smoke_dir"
  tar -xzf "$output_dir/octostate_${version}_linux_amd64.tar.gz" -C "$smoke_dir"
  "$smoke_dir/octostate" --help >/dev/null
fi

(
  cd "$output_dir"
  for artifact in octostate_*.tar.gz octostate_*.zip; do
    [[ -f "$artifact" ]] || continue
    printf '%s  %s\n' "$(sha256 "$artifact")" "$artifact"
  done
) > "$output_dir/checksums.txt"

expected=(
  "octostate_${version}_darwin_amd64.tar.gz"
  "octostate_${version}_darwin_arm64.tar.gz"
  "octostate_${version}_linux_amd64.tar.gz"
  "octostate_${version}_linux_arm64.tar.gz"
  "octostate_${version}_windows_amd64.zip"
)
for artifact in "${expected[@]}"; do
  [[ -s "$output_dir/$artifact" ]] || { echo "missing artifact: $artifact" >&2; exit 1; }
  grep -q "  $artifact$" "$output_dir/checksums.txt" || {
    echo "missing checksum: $artifact" >&2
    exit 1
  }
done

echo "Built release artifacts for $tag in $output_dir"
