#!/usr/bin/env bash
# Writes a krew plugin manifest to stdout for every platform in krew-template.yaml,
# with download URLs for <version> and each archive's sha256 from <dist>/checksums.txt.
#
# Usage: hack/krew-manifest.sh <version> <dist-dir> [plugin-name]
#   hack/krew-manifest.sh v3.0.0 dist > dist/neat.yaml
#
# Needs jq and mikefarah yq v4.
set -euo pipefail

if [[ $# -lt 2 ]]; then
  sed -n '2,8p' "$0" >&2
  exit 2
fi
version="$1"
dir="$2"
plugin="${3:-neat}"
[[ "$version" == v* ]] || version="v$version"
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

sed "s/\${version}/${version}/g" "$root/krew-template.yaml" |
  yq -o json |
  jq --arg plugin "$plugin" --rawfile sums "$dir/checksums.txt" '
    ($sums | split("\n") | map(select(length > 0) | capture("^(?<sha>[0-9a-f]{64})\\s+(?<file>\\S+)$"))
      | map({(.file): .sha}) | add) as $sha
    | .metadata.name = $plugin
    | .spec.platforms |= map(
        (.uri | split("/") | last) as $file
        | if $sha[$file] then .sha256 = $sha[$file]
          else error("no checksum for \($file) in checksums.txt") end)' |
  yq -P
