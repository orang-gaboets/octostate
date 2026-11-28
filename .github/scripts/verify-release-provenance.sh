#!/usr/bin/env bash
# Verify the build provenance attestation of each release archive in a dist
# directory. Fails unless exactly five archives verify against the release
# workflow on refs/heads/main, the given source commit, and a GitHub-hosted
# runner, using a GitHub CLI with patched attestation identity checks.
set -euo pipefail

dist="${1:?usage: verify-release-provenance.sh <dist-dir> <owner/repo> <source-digest>}"
repo="${2:?usage: verify-release-provenance.sh <dist-dir> <owner/repo> <source-digest>}"
source_digest="${3:?usage: verify-release-provenance.sh <dist-dir> <owner/repo> <source-digest>}"
server="${GITHUB_SERVER_URL:-https://github.com}"
min_gh=2.102.0

gh_version=$(gh --version | awk 'NR == 1 { print $3 }')
[[ "$gh_version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || {
  echo "could not determine the GitHub CLI version" >&2
  exit 1
}
[[ "$(printf '%s\n%s\n' "$min_gh" "$gh_version" | sort -V | head -n 1)" == "$min_gh" ]] || {
  echo "GitHub CLI ${gh_version} is older than ${min_gh}, which fixes attestation identity checks" >&2
  exit 1
}

assets=()
while IFS= read -r asset; do
  assets+=("$asset")
done < <(find "$dist" -maxdepth 1 -type f \( -name 'octostate_*.tar.gz' -o -name 'octostate_*.zip' \) -print | sort)
[[ "${#assets[@]}" -eq 5 ]] || {
  echo "expected exactly five release archives, found ${#assets[@]}" >&2
  exit 1
}

for asset in "${assets[@]}"; do
  gh attestation verify "$asset" \
    --repo "$repo" \
    --cert-identity "${server}/${repo}/.github/workflows/release-please.yml@refs/heads/main" \
    --source-ref refs/heads/main \
    --source-digest "$source_digest" \
    --deny-self-hosted-runners
done
